package worker

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func writeFile(t *testing.T, root, rel string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("img"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestNewScreenshotsOnlyReturnsImagesFromTheRun(t *testing.T) {
	ws := t.TempDir()
	// Shipped in the bundle: must never be reported.
	writeFile(t, ws, "node_modules/playwright-core/lib/icon.png")
	writeFile(t, ws, "tests/fixtures/logo.png")
	writeFile(t, ws, "tests/home.spec.ts-snapshots/hero.png")
	before := snapshotImages(ws)

	// Produced by the run.
	writeFile(t, ws, "test-results/home-desktop/test-finished-1.png")
	writeFile(t, ws, "test-results/contact-desktop/test-finished-1.png")
	writeFile(t, ws, "screenshots/manual.jpg")
	writeFile(t, ws, "debug.webp")
	writeFile(t, ws, "playwright-report/data/abc123.png") // duplicate copy
	writeFile(t, ws, "node_modules/x/new.png")
	writeFile(t, ws, "test-results/.playwright-artifacts-0/traces/resources/page@c059-1791708644799.jpeg")
	writeFile(t, ws, "test-results/.playwright-artifacts-0/traces/resources/396d7c90.webp")
	writeFile(t, ws, "test-results/trace.zip")

	got, truncated := newScreenshots(ws, before)
	want := []string{
		"debug.webp",
		"screenshots/manual.jpg",
		"test-results/contact-desktop/test-finished-1.png",
		"test-results/home-desktop/test-finished-1.png",
	}
	if !reflect.DeepEqual(got, want) || truncated != 0 {
		t.Fatalf("got %v (truncated %d), want %v", got, truncated, want)
	}
}

func TestNewScreenshotsIncludesBundledImageOverwrittenByRun(t *testing.T) {
	ws := t.TempDir()
	writeFile(t, ws, "screenshots/home.png")
	before := snapshotImages(ws)

	later := time.Now().Add(time.Minute)
	if err := os.Chtimes(filepath.Join(ws, "screenshots/home.png"), later, later); err != nil {
		t.Fatal(err)
	}

	got, _ := newScreenshots(ws, before)
	if !reflect.DeepEqual(got, []string{"screenshots/home.png"}) {
		t.Fatalf("got %v", got)
	}
}

func TestNewScreenshotsSkipsSymlinks(t *testing.T) {
	ws := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.png")
	if err := os.WriteFile(outside, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	before := snapshotImages(ws)
	if err := os.Symlink(outside, filepath.Join(ws, "leak.png")); err != nil {
		t.Fatal(err)
	}

	if got, _ := newScreenshots(ws, before); len(got) != 0 {
		t.Fatalf("symlink must not be uploaded, got %v", got)
	}
}

func TestNewScreenshotsCapsCount(t *testing.T) {
	ws := t.TempDir()
	before := snapshotImages(ws)
	for i := 0; i < maxScreenshotsPerRun+5; i++ {
		writeFile(t, ws, fmt.Sprintf("test-results/shot-%03d.png", i))
	}

	got, truncated := newScreenshots(ws, before)
	if len(got) != maxScreenshotsPerRun || truncated != 5 {
		t.Fatalf("got %d paths, truncated %d", len(got), truncated)
	}
}
