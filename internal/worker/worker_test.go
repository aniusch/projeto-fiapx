package worker

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/aniusch/projeto-fiapx/internal/messaging"
)

// --- Fakes ----------------------------------------------------------------

type fakeObjects struct{ putKeys []string }

func (f *fakeObjects) Get(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("not a real video")), nil
}
func (f *fakeObjects) Put(_ context.Context, key string, r io.Reader, _ int64, _ string) error {
	_, _ = io.Copy(io.Discard, r)
	f.putKeys = append(f.putKeys, key)
	return nil
}

// fakeEvents records every lifecycle event the worker publishes, standing in for
// the real RabbitMQ publisher.
type fakeEvents struct {
	processing []messaging.VideoProcessingEvent
	done       []messaging.VideoDoneEvent
	failed     []messaging.VideoFailedEvent
}

func (f *fakeEvents) PublishVideoProcessing(_ context.Context, e messaging.VideoProcessingEvent) error {
	f.processing = append(f.processing, e)
	return nil
}
func (f *fakeEvents) PublishVideoDone(_ context.Context, e messaging.VideoDoneEvent) error {
	f.done = append(f.done, e)
	return nil
}
func (f *fakeEvents) PublishVideoFailed(_ context.Context, e messaging.VideoFailedEvent) error {
	f.failed = append(f.failed, e)
	return nil
}

func newTestWorker(t *testing.T, o *fakeObjects, e *fakeEvents) *Worker {
	t.Helper()
	return New(Deps{
		Objects: o,
		Events:  e,
		Config: Config{
			FFmpegPath: "/nonexistent/ffmpeg", // guarantees the ffmpeg step fails
			FPS:        1,
			WorkDir:    t.TempDir(),
			JobTimeout: 30 * time.Second,
		},
	})
}

// --- Tests ----------------------------------------------------------------

func TestHandleProcessingFailurePublishesEvent(t *testing.T) {
	id := uuid.New()
	userID := uuid.New()
	events := &fakeEvents{}

	w := newTestWorker(t, &fakeObjects{}, events)

	job := messaging.VideoJob{
		VideoID:      id,
		UserID:       userID,
		Email:        "owner@example.com",
		SourceKey:    "sources/x.mp4",
		OriginalName: "x.mp4",
	}
	// ffmpeg is bogus, so processing fails -> handled as FAILED, Handle returns nil.
	if err := w.Handle(context.Background(), job); err != nil {
		t.Fatalf("processing failure should be handled (nil error), got %v", err)
	}

	// The worker always announces that it started before doing any work.
	if len(events.processing) != 1 || events.processing[0].VideoID != id {
		t.Errorf("expected a single processing event for %s, got %+v", id, events.processing)
	}
	if len(events.done) != 0 {
		t.Errorf("a failed job must not emit a done event, got %+v", events.done)
	}
	if len(events.failed) != 1 {
		t.Fatalf("expected 1 failure event, got %d", len(events.failed))
	}

	got := events.failed[0]
	// The emailed reason must be the friendly message, never raw ffmpeg output.
	if !strings.Contains(got.Reason, "não pôde ser processado") {
		t.Errorf("expected a user-friendly reason, got %q", got.Reason)
	}
	for _, leak := range []string{"ffmpeg", "fork/exec", "/nonexistent"} {
		if strings.Contains(got.Reason, leak) {
			t.Errorf("user-facing reason leaks technical detail %q: %s", leak, got.Reason)
		}
	}
	// The owner's address rides along in the job (the worker has no database).
	if got.Email != "owner@example.com" {
		t.Errorf("event email = %q, want owner@example.com", got.Email)
	}
	if got.VideoID != id {
		t.Errorf("event video id = %s, want %s", got.VideoID, id)
	}
}

func TestHandleMissingSourcePublishesFailure(t *testing.T) {
	// A source that can't be downloaded is a processing failure, not an infra
	// error: it is reported as failed and the message is acked (nil error).
	events := &fakeEvents{}
	w := New(Deps{
		Objects: &failingGetObjects{},
		Events:  events,
		Config:  Config{FFmpegPath: "/bin/true", FPS: 1, WorkDir: t.TempDir(), JobTimeout: 30 * time.Second},
	})

	job := messaging.VideoJob{VideoID: uuid.New(), Email: "a@b.com", SourceKey: "missing", OriginalName: "x.mp4"}
	if err := w.Handle(context.Background(), job); err != nil {
		t.Fatalf("a missing source should be handled (nil error), got %v", err)
	}
	if len(events.failed) != 1 {
		t.Fatalf("expected 1 failure event, got %d", len(events.failed))
	}
	if !strings.Contains(events.failed[0].Reason, "recuperar o vídeo") {
		t.Errorf("expected the download-failure message, got %q", events.failed[0].Reason)
	}
}

type failingGetObjects struct{ fakeObjects }

func (f *failingGetObjects) Get(context.Context, string) (io.ReadCloser, error) {
	return nil, io.ErrUnexpectedEOF
}
