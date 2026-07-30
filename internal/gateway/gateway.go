// Package gateway implements the public HTTP API: authentication, video uploads,
// per-user status listing, and downloads. Its dependencies are declared as
// interfaces so handlers can be tested with in-memory fakes.
package gateway

import (
	"context"
	"io"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aniusch/projeto-fiapx/internal/domain"
	"github.com/aniusch/projeto-fiapx/internal/messaging"
)

// UserStore persists and looks up users.
type UserStore interface {
	Create(ctx context.Context, u *domain.User) error
	GetByEmail(ctx context.Context, email string) (domain.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (domain.User, error)
}

// VideoStore persists and lists videos.
type VideoStore interface {
	Create(ctx context.Context, v *domain.Video) error
	ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.Video, error)
	GetByID(ctx context.Context, id uuid.UUID) (domain.Video, error)
}

// ObjectStore stores uploaded sources and hands out download URLs.
type ObjectStore interface {
	Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error
	PresignGet(ctx context.Context, key string, expiry time.Duration) (string, error)
}

// JobPublisher enqueues processing jobs.
type JobPublisher interface {
	PublishVideoJob(ctx context.Context, job messaging.VideoJob) error
}

// TokenIssuer issues and verifies auth tokens.
type TokenIssuer interface {
	Issue(userID uuid.UUID) (string, error)
	Parse(token string) (uuid.UUID, error)
}

// Limiter decides whether a request identified by key may proceed. A nil Limiter
// disables rate limiting (used in tests).
type Limiter interface {
	Allow(ctx context.Context, key string) (bool, error)
}

// Config holds tunables for the HTTP layer.
type Config struct {
	MaxUploadBytes int64         // reject uploads larger than this
	PresignTTL     time.Duration // lifetime of download URLs
}

// Server bundles the dependencies and configuration behind the HTTP handlers.
type Server struct {
	users     UserStore
	videos    VideoStore
	objects   ObjectStore
	publisher JobPublisher
	tokens    TokenIssuer
	limiter   Limiter // may be nil
	cfg       Config
}

// Deps groups everything a Server needs, so NewServer stays readable as the list
// grows.
type Deps struct {
	Users     UserStore
	Videos    VideoStore
	Objects   ObjectStore
	Publisher JobPublisher
	Tokens    TokenIssuer
	Limiter   Limiter
	Config    Config
}

// NewServer constructs a Server from its dependencies.
func NewServer(d Deps) *Server {
	return &Server{
		users:     d.Users,
		videos:    d.Videos,
		objects:   d.Objects,
		publisher: d.Publisher,
		tokens:    d.Tokens,
		limiter:   d.Limiter,
		cfg:       d.Config,
	}
}

// RegisterRoutes mounts the API onto a Gin engine.
func (s *Server) RegisterRoutes(r *gin.Engine) {
	auth := r.Group("/auth")
	if s.limiter != nil {
		auth.Use(s.RateLimit())
	}
	auth.POST("/register", s.handleRegister)
	auth.POST("/login", s.handleLogin)

	videos := r.Group("/videos")
	videos.Use(s.AuthRequired())
	videos.POST("", s.handleUpload)
	videos.GET("", s.handleList)
	videos.GET("/:id", s.handleGet)
	videos.GET("/:id/download", s.handleDownload)
}
