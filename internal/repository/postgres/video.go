package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/aniusch/projeto-fiapx/internal/domain"
)

// VideoRepository persists and retrieves video jobs.
type VideoRepository struct {
	pool *pgxpool.Pool
}

// NewVideoRepository wires a repository to a shared connection pool.
func NewVideoRepository(pool *pgxpool.Pool) *VideoRepository {
	return &VideoRepository{pool: pool}
}

// Create inserts a new video row (status defaults to PENDING in the schema) and
// reads back the generated id and timestamps.
func (r *VideoRepository) Create(ctx context.Context, v *domain.Video) error {
	const q = `
		INSERT INTO videos (user_id, original_name, source_key)
		VALUES ($1, $2, $3)
		RETURNING id, status, frame_count, created_at, updated_at`

	err := r.pool.QueryRow(ctx, q, v.UserID, v.OriginalName, v.SourceKey).
		Scan(&v.ID, &v.Status, &v.FrameCount, &v.CreatedAt, &v.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create video: %w", err)
	}
	return nil
}

// GetByID returns a single video. Returns ErrNotFound if absent.
func (r *VideoRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.Video, error) {
	const q = `
		SELECT id, user_id, original_name, status, source_key, zip_key,
		       frame_count, error_message, created_at, updated_at
		FROM videos
		WHERE id = $1`

	row := r.pool.QueryRow(ctx, q, id)
	v, err := scanVideo(row.Scan)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Video{}, domain.ErrNotFound
	}
	return v, err
}

// ListByUser returns a user's videos, newest first — the per-user status listing.
func (r *VideoRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.Video, error) {
	const q = `
		SELECT id, user_id, original_name, status, source_key, zip_key,
		       frame_count, error_message, created_at, updated_at
		FROM videos
		WHERE user_id = $1
		ORDER BY created_at DESC`

	rows, err := r.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("list videos: %w", err)
	}
	defer rows.Close()

	// Start with a non-nil empty slice so the caller (and JSON encoder) sees
	// [] rather than null when a user has no videos.
	videos := make([]domain.Video, 0)
	for rows.Next() {
		v, err := scanVideo(rows.Scan)
		if err != nil {
			return nil, err
		}
		videos = append(videos, v)
	}
	// rows.Err reports errors that happened mid-iteration (e.g. connection loss).
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate videos: %w", err)
	}
	return videos, nil
}

// MarkProcessing transitions a job to PROCESSING (a worker picking up the job).
// It only advances a row that is still PENDING, so a redelivered "processing"
// event cannot regress an already-terminal (DONE/FAILED) row back to PROCESSING.
func (r *VideoRepository) MarkProcessing(ctx context.Context, id uuid.UUID) error {
	const q = `UPDATE videos SET status = $2 WHERE id = $1 AND status = $3`
	return r.exec(ctx, q, id, domain.StatusProcessing, domain.StatusPending)
}

// MarkDone records a successful result: the zip's storage key and frame count.
func (r *VideoRepository) MarkDone(ctx context.Context, id uuid.UUID, zipKey string, frameCount int) error {
	const q = `
		UPDATE videos
		SET status = $2, zip_key = $3, frame_count = $4, error_message = ''
		WHERE id = $1`
	if _, err := r.pool.Exec(ctx, q, id, domain.StatusDone, zipKey, frameCount); err != nil {
		return fmt.Errorf("mark video done: %w", err)
	}
	return nil
}

// MarkFailed records a failure and the reason to show/notify the user.
func (r *VideoRepository) MarkFailed(ctx context.Context, id uuid.UUID, reason string) error {
	const q = `UPDATE videos SET status = $2, error_message = $3 WHERE id = $1`
	if _, err := r.pool.Exec(ctx, q, id, domain.StatusFailed, reason); err != nil {
		return fmt.Errorf("mark video failed: %w", err)
	}
	return nil
}

func (r *VideoRepository) exec(ctx context.Context, q string, args ...any) error {
	if _, err := r.pool.Exec(ctx, q, args...); err != nil {
		return fmt.Errorf("exec: %w", err)
	}
	return nil
}

// scanVideo reads one row into a domain.Video. It takes the row's Scan method as
// a function value so the same logic works for both a single-row QueryRow and
// each row of a multi-row Query.
func scanVideo(scan func(dest ...any) error) (domain.Video, error) {
	var v domain.Video
	err := scan(
		&v.ID, &v.UserID, &v.OriginalName, &v.Status, &v.SourceKey, &v.ZipKey,
		&v.FrameCount, &v.ErrorMessage, &v.CreatedAt, &v.UpdatedAt,
	)
	if err != nil {
		return domain.Video{}, err
	}
	return v, nil
}
