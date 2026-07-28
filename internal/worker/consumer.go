package worker

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/aniusch/projeto-fiapx/internal/messaging"
)

// Consume runs the job-consumption loop until shutdownCtx is cancelled or the
// broker closes the delivery channel.
//
// Concurrency is controlled by the prefetch count via QoS: RabbitMQ delivers at
// most `prefetch` unacknowledged messages at a time, so spawning one goroutine
// per delivery naturally caps in-flight jobs at `prefetch` without a separate
// semaphore. Scaling beyond that is done by running more worker replicas.
func (w *Worker) Consume(shutdownCtx context.Context, ch *amqp.Channel, prefetch int) error {
	// global=false: the prefetch limit applies per-consumer.
	if err := ch.Qos(prefetch, 0, false); err != nil {
		return err
	}

	// autoAck=false: we acknowledge manually after a job is fully handled, so a
	// crash mid-processing leaves the message for redelivery.
	deliveries, err := ch.Consume(messaging.QueueJobs, "", false, false, false, false, nil)
	if err != nil {
		return err
	}

	slog.Info("worker consuming", "queue", messaging.QueueJobs, "prefetch", prefetch)

	var wg sync.WaitGroup
	for {
		select {
		case <-shutdownCtx.Done():
			slog.Info("shutdown requested; waiting for in-flight jobs to finish")
			wg.Wait()
			return nil
		case d, ok := <-deliveries:
			if !ok {
				// Broker/connection closed the channel.
				wg.Wait()
				return nil
			}
			wg.Add(1)
			go func(d amqp.Delivery) {
				defer wg.Done()
				w.processDelivery(d)
			}(d)
		}
	}
}

// processDelivery decodes one message, runs it, and acknowledges it according to
// the outcome:
//
//   - unparseable message  -> Nack without requeue: it goes straight to the DLQ
//     (retrying a malformed message would loop forever).
//   - infra error          -> Nack, requeuing once (using the Redelivered flag as
//     a one-shot retry); a second failure dead-letters it.
//   - success / handled     -> Ack.
func (w *Worker) processDelivery(d amqp.Delivery) {
	var job messaging.VideoJob
	if err := json.Unmarshal(d.Body, &job); err != nil {
		slog.Error("undecodable job message; dead-lettering", "error", err)
		_ = d.Nack(false, false)
		return
	}

	// Each job gets its own bounded context, independent of the shutdown signal,
	// so an in-flight job is allowed to finish (up to JobTimeout) during a
	// graceful stop rather than being cut off.
	ctx, cancel := context.WithTimeout(context.Background(), w.cfg.JobTimeout)
	defer cancel()

	if err := w.Handle(ctx, job); err != nil {
		requeue := !d.Redelivered
		slog.Error("job failed with infrastructure error",
			"error", err, "video_id", job.VideoID, "requeue", requeue)
		_ = d.Nack(false, requeue)
		return
	}

	if err := d.Ack(false); err != nil {
		slog.Error("ack failed", "error", err, "video_id", job.VideoID)
	}
}
