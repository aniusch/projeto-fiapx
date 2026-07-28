// Package worker consumes video-processing jobs, runs the ffmpeg pipeline, and
// records the outcome. Like the gateway, it depends on interfaces (defined here,
// the consumer) so its core logic can be tested without RabbitMQ, Postgres, S3,
// or ffmpeg.
package worker

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"github.com/aniusch/projeto-fiapx/internal/domain"
	"github.com/aniusch/projeto-fiapx/internal/messaging"
	"github.com/aniusch/projeto-fiapx/internal/observability"
	"github.com/aniusch/projeto-fiapx/internal/processing"
)

// VideoStore reads and updates video job state.
type VideoStore interface {
	GetByID(ctx context.Context, id uuid.UUID) (domain.Video, error)
	MarkProcessing(ctx context.Context, id uuid.UUID) error
	MarkDone(ctx context.Context, id uuid.UUID, zipKey string, frameCount int) error
	MarkFailed(ctx context.Context, id uuid.UUID, reason string) error
}

// UserStore looks up the owner of a video (to address failure notifications).
type UserStore interface {
	GetByID(ctx context.Context, id uuid.UUID) (domain.User, error)
}

// ObjectStore reads source videos and writes result archives.
type ObjectStore interface {
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
}

// EventPublisher emits failure events for the notifier.
type EventPublisher interface {
	PublishVideoFailed(ctx context.Context, event messaging.VideoFailedEvent) error
}

// Config carries the processing tunables.
type Config struct {
	FFmpegPath string
	FPS        int
	WorkDir    string
	JobTimeout time.Duration
}

// Deps groups the worker's collaborators. Metrics may be nil (e.g. in tests).
type Deps struct {
	Videos  VideoStore
	Users   UserStore
	Objects ObjectStore
	Events  EventPublisher
	Metrics *observability.WorkerMetrics
	Config  Config
}

// Worker processes one job at a time via Handle; concurrency is managed by the
// consumer (see consumer.go).
type Worker struct {
	videos  VideoStore
	users   UserStore
	objects ObjectStore
	events  EventPublisher
	metrics *observability.WorkerMetrics
	cfg     Config
}

// New builds a Worker from its dependencies.
func New(d Deps) *Worker {
	return &Worker{
		videos:  d.Videos,
		users:   d.Users,
		objects: d.Objects,
		events:  d.Events,
		metrics: d.Metrics,
		cfg:     d.Config,
	}
}

// Handle processes a single job end-to-end.
//
// The returned error is reserved for *infrastructure* failures (database or
// object-store unavailable) where a retry might succeed — the consumer requeues
// those. A *processing* failure (ffmpeg can't decode the video) is not an error
// from the queue's perspective: it is recorded as FAILED, a notification event is
// published, and Handle returns nil so the message is acknowledged.
//
// Handle is idempotent: redelivery of an already-finished job is a no-op, and the
// result object uses a deterministic key so reprocessing overwrites rather than
// duplicates.
func (w *Worker) Handle(ctx context.Context, job messaging.VideoJob) error {
	w.metrics.JobStarted()
	start := time.Now()
	outcome, err := w.process(ctx, job)
	w.metrics.JobFinished(outcome, time.Since(start).Seconds())
	return err
}

// process does the real work and reports an outcome label alongside the error.
// The outcome feeds the metrics; the error (non-nil only for infra failures)
// drives the consumer's ack/requeue decision.
func (w *Worker) process(ctx context.Context, job messaging.VideoJob) (outcome string, err error) {
	video, err := w.videos.GetByID(ctx, job.VideoID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			// The row is gone (e.g. user deleted). Nothing to do; don't retry.
			slog.Warn("job references unknown video, dropping", "video_id", job.VideoID)
			return "skipped", nil
		}
		return "error", fmt.Errorf("load video: %w", err) // infra → retry
	}

	if video.Status == domain.StatusDone {
		slog.Info("job already done, skipping", "video_id", job.VideoID)
		return "skipped", nil // idempotent skip
	}

	if err := w.videos.MarkProcessing(ctx, job.VideoID); err != nil {
		return "error", fmt.Errorf("mark processing: %w", err) // infra → retry
	}

	// Per-job scratch directory, cleaned up regardless of outcome.
	jobDir, err := os.MkdirTemp(w.cfg.WorkDir, "job-"+job.VideoID.String()+"-")
	if err != nil {
		return "error", fmt.Errorf("create work dir: %w", err)
	}
	defer os.RemoveAll(jobDir)

	srcPath := filepath.Join(jobDir, "source"+filepath.Ext(job.OriginalName))
	if err := w.download(ctx, job.SourceKey, srcPath); err != nil {
		// Treat a missing/unreadable source as a processing failure: retrying
		// will not conjure the bytes back.
		return w.failOutcome(ctx, job,
			"Não foi possível recuperar o vídeo enviado. Por favor, tente enviá-lo novamente.", err)
	}

	result, err := processing.Run(ctx, w.cfg.FFmpegPath, srcPath, jobDir, w.cfg.FPS)
	if err != nil {
		return w.failOutcome(ctx, job,
			"O vídeo não pôde ser processado. Ele pode estar corrompido ou em um formato não suportado.", err)
	}

	zipKey := fmt.Sprintf("results/%s.zip", job.VideoID)
	if err := w.uploadZip(ctx, zipKey, result.ZipPath); err != nil {
		return "error", fmt.Errorf("upload result: %w", err) // infra → retry
	}

	if err := w.videos.MarkDone(ctx, job.VideoID, zipKey, result.FrameCount); err != nil {
		return "error", fmt.Errorf("mark done: %w", err) // infra → retry
	}

	slog.Info("video processed", "video_id", job.VideoID, "frames", result.FrameCount)
	return "done", nil
}

// failOutcome records a processing failure and maps it to a metrics outcome. If
// even recording the failure fails, it surfaces an infra error so the job retries.
func (w *Worker) failOutcome(ctx context.Context, job messaging.VideoJob, userMessage string, cause error) (string, error) {
	if err := w.fail(ctx, job, userMessage, cause); err != nil {
		return "error", err
	}
	return "failed", nil
}

// fail records a processing failure and notifies the user. userMessage is the
// friendly, user-facing text (stored and emailed); cause is the technical detail
// (e.g. raw ffmpeg output) which is only logged, never shown to the user. It
// returns nil so the message is acked — the failure is durably recorded and the
// user informed, so re-running the doomed job would serve no purpose.
func (w *Worker) fail(ctx context.Context, job messaging.VideoJob, userMessage string, cause error) error {
	// Operators get the full technical cause in the logs.
	slog.Warn("video failed", "video_id", job.VideoID, "message", userMessage, "cause", cause)

	if err := w.videos.MarkFailed(ctx, job.VideoID, userMessage); err != nil {
		// If we can't even record the failure, let the job retry.
		return fmt.Errorf("mark failed: %w", err)
	}

	// Look up the owner's email so the notifier doesn't have to. A lookup miss
	// is non-fatal — we still emit the event, just without an address.
	email := ""
	if u, err := w.users.GetByID(ctx, job.UserID); err == nil {
		email = u.Email
	}

	event := messaging.VideoFailedEvent{
		VideoID:      job.VideoID,
		UserID:       job.UserID,
		Email:        email,
		OriginalName: job.OriginalName,
		Reason:       userMessage,
		OccurredAt:   time.Now(),
	}
	if err := w.events.PublishVideoFailed(ctx, event); err != nil {
		// The FAILED status is recorded; a lost notification shouldn't requeue
		// the whole job. Log and move on.
		slog.Error("publish failure event", "error", err, "video_id", job.VideoID)
	}

	return nil
}

// download streams an object from storage to a local file.
func (w *Worker) download(ctx context.Context, key, dstPath string) error {
	rc, err := w.objects.Get(ctx, key)
	if err != nil {
		return err
	}
	defer rc.Close()

	dst, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	if _, err := io.Copy(dst, rc); err != nil {
		return err
	}
	return nil
}

// uploadZip streams a local zip file to object storage.
func (w *Worker) uploadZip(ctx context.Context, key, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return err
	}
	return w.objects.Put(ctx, key, f, info.Size(), "application/zip")
}
