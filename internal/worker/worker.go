// Package worker consumes video-processing jobs, runs the ffmpeg pipeline, and
// records the outcome. Like the gateway, it depends on interfaces (defined here,
// the consumer) so its core logic can be tested without RabbitMQ, Postgres, S3,
// or ffmpeg.
package worker

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/aniusch/projeto-fiapx/internal/messaging"
	"github.com/aniusch/projeto-fiapx/internal/observability"
	"github.com/aniusch/projeto-fiapx/internal/processing"
)

// ObjectStore reads source videos and writes result archives.
type ObjectStore interface {
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
}

// EventPublisher emits the video's lifecycle events. The worker owns no database;
// it reports every state transition through these events, and the gateway (the
// sole writer of the videos table) applies them.
type EventPublisher interface {
	PublishVideoProcessing(ctx context.Context, event messaging.VideoProcessingEvent) error
	PublishVideoDone(ctx context.Context, event messaging.VideoDoneEvent) error
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
	Objects ObjectStore
	Events  EventPublisher
	Metrics *observability.WorkerMetrics
	Config  Config
}

// Worker processes one job at a time via Handle; concurrency is managed by the
// consumer (see consumer.go). It is stateless: it holds no database handle and
// communicates results purely through events.
type Worker struct {
	objects ObjectStore
	events  EventPublisher
	metrics *observability.WorkerMetrics
	cfg     Config
}

// New builds a Worker from its dependencies.
func New(d Deps) *Worker {
	return &Worker{
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
// Handle is safe to retry: the result object uses a deterministic key so
// reprocessing overwrites rather than duplicates, and the gateway applies the
// resulting status events idempotently.
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
	// Announce that work has started. This is only a status hint: if it never
	// reaches the gateway the job still runs and the terminal (done/failed) event
	// carries the row to its final state, so a publish failure only warns.
	if err := w.events.PublishVideoProcessing(ctx, messaging.VideoProcessingEvent{VideoID: job.VideoID}); err != nil {
		slog.Warn("publish processing event", "error", err, "video_id", job.VideoID)
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

	// The done event is the only record of success, so a publish failure must
	// requeue the job (the deterministic result key makes a re-run a safe overwrite).
	if err := w.events.PublishVideoDone(ctx, messaging.VideoDoneEvent{
		VideoID:    job.VideoID,
		ZipKey:     zipKey,
		FrameCount: result.FrameCount,
	}); err != nil {
		return "error", fmt.Errorf("publish done event: %w", err) // infra → retry
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

// fail reports a processing failure. userMessage is the friendly, user-facing
// text (stored by the gateway and emailed by the notifier); cause is the
// technical detail (e.g. raw ffmpeg output) which is only logged, never shown to
// the user. The failure event is the sole record of the failure, so if it can't
// be published the error is returned to requeue the job; otherwise it returns
// nil so the message is acked — re-running a doomed job would serve no purpose.
// The owner's email rides along in the job, so no database lookup is needed.
func (w *Worker) fail(ctx context.Context, job messaging.VideoJob, userMessage string, cause error) error {
	// Operators get the full technical cause in the logs.
	slog.Warn("video failed", "video_id", job.VideoID, "message", userMessage, "cause", cause)

	event := messaging.VideoFailedEvent{
		VideoID:      job.VideoID,
		UserID:       job.UserID,
		Email:        job.Email,
		OriginalName: job.OriginalName,
		Reason:       userMessage,
		OccurredAt:   time.Now(),
	}
	if err := w.events.PublishVideoFailed(ctx, event); err != nil {
		return fmt.Errorf("publish failure event: %w", err)
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
