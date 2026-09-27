package bundle

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// UnzipFile extracts zipPath into destDir. Paths must not escape destDir (zip slip).
// If every entry sits under one top-level directory, that prefix is stripped so
// package.json at the project root ends up directly in destDir.
func UnzipFile(destDir, zipPath string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	strip := singleRootPrefix(r.File)
	destClean := filepath.Clean(destDir)
	if err := os.MkdirAll(destClean, 0755); err != nil {
		return err
	}

	for _, f := range r.File {
		name := f.Name
		if strip != "" {
			if name == strip {
				continue
			}
			if strings.HasPrefix(name, strip) {
				name = strings.TrimPrefix(name, strip)
			}
		}
		name = path.Clean(name)
		if name == "." || name == "" {
			continue
		}
		if strings.HasPrefix(name, "..") || path.IsAbs(name) {
			return fmt.Errorf("illegal path in zip: %s", f.Name)
		}
		target := filepath.Join(destClean, filepath.FromSlash(name))
		if !withinDir(destClean, target) {
			return fmt.Errorf("zip slip: %s", f.Name)
		}

		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		if err := extractFile(f, target); err != nil {
			return err
		}
	}
	return nil
}

func extractFile(f *zip.File, target string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode()&0777|0200)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, rc)
	return err
}

func withinDir(base, target string) bool {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

func singleRootPrefix(files []*zip.File) string {
	var root string
	for _, f := range files {
		p := path.Clean(f.Name)
		if p == "." {
			continue
		}
		parts := strings.Split(strings.TrimSuffix(p, "/"), "/")
		if len(parts) == 0 || parts[0] == "" || parts[0] == ".." {
			return ""
		}
		if root == "" {
			root = parts[0]
			continue
		}
		if parts[0] != root {
			return ""
		}
	}
	if root == "" {
		return ""
	}
	return root + "/"
}
