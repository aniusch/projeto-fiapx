// Package domain holds the core business types shared across services. These
// structs know nothing about databases, HTTP, or JSON — keeping them pure means
// the storage and transport layers depend on the domain, not the other way around.
package domain

import (
	"github.com/google/uuid"
	"time"
)

// User is an authenticated account that owns videos.
type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string // bcrypt hash — never the plaintext password
	CreatedAt    time.Time
}
