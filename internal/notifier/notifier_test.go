package notifier

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/aniusch/projeto-fiapx/internal/messaging"
)

type fakeMailer struct {
	sent    []sentMail
	sendErr error
}

type sentMail struct{ to, subject, body string }

func (f *fakeMailer) Send(to, subject, body string) error {
	if f.sendErr != nil {
		return f.sendErr
	}
	f.sent = append(f.sent, sentMail{to, subject, body})
	return nil
}

func TestHandleSendsEmail(t *testing.T) {
	m := &fakeMailer{}
	n := New(m, nil)

	event := messaging.VideoFailedEvent{
		VideoID:      uuid.New(),
		Email:        "owner@example.com",
		OriginalName: "holiday.mp4",
		Reason:       "ffmpeg failed",
	}
	if err := n.Handle(context.Background(), event); err != nil {
		t.Fatalf("Handle: %v", err)
	}

	if len(m.sent) != 1 {
		t.Fatalf("expected 1 email, got %d", len(m.sent))
	}
	got := m.sent[0]
	if got.to != "owner@example.com" {
		t.Errorf("to = %q, want owner@example.com", got.to)
	}
	if !strings.Contains(got.body, "holiday.mp4") {
		t.Errorf("body should mention the original filename; got:\n%s", got.body)
	}
	if !strings.Contains(got.body, "ffmpeg failed") {
		t.Errorf("body should mention the reason; got:\n%s", got.body)
	}
}

func TestHandleSkipsWhenNoEmail(t *testing.T) {
	m := &fakeMailer{}
	n := New(m, nil)

	if err := n.Handle(context.Background(), messaging.VideoFailedEvent{VideoID: uuid.New()}); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(m.sent) != 0 {
		t.Fatal("no email should be sent when the event has no recipient")
	}
}

func TestHandlePropagatesSendError(t *testing.T) {
	m := &fakeMailer{sendErr: errors.New("smtp down")}
	n := New(m, nil)

	event := messaging.VideoFailedEvent{VideoID: uuid.New(), Email: "x@y.com"}
	if err := n.Handle(context.Background(), event); err == nil {
		t.Fatal("expected an error when the mailer fails, so the message is retried")
	}
}
