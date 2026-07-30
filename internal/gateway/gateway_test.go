package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aniusch/projeto-fiapx/internal/auth"
	"github.com/aniusch/projeto-fiapx/internal/domain"
	"github.com/aniusch/projeto-fiapx/internal/messaging"
)

// In-memory fakes for the gateway's dependencies.

type fakeUsers struct{ byEmail map[string]domain.User }

func (f *fakeUsers) Create(_ context.Context, u *domain.User) error {
	key := strings.ToLower(u.Email)
	if _, exists := f.byEmail[key]; exists {
		return domain.ErrDuplicate
	}
	u.ID = uuid.New()
	u.CreatedAt = time.Now()
	f.byEmail[key] = *u
	return nil
}

func (f *fakeUsers) GetByEmail(_ context.Context, email string) (domain.User, error) {
	u, ok := f.byEmail[strings.ToLower(email)]
	if !ok {
		return domain.User{}, domain.ErrNotFound
	}
	return u, nil
}

func (f *fakeUsers) GetByID(_ context.Context, id uuid.UUID) (domain.User, error) {
	for _, u := range f.byEmail {
		if u.ID == id {
			return u, nil
		}
	}
	return domain.User{}, domain.ErrNotFound
}

type fakeVideos struct{ items map[uuid.UUID]domain.Video }

func (f *fakeVideos) Create(_ context.Context, v *domain.Video) error {
	v.ID = uuid.New()
	v.Status = domain.StatusPending
	now := time.Now()
	v.CreatedAt, v.UpdatedAt = now, now
	f.items[v.ID] = *v
	return nil
}

func (f *fakeVideos) GetByID(_ context.Context, id uuid.UUID) (domain.Video, error) {
	v, ok := f.items[id]
	if !ok {
		return domain.Video{}, domain.ErrNotFound
	}
	return v, nil
}

func (f *fakeVideos) ListByUser(_ context.Context, userID uuid.UUID) ([]domain.Video, error) {
	out := make([]domain.Video, 0)
	for _, v := range f.items {
		if v.UserID == userID {
			out = append(out, v)
		}
	}
	return out, nil
}

type fakeObjects struct{ puts map[string][]byte }

func (f *fakeObjects) Put(_ context.Context, key string, r io.Reader, _ int64, _ string) error {
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	f.puts[key] = b
	return nil
}

func (f *fakeObjects) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	return "https://objects.test/" + key, nil
}

type fakePublisher struct{ jobs []messaging.VideoJob }

func (f *fakePublisher) PublishVideoJob(_ context.Context, job messaging.VideoJob) error {
	f.jobs = append(f.jobs, job)
	return nil
}

// --- Test harness ---------------------------------------------------------

type harness struct {
	router *gin.Engine
	tokens *auth.Manager
	users  *fakeUsers
	videos *fakeVideos
	objs   *fakeObjects
	pub    *fakePublisher
}

func newHarness() *harness {
	gin.SetMode(gin.TestMode)
	h := &harness{
		tokens: auth.NewManager("test-secret", time.Hour),
		users:  &fakeUsers{byEmail: map[string]domain.User{}},
		videos: &fakeVideos{items: map[uuid.UUID]domain.Video{}},
		objs:   &fakeObjects{puts: map[string][]byte{}},
		pub:    &fakePublisher{},
	}
	server := NewServer(Deps{
		Users:     h.users,
		Videos:    h.videos,
		Objects:   h.objs,
		Publisher: h.pub,
		Tokens:    h.tokens,
		Limiter:   nil, // no rate limiting in tests
		Config:    Config{MaxUploadBytes: 10 << 20, PresignTTL: time.Minute},
	})
	h.router = gin.New()
	server.RegisterRoutes(h.router)
	return h
}

func (h *harness) do(t *testing.T, req *http.Request) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.router.ServeHTTP(rec, req)
	return rec
}

func jsonReq(method, path, body string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

// --- Tests ----------------------------------------------------------------

func TestRegisterAndLogin(t *testing.T) {
	h := newHarness()

	// Register.
	rec := h.do(t, jsonReq(http.MethodPost, "/auth/register", `{"email":"a@b.com","password":"password123"}`))
	if rec.Code != http.StatusCreated {
		t.Fatalf("register: got %d body=%s", rec.Code, rec.Body.String())
	}
	var reg authResponse
	mustUnmarshal(t, rec.Body.Bytes(), &reg)
	if _, err := h.tokens.Parse(reg.Token); err != nil {
		t.Fatalf("register returned unparseable token: %v", err)
	}

	// Duplicate email.
	rec = h.do(t, jsonReq(http.MethodPost, "/auth/register", `{"email":"a@b.com","password":"password123"}`))
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate register: got %d, want 409", rec.Code)
	}

	// Invalid payload (short password, bad email).
	rec = h.do(t, jsonReq(http.MethodPost, "/auth/register", `{"email":"nope","password":"x"}`))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("invalid register: got %d, want 400", rec.Code)
	}

	// Login success.
	rec = h.do(t, jsonReq(http.MethodPost, "/auth/login", `{"email":"a@b.com","password":"password123"}`))
	if rec.Code != http.StatusOK {
		t.Fatalf("login: got %d body=%s", rec.Code, rec.Body.String())
	}

	// Login wrong password.
	rec = h.do(t, jsonReq(http.MethodPost, "/auth/login", `{"email":"a@b.com","password":"wrong"}`))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad login: got %d, want 401", rec.Code)
	}
}

func TestProtectedRouteRequiresToken(t *testing.T) {
	h := newHarness()
	rec := h.do(t, httptest.NewRequest(http.MethodGet, "/videos", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated list: got %d, want 401", rec.Code)
	}
}

func TestUploadEnqueuesJob(t *testing.T) {
	h := newHarness()
	userID := uuid.New()
	token, _ := h.tokens.Issue(userID)

	// Build a multipart body with a fake video file.
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("video", "clip.mp4")
	fw.Write([]byte("fake-video-bytes"))
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/videos", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)

	rec := h.do(t, req)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("upload: got %d body=%s", rec.Code, rec.Body.String())
	}

	// A source object was stored...
	if len(h.objs.puts) != 1 {
		t.Fatalf("expected 1 stored object, got %d", len(h.objs.puts))
	}
	// ...a PENDING row was created...
	if len(h.videos.items) != 1 {
		t.Fatalf("expected 1 video row, got %d", len(h.videos.items))
	}
	// ...and exactly one job was published, for this user.
	if len(h.pub.jobs) != 1 {
		t.Fatalf("expected 1 published job, got %d", len(h.pub.jobs))
	}
	if h.pub.jobs[0].UserID != userID {
		t.Fatalf("job user mismatch: got %s want %s", h.pub.jobs[0].UserID, userID)
	}

	var resp videoResponse
	mustUnmarshal(t, rec.Body.Bytes(), &resp)
	if resp.Status != string(domain.StatusPending) {
		t.Fatalf("upload status: got %s want PENDING", resp.Status)
	}

	// The new video shows up in the user's listing.
	listReq := httptest.NewRequest(http.MethodGet, "/videos", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listRec := h.do(t, listReq)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list: got %d", listRec.Code)
	}
	var list struct {
		Videos []videoResponse `json:"videos"`
	}
	mustUnmarshal(t, listRec.Body.Bytes(), &list)
	if len(list.Videos) != 1 {
		t.Fatalf("expected 1 video in listing, got %d", len(list.Videos))
	}
}

func TestUploadRejectsBadExtension(t *testing.T) {
	h := newHarness()
	token, _ := h.tokens.Issue(uuid.New())

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	fw, _ := mw.CreateFormFile("video", "notes.txt")
	fw.Write([]byte("not a video"))
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/videos", &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)

	rec := h.do(t, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad extension: got %d, want 400", rec.Code)
	}
	if len(h.pub.jobs) != 0 {
		t.Fatal("no job should be published for a rejected upload")
	}
}

func mustUnmarshal(t *testing.T, data []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("unmarshal: %v (body=%s)", err, string(data))
	}
}
