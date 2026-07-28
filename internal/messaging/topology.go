package messaging

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

// DeclareTopology sets up every exchange and queue the system uses. It is safe to
// call from any service on startup — AMQP declarations are idempotent, so the
// first service to start creates the topology and the rest confirm it matches.
//
// The jobs queue is wired to a dead-letter exchange: when a message is rejected
// without requeue (a job that failed too many times), RabbitMQ routes it to the
// DLQ instead of discarding it.
func DeclareTopology(ch *amqp.Channel) error {
	// --- Jobs exchange + queue, with dead-lettering -----------------------
	if err := ch.ExchangeDeclare(ExchangeJobs, amqp.ExchangeDirect, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare jobs exchange: %w", err)
	}
	if err := ch.ExchangeDeclare(ExchangeJobsDLX, amqp.ExchangeFanout, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare jobs DLX: %w", err)
	}

	if _, err := ch.QueueDeclare(QueueJobs, true, false, false, false, amqp.Table{
		"x-dead-letter-exchange": ExchangeJobsDLX,
	}); err != nil {
		return fmt.Errorf("declare jobs queue: %w", err)
	}
	if err := ch.QueueBind(QueueJobs, RoutingKeyJob, ExchangeJobs, false, nil); err != nil {
		return fmt.Errorf("bind jobs queue: %w", err)
	}

	if _, err := ch.QueueDeclare(QueueJobsDLQ, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare jobs DLQ: %w", err)
	}
	if err := ch.QueueBind(QueueJobsDLQ, "", ExchangeJobsDLX, false, nil); err != nil {
		return fmt.Errorf("bind jobs DLQ: %w", err)
	}

	// --- Events exchange + notifications queue ----------------------------
	if err := ch.ExchangeDeclare(ExchangeEvents, amqp.ExchangeTopic, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare events exchange: %w", err)
	}
	if _, err := ch.QueueDeclare(QueueNotifications, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare notifications queue: %w", err)
	}
	if err := ch.QueueBind(QueueNotifications, RoutingKeyFailed, ExchangeEvents, false, nil); err != nil {
		return fmt.Errorf("bind notifications queue: %w", err)
	}

	return nil
}
