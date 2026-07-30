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

	// Events: the worker is stateless and never touches the database. Instead it
	// publishes lifecycle events to this topic exchange. The gateway — the sole
	// owner of the videos table — consumes them to advance each row's status
	// (single-writer + event-carried state transfer), and the notifier consumes
	// the failure events to email the user. Both consumer queues bind to the same
	// exchange, so a failure fans out to both.
	ExchangeEvents       = "videos.events"
	QueueNotifications   = "videos.notifications"
	QueueVideoStatus     = "videos.status"
	RoutingKeyProcessing = "video.processing"
	RoutingKeyDone       = "video.done"
	RoutingKeyFailed     = "video.failed"
)

// VideoJob is the message the gateway publishes to request processing of one
// uploaded video. It carries everything the (stateless) worker needs — including
// the owner's Email — so the worker never has to read the database.
type VideoJob struct {
	VideoID      uuid.UUID `json:"video_id"`
	UserID       uuid.UUID `json:"user_id"`
	Email        string    `json:"email"`
	SourceKey    string    `json:"source_key"`
	OriginalName string    `json:"original_name"`
}

// VideoProcessingEvent is published by the worker when it starts a job, so the
// gateway can advance the row from PENDING to PROCESSING.
type VideoProcessingEvent struct {
	VideoID uuid.UUID `json:"video_id"`
}

// VideoDoneEvent is published by the worker on success; the gateway records the
// result archive's key and frame count and marks the row DONE.
type VideoDoneEvent struct {
	VideoID    uuid.UUID `json:"video_id"`
	ZipKey     string    `json:"zip_key"`
	FrameCount int       `json:"frame_count"`
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
