package gateway

import (
	"time"

	"github.com/aniusch/projeto-fiapx/internal/domain"
)

// --- Requests -------------------------------------------------------------
//
// The `binding` struct tags are validated by Gin (via go-playground/validator)
// when we call ShouldBindJSON — declarative validation instead of hand-written
// if-checks. A failing rule produces a 400 with a descriptive message.

type registerRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8,max=72"` // bcrypt caps at 72 bytes
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// --- Responses ------------------------------------------------------------

type authResponse struct {
	Token string `json:"token"`
}

type videoResponse struct {
	ID           string    `json:"id"`
	OriginalName string    `json:"original_name"`
	Status       string    `json:"status"`
	FrameCount   int       `json:"frame_count"`
	ErrorMessage string    `json:"error_message,omitempty"`
	DownloadURL  string    `json:"download_url,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// newVideoResponse maps a domain.Video to its API shape. The download URL is
// only meaningful once processing is DONE, so it is left empty otherwise (and,
// being `omitempty`, disappears from the JSON entirely).
func newVideoResponse(v domain.Video, downloadURL string) videoResponse {
	return videoResponse{
		ID:           v.ID.String(),
		OriginalName: v.OriginalName,
		Status:       string(v.Status),
		FrameCount:   v.FrameCount,
		ErrorMessage: v.ErrorMessage,
		DownloadURL:  downloadURL,
		CreatedAt:    v.CreatedAt,
		UpdatedAt:    v.UpdatedAt,
	}
}

// errorResponse is the uniform error body for the API.
func errorResponse(message string) map[string]string {
	return map[string]string{"error": message}
}
