package worker

import (
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/aniusch/projeto-fiapx/internal/domain"
	"github.com/aniusch/projeto-fiapx/internal/messaging"
)

// --- Fakes ----------------------------------------------------------------

type fakeVideos struct {
	video      domain.Video
	getErr     error
	processing bool
	done       bool
	failed     bool
	failReason string
}

func (f *fakeVideos) GetByID(context.Context, uuid.UUID) (domain.Video, error) {
	return f.video, f.getErr
}
func (f *fakeVideos) MarkProcessing(context.Context, uuid.UUID) error {
	f.processing = true
	return nil
}
func (f *fakeVideos) MarkDone(_ context.Context, _ uuid.UUID, _ string, _ int) error {
	f.done = true
	return nil
}
func (f *fakeVideos) MarkFailed(_ context.Context, _ uuid.UUID, reason string) error {
	f.failed = true
	f.failReason = reason
	return nil
}

type fakeUsers struct{ email string }

func (f *fakeUsers) GetByID(context.Context, uuid.UUID) (domain.User, error) {
	return domain.User{Email: f.email}, nil
}

type fakeObjects struct{ putKeys []string }

func (f *fakeObjects) Get(context.Context, string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("not a real video")), nil
}
func (f *fakeObjects) Put(_ context.Context, key string, r io.Reader, _ int64, _ string) error {
	_, _ = io.Copy(io.Discard, r)
	f.putKeys = append(f.putKeys, key)
	return nil
}

type fakeEvents struct{ published []messaging.VideoFailedEvent }

func (f *fakeEvents) PublishVideoFailed(_ context.Context, e messaging.VideoFailedEvent) error {
	f.published = append(f.published, e)
	return nil
}

func newTestWorker(t *testing.T, v *fakeVideos, u *fakeUsers, o *fakeObjects, e *fakeEvents) *Worker {
	t.Helper()
	return New(Deps{
		Videos:  v,
		Users:   u,
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

func TestHandleIdempotentSkip(t *testing.T) {
	v := &fakeVideos{video: domain.Video{ID: uuid.New(), Status: domain.StatusDone}}
	w := newTestWorker(t, v, &fakeUsers{}, &fakeObjects{}, &fakeEvents{})

	if err := w.Handle(context.Background(), messaging.VideoJob{VideoID: v.video.ID}); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if v.processing || v.failed {
		t.Fatal("an already-DONE video must not be reprocessed")
	}
}

func TestHandleUnknownVideoDropped(t *testing.T) {
	v := &fakeVideos{getErr: domain.ErrNotFound}
	w := newTestWorker(t, v, &fakeUsers{}, &fakeObjects{}, &fakeEvents{})

	if err := w.Handle(context.Background(), messaging.VideoJob{VideoID: uuid.New()}); err != nil {
		t.Fatalf("unknown video should be dropped without error, got %v", err)
	}
}

func TestHandleProcessingFailurePublishesEvent(t *testing.T) {
	id := uuid.New()
	userID := uuid.New()
	v := &fakeVideos{video: domain.Video{ID: id, UserID: userID, Status: domain.StatusPending, SourceKey: "sources/x.mp4"}}
	users := &fakeUsers{email: "owner@example.com"}
	events := &fakeEvents{}

	w := newTestWorker(t, v, users, &fakeObjects{}, events)

	job := messaging.VideoJob{VideoID: id, UserID: userID, SourceKey: "sources/x.mp4", OriginalName: "x.mp4"}
	// ffmpeg is bogus, so processing fails -> handled as FAILED, Handle returns nil.
	if err := w.Handle(context.Background(), job); err != nil {
		t.Fatalf("processing failure should be handled (nil error), got %v", err)
	}

	if !v.processing {
		t.Error("expected MarkProcessing to have been called")
	}
	if !v.failed {
		t.Fatal("expected MarkFailed to have been called")
	}
	// The stored/emailed reason must be the friendly message, never raw ffmpeg
	// output or the binary path.
	if !strings.Contains(v.failReason, "não pôde ser processado") {
		t.Errorf("expected a user-friendly reason, got %q", v.failReason)
	}
	for _, leak := range []string{"ffmpeg", "fork/exec", "/nonexistent"} {
		if strings.Contains(v.failReason, leak) {
			t.Errorf("user-facing reason leaks technical detail %q: %s", leak, v.failReason)
		}
	}
	if len(events.published) != 1 {
		t.Fatalf("expected 1 failure event, got %d", len(events.published))
	}
	if events.published[0].Reason != v.failReason {
		t.Errorf("event reason %q != stored reason %q", events.published[0].Reason, v.failReason)
	}
	if events.published[0].Email != "owner@example.com" {
		t.Errorf("event email = %q, want owner@example.com", events.published[0].Email)
	}
}
