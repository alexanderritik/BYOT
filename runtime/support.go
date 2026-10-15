package runtime

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// SupportDir is where runtime support files land in the workspace; the
// workspace is mounted at /app, so the container sees /app/.byot.
const SupportDir = ".byot"

//go:embed locust/run.sh locust/byot_checks.py
var locustFiles embed.FS

var supportFiles = map[string]embed.FS{
	"locust": locustFiles,
}

// StageSupportFiles writes the files a runtime's default command relies on
// into workspace/.byot. Runtimes without support files are a no-op.
func StageSupportFiles(runtimeName, workspace string) error {
	files, ok := supportFiles[runtimeName]
	if !ok {
		return nil
	}
	dir := filepath.Join(workspace, SupportDir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return fs.WalkDir(files, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := files.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dir, filepath.Base(path)), data, 0644)
	})
}

// IsBundle reports whether a runtime's artifact is a zip that gets unpacked
// into the workspace. Locust takes either a single locustfile or a zip.
func IsBundle(runtimeName, originalFilename string) bool {
	switch runtimeName {
	case "playwright":
		return true
	case "locust":
		return strings.EqualFold(filepath.Ext(originalFilename), ".zip")
	}
	return false
}
