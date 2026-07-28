package processing

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ZipFiles writes the given files into a single deflate-compressed zip archive
// at zipPath. Each entry is stored under just its base name (no directory
// structure), so the archive unpacks to a flat folder of frames.
//
// This is ported from the original monolith, but split out as a pure function so
// it can be tested without ffmpeg or a real video.
func ZipFiles(files []string, zipPath string) error {
	zipFile, err := os.Create(zipPath)
	if err != nil {
		return fmt.Errorf("create zip: %w", err)
	}
	defer zipFile.Close()

	zw := zip.NewWriter(zipFile)
	defer zw.Close()

	for _, file := range files {
		if err := addToZip(zw, file); err != nil {
			return err
		}
	}
	return nil
}

func addToZip(zw *zip.Writer, filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("open %s: %w", filename, err)
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat %s: %w", filename, err)
	}

	header, err := zip.FileInfoHeader(info)
	if err != nil {
		return fmt.Errorf("zip header for %s: %w", filename, err)
	}
	header.Name = filepath.Base(filename)
	header.Method = zip.Deflate

	writer, err := zw.CreateHeader(header)
	if err != nil {
		return fmt.Errorf("create zip entry for %s: %w", filename, err)
	}
	if _, err := io.Copy(writer, file); err != nil {
		return fmt.Errorf("write zip entry for %s: %w", filename, err)
	}
	return nil
}
