package gateway

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aniusch/projeto-fiapx/internal/auth"
	"github.com/aniusch/projeto-fiapx/internal/domain"
)

type fakeLimiter struct {
	allow bool
	err   error
}

func (f *fakeLimiter) Allow(context.Context, string) (bool, error) {
	return f.allow, f.err
}

// serverWithLimiter builds a router whose auth routes use the given limiter.
func serverWithLimiter(l Limiter) *gin.Engine {
	gin.SetMode(gin.TestMode)
	server := NewServer(Deps{
		Users:     &fakeUsers{byEmail: map[string]domain.User{}},
		Videos:    &fakeVideos{items: map[uuid.UUID]domain.Video{}},
		Objects:   &fakeObjects{puts: map[string][]byte{}},
		Publisher: &fakePublisher{},
		Tokens:    auth.NewManager("test-secret", time.Hour),
		Limiter:   l,
		Config:    Config{MaxUploadBytes: 1 << 20, PresignTTL: time.Minute},
	})
	r := gin.New()
	server.RegisterRoutes(r)
	return r
}

func TestRateLimitBlocksWhenExceeded(t *testing.T) {
	r := serverWithLimiter(&fakeLimiter{allow: false})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, jsonReq(http.MethodPost, "/auth/register", `{"email":"a@b.com","password":"password123"}`))
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("got %d, want 429", rec.Code)
	}
}

func TestRateLimitFailsOpenOnError(t *testing.T) {
	// If Redis is unreachable, the limiter errors — we must not lock users out.
	r := serverWithLimiter(&fakeLimiter{allow: false, err: context.DeadlineExceeded})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, jsonReq(http.MethodPost, "/auth/register", `{"email":"a@b.com","password":"password123"}`))
	if rec.Code == http.StatusTooManyRequests {
		t.Fatalf("limiter error should fail open, but got 429")
	}
}

func TestAuthRejectsMalformedHeader(t *testing.T) {
	h := newHarness()
	for _, header := range []string{"", "token-without-bearer", "Bearer ", "Bearer not.a.jwt"} {
		req := httptest.NewRequest(http.MethodGet, "/videos", nil)
		if header != "" {
			req.Header.Set("Authorization", header)
		}
		rec := h.do(t, req)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("header %q: got %d, want 401", header, rec.Code)
		}
	}
}

func TestGetVideoOwnershipAndValidation(t *testing.T) {
	h := newHarness()
	owner := uuid.New()
	other := uuid.New()
	ownerToken, _ := h.tokens.Issue(owner)
	otherToken, _ := h.tokens.Issue(other)

	vid := uuid.New()
	h.videos.items[vid] = domain.Video{ID: vid, UserID: owner, Status: domain.StatusPending, OriginalName: "v.mp4"}

	// Owner can read it.
	if rec := h.get(t, "/videos/"+vid.String(), ownerToken); rec.Code != http.StatusOK {
		t.Fatalf("owner get: got %d, want 200", rec.Code)
	}
	// A different user gets 404 (not 403 — we don't reveal existence).
	if rec := h.get(t, "/videos/"+vid.String(), otherToken); rec.Code != http.StatusNotFound {
		t.Fatalf("non-owner get: got %d, want 404", rec.Code)
	}
	// Unknown id -> 404.
	if rec := h.get(t, "/videos/"+uuid.New().String(), ownerToken); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown get: got %d, want 404", rec.Code)
	}
	// Malformed id -> 400.
	if rec := h.get(t, "/videos/not-a-uuid", ownerToken); rec.Code != http.StatusBadRequest {
		t.Fatalf("malformed id: got %d, want 400", rec.Code)
	}
}

func TestDownloadReadyAndNotReady(t *testing.T) {
	h := newHarness()
	owner := uuid.New()
	token, _ := h.tokens.Issue(owner)

	pending := uuid.New()
	h.videos.items[pending] = domain.Video{ID: pending, UserID: owner, Status: domain.StatusProcessing}
	done := uuid.New()
	h.videos.items[done] = domain.Video{ID: done, UserID: owner, Status: domain.StatusDone, ZipKey: "results/x.zip"}

	// Not ready -> 409.
	if rec := h.get(t, "/videos/"+pending.String()+"/download", token); rec.Code != http.StatusConflict {
		t.Fatalf("pending download: got %d, want 409", rec.Code)
	}
	// Ready -> redirect to the presigned URL from the (fake) object store.
	rec := h.get(t, "/videos/"+done.String()+"/download", token)
	if rec.Code != http.StatusFound {
		t.Fatalf("done download: got %d, want 302", rec.Code)
	}
	if loc := rec.Header().Get("Location"); loc != "https://objects.test/results/x.zip" {
		t.Fatalf("unexpected redirect location: %q", loc)
	}
}

// get issues an authenticated GET and returns the recorder.
func (h *harness) get(t *testing.T, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	return h.do(t, req)
}
