package worker

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadWritesObjectToFile(t *testing.T) {
	o := &fakeObjects{}
	w := newTestWorker(t, o, &fakeEvents{})

	dst := filepath.Join(t.TempDir(), "source.mp4")
	if err := w.download(context.Background(), "sources/x.mp4", dst); err != nil {
		t.Fatalf("download: %v", err)
	}
	b, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(b) != "not a real video" {
		t.Fatalf("downloaded content = %q", string(b))
	}
}

func TestUploadZipStoresObject(t *testing.T) {
	o := &fakeObjects{}
	w := newTestWorker(t, o, &fakeEvents{})

	path := filepath.Join(t.TempDir(), "frames.zip")
	if err := os.WriteFile(path, []byte("zip-bytes"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := w.uploadZip(context.Background(), "results/x.zip", path); err != nil {
		t.Fatalf("uploadZip: %v", err)
	}
	if len(o.putKeys) != 1 || o.putKeys[0] != "results/x.zip" {
		t.Fatalf("expected put of results/x.zip, got %v", o.putKeys)
	}
}
