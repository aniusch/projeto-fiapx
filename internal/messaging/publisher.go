package messaging

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Publisher publishes messages to RabbitMQ over a single channel. A RabbitMQ
// channel is NOT safe for concurrent use, but the gateway publishes from many
// HTTP handlers at once, so we serialize publishes with a mutex. (Publishing is
// fast — a network write — so this is not a meaningful bottleneck here.)
type Publisher struct {
	mu sync.Mutex
	ch *amqp.Channel
}

// NewPublisher wraps an AMQP channel. The caller owns the channel's lifecycle.
func NewPublisher(ch *amqp.Channel) *Publisher {
	return &Publisher{ch: ch}
}

// PublishVideoJob sends a job to the jobs exchange. Messages are marked
// persistent so they survive a broker restart while sitting in the durable queue.
func (p *Publisher) PublishVideoJob(ctx context.Context, job VideoJob) error {
	return p.publish(ctx, ExchangeJobs, RoutingKeyJob, job)
}

// PublishVideoFailed sends a failure event to the events exchange.
func (p *Publisher) PublishVideoFailed(ctx context.Context, event VideoFailedEvent) error {
	return p.publish(ctx, ExchangeEvents, RoutingKeyFailed, event)
}

func (p *Publisher) publish(ctx context.Context, exchange, routingKey string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// PublishWithContext respects ctx cancellation/deadline. mandatory=false:
	// we don't error if no queue is bound (the topology guarantees one is).
	err = p.ch.PublishWithContext(ctx, exchange, routingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         body,
	})
	if err != nil {
		return fmt.Errorf("publish to %s: %w", exchange, err)
	}
	return nil
}
