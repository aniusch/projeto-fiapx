// Package messaging defines the RabbitMQ topology and the message contracts
// exchanged between services. Keeping the exchange/queue names and payload
// structs in one shared package means the gateway (publisher) and the worker and
// notifier (consumers) can never disagree on the wire format.
package messaging

import (
	"time"

	"github.com/google/uuid"
)

// Exchange, queue, and routing-key names. AMQP declarations are idempotent, so
// every service can safely (re)declare this topology on startup.
const (
	// Jobs: the gateway publishes one message per uploaded video; workers consume.
	ExchangeJobs  = "videos.jobs"
	QueueJobs     = "videos.jobs"
	RoutingKeyJob = "process"

	// Dead-letter target for jobs that exhaust their retries. Nothing is lost;
	// failed messages land here for inspection/replay.
	ExchangeJobsDLX = "videos.jobs.dlx"
	QueueJobsDLQ    = "videos.jobs.dlq"

	// Events: the worker publishes failures; the notifier consumes them.
	ExchangeEvents     = "videos.events"
	QueueNotifications = "videos.notifications"
	RoutingKeyFailed   = "video.failed"
)

// VideoJob is the message the gateway publishes to request processing of one
// uploaded video.
type VideoJob struct {
	VideoID      uuid.UUID `json:"video_id"`
	UserID       uuid.UUID `json:"user_id"`
	SourceKey    string    `json:"source_key"`
	OriginalName string    `json:"original_name"`
}

// VideoFailedEvent is published by the worker when processing fails, and consumed
// by the notifier to email the user. It carries the email so the notifier need
// not hit the database.
type VideoFailedEvent struct {
	VideoID      uuid.UUID `json:"video_id"`
	UserID       uuid.UUID `json:"user_id"`
	Email        string    `json:"email"`
	OriginalName string    `json:"original_name"`
	Reason       string    `json:"reason"`
	OccurredAt   time.Time `json:"occurred_at"`
}
