package bundle

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestUnzipFile_stripsSingleRoot(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "bundle.zip")
	dest := filepath.Join(dir, "out")

	mkZip(zipPath, map[string]string{
		"proj/package.json": `{"name":"proj"}`,
		"proj/tests/a.spec.js": "x",
	})

	if err := UnzipFile(dest, zipPath); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "package.json")); err != nil {
		t.Fatalf("expected package.json at dest root: %v", err)
	}
}

func TestUnzipFile_rejectsZipSlip(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "evil.zip")
	dest := filepath.Join(dir, "out")

	mkZip(zipPath, map[string]string{
		"../escape.txt": "nope",
	})

	err := UnzipFile(dest, zipPath)
	if err == nil {
		t.Fatal("expected zip slip error")
	}
}

func mkZip(path string, files map[string]string) {
	f, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	w := zip.NewWriter(f)
	for name, body := range files {
		zw, err := w.Create(name)
		if err != nil {
			panic(err)
		}
		if _, err := zw.Write([]byte(body)); err != nil {
			panic(err)
		}
	}
	if err := w.Close(); err != nil {
		panic(err)
	}
	if err := f.Close(); err != nil {
		panic(err)
	}
}
