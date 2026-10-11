package worker

import (
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// maxScreenshotsPerRun caps uploads so a misbehaving bundle can't flood storage.
const maxScreenshotsPerRun = 50

// skipScreenshotDirs are never scanned: dependencies, VCS data, and the HTML
// report (it holds copies of the same images already in test-results).
var skipScreenshotDirs = map[string]bool{
	"node_modules":      true,
	".git":              true,
	"playwright-report": true,
	"blob-report":       true,
}

func isScreenshotFile(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".webp":
		return true
	}
	return false
}

// imageSnapshot maps workspace-relative image paths to their modification time.
type imageSnapshot map[string]time.Time

// snapshotImages records every image in the workspace, so images shipped in
// the bundle (fixtures, visual-regression baselines) can be told apart from
// screenshots the run produced.
func snapshotImages(workspace string) imageSnapshot {
	snap := imageSnapshot{}
	walkImages(workspace, func(rel string, mod time.Time) {
		snap[rel] = mod
	})
	return snap
}

// newScreenshots returns workspace-relative paths of images created or
// modified since the snapshot, sorted, and capped at maxScreenshotsPerRun.
func newScreenshots(workspace string, before imageSnapshot) (paths []string, truncated int) {
	walkImages(workspace, func(rel string, mod time.Time) {
		if prev, ok := before[rel]; ok && prev.Equal(mod) {
			return
		}
		paths = append(paths, rel)
	})
	sort.Strings(paths)
	if len(paths) > maxScreenshotsPerRun {
		truncated = len(paths) - maxScreenshotsPerRun
		paths = paths[:maxScreenshotsPerRun]
	}
	return paths, truncated
}

func walkImages(workspace string, visit func(rel string, mod time.Time)) {
	_ = filepath.WalkDir(workspace, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			// Hidden dirs include test-results/.playwright-artifacts-N, where
			// traces keep screencast frames and captured site images; they are
			// left behind when a run is killed before Playwright cleans up.
			if path != workspace && (skipScreenshotDirs[d.Name()] || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.Type().IsRegular() || !isScreenshotFile(d.Name()) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(workspace, path)
		if err != nil {
			return nil
		}
		visit(filepath.ToSlash(rel), info.ModTime())
		return nil
	})
}
