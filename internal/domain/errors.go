package domain

import "errors"

// Repository-level sentinel errors. They live in the domain so any layer can
// react to them (with errors.Is) without importing a specific storage package —
// the gateway checks domain.ErrDuplicate, not postgres.ErrDuplicate.
var (
	// ErrNotFound means a requested entity does not exist.
	ErrNotFound = errors.New("not found")
	// ErrDuplicate means a uniqueness constraint was violated (e.g. email taken).
	ErrDuplicate = errors.New("duplicate")
)
