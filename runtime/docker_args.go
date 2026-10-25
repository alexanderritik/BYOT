package runtime

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// nobody: tests never run as root, even when the server does (e.g. in the
// published image, which needs root for the docker socket).
const sandboxUID, sandboxGID = 65534, 65534

// sandboxUser is the uid:gid tests run as: the server's own, or nobody when
// the server is root.
func sandboxUser() (int, int) {
	if os.Getuid() == 0 {
		return sandboxUID, sandboxGID
	}
	return os.Getuid(), os.Getgid()
}

// ChownWorkspace hands the workspace to the sandbox user. A no-op unless the
// server runs as root; otherwise the files already belong to the right uid.
func ChownWorkspace(workspace string) error {
	if os.Getuid() != 0 {
		return nil
	}
	uid, gid := sandboxUser()
	return filepath.WalkDir(workspace, func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		return os.Lchown(path, uid, gid)
	})
}

func dockerRunArgs(workspace string, spec RuntimeSpec) []string {
	network := "none"
	if spec.NetworkOverride != nil {
		if *spec.NetworkOverride {
			network = "bridge"
		} else {
			network = "none"
		}
	} else if spec.NetworkEnabled {
		network = "bridge"
	}

	memory := "512m"
	if spec.MemoryMB != nil && *spec.MemoryMB > 0 {
		memory = fmt.Sprintf("%dm", *spec.MemoryMB)
	}

	cpus := "1"
	if spec.CPUs != nil && *spec.CPUs > 0 {
		cpus = fmt.Sprintf("%.2f", *spec.CPUs)
	}

	var envArgs []string
	for _, name := range envNames(spec.Env) {
		// Name only: docker reads the value from its own environment, so
		// secrets never appear in the process list.
		envArgs = append(envArgs, "-e", name)
	}

	command := spec.Command
	// normalized in Execute; kept here for tests that call dockerRunArgs directly

	args := []string{
		"run",
		"--rm",
		"--network=" + network,
		"--memory=" + memory,
		"--cpus=" + cpus,
		"--pids-limit=128",
		// Sandbox hardening: untrusted code runs unprivileged, with no
		// capabilities, a read-only root fs and only /app and /tmp writable.
		"--cap-drop=ALL",
		"--security-opt=no-new-privileges",
		"--read-only",
		"--tmpfs=/tmp:rw,exec,nosuid,size=256m",
		// The uid that owns the mounted workspace (see ChownWorkspace);
		// CAP_DAC_OVERRIDE is dropped, so nothing can bypass file perms.
		userArg(),
		"-e",
		"HOME=/tmp",
		"-v",
		fmt.Sprintf("%s:/app", workspace),
		"-w",
		"/app",
		"--entrypoint",
		"sh",
	}
	if spec.Name != "" {
		args = append(args, "--name", spec.Name)
	}
	args = append(args, envArgs...)
	return append(args, spec.Image, "-c", command)
}

func userArg() string {
	uid, gid := sandboxUser()
	return fmt.Sprintf("--user=%d:%d", uid, gid)
}

func envNames(env map[string]string) []string {
	names := make([]string, 0, len(env))
	for n := range env {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
