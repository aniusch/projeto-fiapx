// Package domain holds the core business types shared across services,
// independent of any database, transport, or serialization concern.
package domain

import (
	"time"

	"github.com/google/uuid"
)

// User is an authenticated account that owns videos.
type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string // bcrypt hash — never the plaintext password
	CreatedAt    time.Time
}
