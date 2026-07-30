package gateway

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/aniusch/projeto-fiapx/internal/messaging"
)

// VideoStatusStore applies worker-reported lifecycle transitions to the videos
// table. The gateway is the single writer of this table; the worker only emits
// events, which this consumer turns into updates.
type VideoStatusStore interface {
	MarkProcessing(ctx context.Context, id uuid.UUID) error
	MarkDone(ctx context.Context, id uuid.UUID, zipKey string, frameCount int) error
	MarkFailed(ctx context.Context, id uuid.UUID, reason string) error
}

// StatusConsumer subscribes to the video lifecycle events published by the
// worker and applies them to the store. It mirrors the worker/notifier consumer
// pattern: prefetch-bounded concurrency, manual acks, and a one-shot requeue on
// transient failures.
type StatusConsumer struct {
	videos VideoStatusStore
}

// NewStatusConsumer builds a StatusConsumer over a store.
func NewStatusConsumer(videos VideoStatusStore) *StatusConsumer {
	return &StatusConsumer{videos: videos}
}

// Consume runs the status-update loop until shutdownCtx is cancelled or the
// broker closes the delivery channel.
func (s *StatusConsumer) Consume(shutdownCtx context.Context, ch *amqp.Channel, prefetch int) error {
	if err := ch.Qos(prefetch, 0, false); err != nil {
		return err
	}

	deliveries, err := ch.Consume(messaging.QueueVideoStatus, "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	slog.Info("gateway consuming status events", "queue", messaging.QueueVideoStatus, "prefetch", prefetch)

	var wg sync.WaitGroup
	for {
		select {
		case <-shutdownCtx.Done():
			slog.Info("shutdown requested; waiting for in-flight status updates")
			wg.Wait()
			return nil
		case d, ok := <-deliveries:
			if !ok {
				wg.Wait()
				return nil
			}
			wg.Add(1)
			go func(d amqp.Delivery) {
				defer wg.Done()
				s.processDelivery(d)
			}(d)
		}
	}
}

// processDelivery decodes one lifecycle event (dispatched by its routing key),
// applies it to the store, and acks accordingly: undecodable or unknown messages
// are dropped, store failures are retried once (via the Redelivered flag) then
// dropped.
func (s *StatusConsumer) processDelivery(d amqp.Delivery) {
	if err := s.apply(d); err != nil {
		requeue := !d.Redelivered
		slog.Error("apply status event failed", "error", err, "routing_key", d.RoutingKey, "requeue", requeue)
		_ = d.Nack(false, requeue)
		return
	}
	if err := d.Ack(false); err != nil {
		slog.Error("ack failed", "error", err, "routing_key", d.RoutingKey)
	}
}

// apply routes a delivery to the matching store update. A nil error means the
// event was handled (or safely ignored) and can be acked; a non-nil error is a
// transient store failure worth one requeue. Decode/unknown-key problems are
// permanent, so they are logged and treated as handled to avoid a poison loop.
func (s *StatusConsumer) apply(d amqp.Delivery) error {
	ctx := context.Background()
	switch d.RoutingKey {
	case messaging.RoutingKeyProcessing:
		var e messaging.VideoProcessingEvent
		if err := json.Unmarshal(d.Body, &e); err != nil {
			slog.Error("undecodable processing event; dropping", "error", err)
			return nil
		}
		return s.videos.MarkProcessing(ctx, e.VideoID)
	case messaging.RoutingKeyDone:
		var e messaging.VideoDoneEvent
		if err := json.Unmarshal(d.Body, &e); err != nil {
			slog.Error("undecodable done event; dropping", "error", err)
			return nil
		}
		return s.videos.MarkDone(ctx, e.VideoID, e.ZipKey, e.FrameCount)
	case messaging.RoutingKeyFailed:
		var e messaging.VideoFailedEvent
		if err := json.Unmarshal(d.Body, &e); err != nil {
			slog.Error("undecodable failed event; dropping", "error", err)
			return nil
		}
		return s.videos.MarkFailed(ctx, e.VideoID, e.Reason)
	default:
		slog.Warn("unknown status routing key; dropping", "routing_key", d.RoutingKey)
		return nil
	}
}
