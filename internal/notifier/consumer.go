package notifier

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/aniusch/projeto-fiapx/internal/messaging"
)

// Consume runs the notification-consumption loop until shutdownCtx is cancelled
// or the broker closes the delivery channel. It follows the same prefetch-bounded
// concurrency and graceful-drain pattern as the worker's consumer.
func (n *Notifier) Consume(shutdownCtx context.Context, ch *amqp.Channel, prefetch int) error {
	if err := ch.Qos(prefetch, 0, false); err != nil {
		return err
	}

	deliveries, err := ch.Consume(messaging.QueueNotifications, "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	slog.Info("notifier consuming", "queue", messaging.QueueNotifications, "prefetch", prefetch)

	var wg sync.WaitGroup
	for {
		select {
		case <-shutdownCtx.Done():
			slog.Info("shutdown requested; waiting for in-flight notifications")
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
				n.processDelivery(d)
			}(d)
		}
	}
}

// processDelivery decodes an event, sends the email, and acks accordingly:
// undecodable messages are dropped, send failures are retried once (via the
// Redelivered flag) then dropped — notifications are best-effort, so we never
// requeue indefinitely.
func (n *Notifier) processDelivery(d amqp.Delivery) {
	var event messaging.VideoFailedEvent
	if err := json.Unmarshal(d.Body, &event); err != nil {
		slog.Error("undecodable event message; dropping", "error", err)
		_ = d.Nack(false, false)
		return
	}

	if err := n.Handle(context.Background(), event); err != nil {
		requeue := !d.Redelivered
		slog.Error("failed to send notification", "error", err, "video_id", event.VideoID, "requeue", requeue)
		_ = d.Nack(false, requeue)
		return
	}

	if err := d.Ack(false); err != nil {
		slog.Error("ack failed", "error", err, "video_id", event.VideoID)
	}
}
