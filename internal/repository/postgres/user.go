// Package postgres contains concrete, Postgres-backed implementations of the
// repositories the services use to persist domain objects.
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gabrielschuina/projeto-fiapx/internal/domain"
)

// ErrNotFound is returned when a lookup matches no row. Callers compare against
// it with errors.Is instead of poking at driver-specific error values.
var ErrNotFound = errors.New("not found")

// UserRepository persists and retrieves users.
type UserRepository struct {
	pool *pgxpool.Pool
}

// NewUserRepository wires a repository to a shared connection pool.
func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

// Create inserts a user. The database generates the id and created_at, which we
// read back via RETURNING so the passed-in struct is fully populated afterwards.
func (r *UserRepository) Create(ctx context.Context, u *domain.User) error {
	const q = `
		INSERT INTO users (email, password_hash)
		VALUES ($1, $2)
		RETURNING id, created_at`

	err := r.pool.QueryRow(ctx, q, u.Email, u.PasswordHash).
		Scan(&u.ID, &u.CreatedAt)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// GetByEmail fetches a user by email (case-insensitive, thanks to the citext
// column). Returns ErrNotFound if no such user exists.
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (domain.User, error) {
	const q = `
		SELECT id, email, password_hash, created_at
		FROM users
		WHERE email = $1`

	return r.scanUser(r.pool.QueryRow(ctx, q, email))
}

// GetByID fetches a user by id. Returns ErrNotFound if absent.
func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.User, error) {
	const q = `
		SELECT id, email, password_hash, created_at
		FROM users
		WHERE id = $1`

	return r.scanUser(r.pool.QueryRow(ctx, q, id))
}

// scanUser reads one row into a domain.User, translating pgx's "no rows"
// sentinel into our own ErrNotFound.
func (r *UserRepository) scanUser(row pgx.Row) (domain.User, error) {
	var u domain.User
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, ErrNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("scan user: %w", err)
	}
	return u, nil
}
