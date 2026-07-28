package domain

import (
	"time"

	"github.com/google/uuid"
)

// VideoStatus is the lifecycle state of a processing job; it mirrors the
// video_status ENUM in Postgres.
type VideoStatus string

const (
	StatusPending    VideoStatus = "PENDING"
	StatusProcessing VideoStatus = "PROCESSING"
	StatusDone       VideoStatus = "DONE"
	StatusFailed     VideoStatus = "FAILED"
)

// Video is a single upload and its processing result.
type Video struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	OriginalName string
	Status       VideoStatus
	SourceKey    string // object-storage key of the uploaded source video
	ZipKey       string // object-storage key of the resulting frames zip
	FrameCount   int
	ErrorMessage string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
