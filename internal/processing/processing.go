// Package processing turns a video file into a zip of extracted frames. It is
// deliberately free of any messaging, storage, or database concerns — it takes
// file paths in and produces a zip on disk — so it can be unit-tested in
// isolation and reused anywhere.
package processing

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Result describes a successful extraction.
type Result struct {
	ZipPath    string
	FrameCount int
}

// Run extracts frames from srcPath at the given fps and zips them, writing all
// intermediate files under workDir. The caller owns workDir and should remove it
// afterwards. The context bounds the ffmpeg invocation — cancelling it kills the
// child process.
func Run(ctx context.Context, ffmpegPath, srcPath, workDir string, fps int) (Result, error) {
	framesDir := filepath.Join(workDir, "frames")
	if err := os.MkdirAll(framesDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("create frames dir: %w", err)
	}

	frames, err := extractFrames(ctx, ffmpegPath, srcPath, framesDir, fps)
	if err != nil {
		return Result{}, err
	}
	if len(frames) == 0 {
		return Result{}, errors.New("no frames were extracted from the video")
	}

	zipPath := filepath.Join(workDir, "frames.zip")
	if err := ZipFiles(frames, zipPath); err != nil {
		return Result{}, err
	}

	return Result{ZipPath: zipPath, FrameCount: len(frames)}, nil
}

// extractFrames shells out to ffmpeg to write one PNG per (1/fps) seconds of
// video, then returns the sorted list of produced frame files.
func extractFrames(ctx context.Context, ffmpegPath, srcPath, framesDir string, fps int) ([]string, error) {
	pattern := filepath.Join(framesDir, "frame_%05d.png")

	// CommandContext ties the process lifetime to ctx: if ctx is cancelled (job
	// timeout or shutdown), Go sends SIGKILL to ffmpeg instead of leaking it.
	cmd := exec.CommandContext(ctx, ffmpegPath,
		"-i", srcPath,
		"-vf", fmt.Sprintf("fps=%d", fps),
		"-y", // overwrite without prompting
		pattern,
	)

	// ffmpeg writes progress/errors to stderr; capture both for diagnostics.
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg failed: %w: %s", err, lastLines(output, 400))
	}

	return filepath.Glob(filepath.Join(framesDir, "*.png"))
}

// lastLines returns up to the final n bytes of ffmpeg output, so error messages
// carry useful context without dumping the entire (often verbose) log.
func lastLines(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return "..." + string(b[len(b)-n:])
}
