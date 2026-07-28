//go:build integration

// Integration test for the full ffmpeg extraction + zip pipeline. It generates
// its own input video with ffmpeg, so it needs ffmpeg on PATH; if ffmpeg is
// absent (e.g. a dev host without it) the test skips rather than fails.
//
//	go test -tags=integration ./internal/processing/...
package processing

import (
	"archive/zip"
	"context"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestRunExtractsAndZipsFrames(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed; skipping pipeline integration test")
	}

	dir := t.TempDir()
	src := filepath.Join(dir, "src.mp4")

	// Generate a 2-second synthetic clip.
	gen := exec.Command("ffmpeg",
		"-f", "lavfi", "-i", "testsrc=duration=2:size=160x120:rate=5",
		"-pix_fmt", "yuv420p", "-y", src)
	if out, err := gen.CombinedOutput(); err != nil {
		t.Fatalf("generate test video: %v\n%s", err, out)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// fps=1 over 2 seconds -> 2 frames.
	result, err := Run(ctx, "ffmpeg", src, dir, 1)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.FrameCount != 2 {
		t.Fatalf("FrameCount = %d, want 2", result.FrameCount)
	}

	zr, err := zip.OpenReader(result.ZipPath)
	if err != nil {
		t.Fatalf("open result zip: %v", err)
	}
	defer zr.Close()
	if len(zr.File) != 2 {
		t.Fatalf("zip has %d entries, want 2", len(zr.File))
	}
}
