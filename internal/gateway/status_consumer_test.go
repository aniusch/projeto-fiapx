package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/aniusch/projeto-fiapx/internal/messaging"
)

type recordingStatusStore struct {
	processing []uuid.UUID
	done       []messaging.VideoDoneEvent
	failed     map[uuid.UUID]string
	err        error
}

func newRecordingStore() *recordingStatusStore {
	return &recordingStatusStore{failed: map[uuid.UUID]string{}}
}

func (s *recordingStatusStore) MarkProcessing(_ context.Context, id uuid.UUID) error {
	s.processing = append(s.processing, id)
	return s.err
}
func (s *recordingStatusStore) MarkDone(_ context.Context, id uuid.UUID, zipKey string, frameCount int) error {
	s.done = append(s.done, messaging.VideoDoneEvent{VideoID: id, ZipKey: zipKey, FrameCount: frameCount})
	return s.err
}
func (s *recordingStatusStore) MarkFailed(_ context.Context, id uuid.UUID, reason string) error {
	s.failed[id] = reason
	return s.err
}

func delivery(t *testing.T, routingKey string, payload any) amqp.Delivery {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return amqp.Delivery{RoutingKey: routingKey, Body: body}
}

func TestStatusConsumerApplyProcessing(t *testing.T) {
	store := newRecordingStore()
	sc := NewStatusConsumer(store)
	id := uuid.New()

	if err := sc.apply(delivery(t, messaging.RoutingKeyProcessing, messaging.VideoProcessingEvent{VideoID: id})); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(store.processing) != 1 || store.processing[0] != id {
		t.Fatalf("expected MarkProcessing(%s), got %v", id, store.processing)
	}
}

func TestStatusConsumerApplyDone(t *testing.T) {
	store := newRecordingStore()
	sc := NewStatusConsumer(store)
	id := uuid.New()

	evt := messaging.VideoDoneEvent{VideoID: id, ZipKey: "results/x.zip", FrameCount: 7}
	if err := sc.apply(delivery(t, messaging.RoutingKeyDone, evt)); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(store.done) != 1 || store.done[0] != evt {
		t.Fatalf("expected MarkDone %+v, got %v", evt, store.done)
	}
}

func TestStatusConsumerApplyFailed(t *testing.T) {
	store := newRecordingStore()
	sc := NewStatusConsumer(store)
	id := uuid.New()

	evt := messaging.VideoFailedEvent{VideoID: id, Reason: "corrupt file"}
	if err := sc.apply(delivery(t, messaging.RoutingKeyFailed, evt)); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if store.failed[id] != "corrupt file" {
		t.Fatalf("expected MarkFailed reason %q, got %q", "corrupt file", store.failed[id])
	}
}

func TestStatusConsumerUnknownKeyIgnored(t *testing.T) {
	store := newRecordingStore()
	sc := NewStatusConsumer(store)

	if err := sc.apply(delivery(t, "video.bogus", struct{}{})); err != nil {
		t.Fatalf("unknown key should be dropped without error, got %v", err)
	}
	if len(store.processing)+len(store.done)+len(store.failed) != 0 {
		t.Fatal("unknown routing key must not touch the store")
	}
}

func TestStatusConsumerUndecodableDropped(t *testing.T) {
	store := newRecordingStore()
	sc := NewStatusConsumer(store)

	// Valid routing key but a body that isn't the expected JSON object.
	bad := amqp.Delivery{RoutingKey: messaging.RoutingKeyDone, Body: []byte("not json")}
	if err := sc.apply(bad); err != nil {
		t.Fatalf("undecodable message should be dropped (nil), got %v", err)
	}
	if len(store.done) != 0 {
		t.Fatal("undecodable message must not reach the store")
	}
}

func TestStatusConsumerStoreErrorPropagates(t *testing.T) {
	// A transient store failure must surface so the delivery is requeued.
	store := newRecordingStore()
	store.err = errors.New("db down")
	sc := NewStatusConsumer(store)

	err := sc.apply(delivery(t, messaging.RoutingKeyProcessing, messaging.VideoProcessingEvent{VideoID: uuid.New()}))
	if err == nil {
		t.Fatal("expected the store error to propagate")
	}
}
