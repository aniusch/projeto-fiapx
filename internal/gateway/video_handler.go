package gateway

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/aniusch/projeto-fiapx/internal/domain"
	"github.com/aniusch/projeto-fiapx/internal/messaging"
)

// allowedVideoExts is the set of accepted upload extensions.
var allowedVideoExts = map[string]struct{}{
	".mp4": {}, ".avi": {}, ".mov": {}, ".mkv": {},
	".wmv": {}, ".flv": {}, ".webm": {},
}

// handleUpload accepts a multipart video, stores it in object storage, records a
// PENDING job, and enqueues it for a worker. It returns 202 Accepted: the work
// has been accepted but not yet done.
func (s *Server) handleUpload(c *gin.Context) {
	userID := userIDFrom(c)
	ctx := c.Request.Context()

	fileHeader, err := c.FormFile("video")
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("a 'video' file field is required"))
		return
	}
	if fileHeader.Size > s.cfg.MaxUploadBytes {
		c.JSON(http.StatusRequestEntityTooLarge, errorResponse(
			fmt.Sprintf("file exceeds the %d byte limit", s.cfg.MaxUploadBytes)))
		return
	}

	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if _, ok := allowedVideoExts[ext]; !ok {
		c.JSON(http.StatusBadRequest, errorResponse("unsupported format; use mp4, avi, mov, mkv, wmv, flv or webm"))
		return
	}

	file, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("could not read upload"))
		return
	}
	defer file.Close()

	// Namespace source objects by user, with a random name to avoid collisions.
	key := fmt.Sprintf("sources/%s/%s%s", userID, uuid.NewString(), ext)
	contentType := fileHeader.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	if err := s.objects.Put(ctx, key, file, fileHeader.Size, contentType); err != nil {
		slog.Error("upload to object store failed", "error", err, "key", key)
		c.JSON(http.StatusInternalServerError, errorResponse("could not store upload"))
		return
	}

	video := &domain.Video{UserID: userID, OriginalName: fileHeader.Filename, SourceKey: key}
	if err := s.videos.Create(ctx, video); err != nil {
		slog.Error("create video row failed", "error", err)
		c.JSON(http.StatusInternalServerError, errorResponse("could not record video"))
		return
	}

	job := messaging.VideoJob{
		VideoID:      video.ID,
		UserID:       userID,
		SourceKey:    key,
		OriginalName: video.OriginalName,
	}
	if err := s.publisher.PublishVideoJob(ctx, job); err != nil {
		// The row exists (PENDING) but was not enqueued. Surface the failure; a
		// future reconciler could re-publish orphaned PENDING jobs.
		slog.Error("publish job failed", "error", err, "video_id", video.ID)
		c.JSON(http.StatusInternalServerError, errorResponse("could not enqueue processing job"))
		return
	}

	c.JSON(http.StatusAccepted, newVideoResponse(*video, ""))
}

// handleList returns the caller's videos, newest first.
func (s *Server) handleList(c *gin.Context) {
	userID := userIDFrom(c)
	ctx := c.Request.Context()

	videos, err := s.videos.ListByUser(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("could not list videos"))
		return
	}

	resp := make([]videoResponse, 0, len(videos))
	for _, v := range videos {
		resp = append(resp, newVideoResponse(v, s.downloadURL(ctx, v)))
	}
	c.JSON(http.StatusOK, gin.H{"videos": resp})
}

// handleGet returns a single video the caller owns.
func (s *Server) handleGet(c *gin.Context) {
	video, ok := s.ownedVideo(c)
	if !ok {
		return // ownedVideo already wrote the response
	}
	c.JSON(http.StatusOK, newVideoResponse(video, s.downloadURL(c.Request.Context(), video)))
}

// handleDownload redirects to a presigned URL for the result zip once ready.
func (s *Server) handleDownload(c *gin.Context) {
	video, ok := s.ownedVideo(c)
	if !ok {
		return
	}
	if video.Status != domain.StatusDone || video.ZipKey == "" {
		c.JSON(http.StatusConflict, errorResponse("video is not ready for download"))
		return
	}

	url, err := s.objects.PresignGet(c.Request.Context(), video.ZipKey, s.cfg.PresignTTL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("could not generate download link"))
		return
	}
	c.Redirect(http.StatusFound, url)
}

// ownedVideo parses the :id param, loads the video, and enforces ownership. It
// returns false (having written the response) on any failure, so callers can
// simply `if !ok { return }`.
func (s *Server) ownedVideo(c *gin.Context) (domain.Video, bool) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid video id"))
		return domain.Video{}, false
	}

	video, err := s.videos.GetByID(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			c.JSON(http.StatusNotFound, errorResponse("video not found"))
			return domain.Video{}, false
		}
		c.JSON(http.StatusInternalServerError, errorResponse("could not load video"))
		return domain.Video{}, false
	}

	// Return 404 rather than 403 for someone else's video, so we don't reveal
	// that a video with that id exists.
	if video.UserID != userIDFrom(c) {
		c.JSON(http.StatusNotFound, errorResponse("video not found"))
		return domain.Video{}, false
	}
	return video, true
}

// downloadURL returns a presigned URL for a finished video, or "" otherwise.
// A presign failure is logged and treated as "no URL" rather than failing the
// whole listing.
func (s *Server) downloadURL(ctx context.Context, v domain.Video) string {
	if v.Status != domain.StatusDone || v.ZipKey == "" {
		return ""
	}
	url, err := s.objects.PresignGet(ctx, v.ZipKey, s.cfg.PresignTTL)
	if err != nil {
		slog.Warn("presign failed", "error", err, "video_id", v.ID)
		return ""
	}
	return url
}
