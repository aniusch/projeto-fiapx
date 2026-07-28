//go:build integration

// Integration tests that require a running Postgres (from docker-compose).
// They are excluded from the normal `go test ./...` run by the build tag above
// and executed explicitly with:
//
//	go test -tags=integration ./internal/repository/postgres/...
//
// The connection string can be overridden with TEST_POSTGRES_DSN.
package postgres_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aniusch/projeto-fiapx/internal/domain"
	"github.com/aniusch/projeto-fiapx/internal/platform"
	"github.com/aniusch/projeto-fiapx/internal/repository/postgres"
)

func dsn() string {
	if v := os.Getenv("TEST_POSTGRES_DSN"); v != "" {
		return v
	}
	return "postgres://fiapx:fiapx@localhost:5432/fiapx?sslmode=disable"
}

func TestUserAndVideoRoundTrip(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := platform.NewPostgresPool(ctx, dsn())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	users := postgres.NewUserRepository(pool)
	videos := postgres.NewVideoRepository(pool)

	email := fmt.Sprintf("it+%d@fiapx.local", time.Now().UnixNano())
	u := &domain.User{Email: email, PasswordHash: "hash"}
	if err := users.Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	if u.ID.String() == "" || u.CreatedAt.IsZero() {
		t.Fatal("expected generated id and created_at to be populated")
	}

	// citext column: an upper-cased lookup must match the stored email.
	upper := "IT" + email[2:]
	got, err := users.GetByEmail(ctx, upper)
	if err != nil {
		t.Fatalf("case-insensitive lookup: %v", err)
	}
	if got.ID != u.ID {
		t.Fatalf("citext lookup mismatch: got %s want %s", got.ID, u.ID)
	}

	v := &domain.Video{UserID: u.ID, OriginalName: "clip.mp4", SourceKey: "sources/clip.mp4"}
	if err := videos.Create(ctx, v); err != nil {
		t.Fatalf("create video: %v", err)
	}
	if v.Status != domain.StatusPending {
		t.Fatalf("new video status = %s, want PENDING", v.Status)
	}

	if err := videos.MarkProcessing(ctx, v.ID); err != nil {
		t.Fatalf("mark processing: %v", err)
	}
	if err := videos.MarkDone(ctx, v.ID, "results/clip.zip", 42); err != nil {
		t.Fatalf("mark done: %v", err)
	}

	list, err := videos.ListByUser(ctx, u.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 video, got %d", len(list))
	}
	done := list[0]
	if done.Status != domain.StatusDone || done.FrameCount != 42 || done.ZipKey != "results/clip.zip" {
		t.Fatalf("unexpected done state: %+v", done)
	}
	if !done.UpdatedAt.After(done.CreatedAt) {
		t.Fatal("updated_at trigger did not advance updated_at past created_at")
	}
}

func TestGetByEmailNotFound(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := platform.NewPostgresPool(ctx, dsn())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	users := postgres.NewUserRepository(pool)
	if _, err := users.GetByEmail(ctx, "does-not-exist@nowhere.local"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
