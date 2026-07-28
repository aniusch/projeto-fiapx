package processing

import (
	"archive/zip"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestZipFiles(t *testing.T) {
	dir := t.TempDir() // auto-removed when the test finishes

	// Create a few fake "frame" files with known contents.
	want := map[string]string{
		"frame_00001.png": "first",
		"frame_00002.png": "second",
		"frame_00003.png": "third",
	}
	var files []string
	for name, content := range want {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		files = append(files, p)
	}
	sort.Strings(files)

	zipPath := filepath.Join(dir, "out.zip")
	if err := ZipFiles(files, zipPath); err != nil {
		t.Fatalf("ZipFiles: %v", err)
	}

	// Re-open the archive and verify every entry is present with flat names and
	// intact contents.
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	defer zr.Close()

	if len(zr.File) != len(want) {
		t.Fatalf("zip has %d entries, want %d", len(zr.File), len(want))
	}
	for _, f := range zr.File {
		if filepath.Base(f.Name) != f.Name {
			t.Errorf("entry %q is not flat (should be base name only)", f.Name)
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open entry %s: %v", f.Name, err)
		}
		got, _ := os.ReadFile(filepath.Join(dir, f.Name)) // original on disk
		buf := make([]byte, len(got))
		_, _ = rc.Read(buf)
		rc.Close()
		if string(buf) != want[f.Name] {
			t.Errorf("entry %s content = %q, want %q", f.Name, string(buf), want[f.Name])
		}
	}
}
