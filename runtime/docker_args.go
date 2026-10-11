package runtime

import (
	"fmt"
	"os"
	"sort"
)

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
		// Same uid as the host process so it owns the mounted workspace
		// (CAP_DAC_OVERRIDE is dropped, so root-in-container can't bypass perms).
		fmt.Sprintf("--user=%d:%d", os.Getuid(), os.Getgid()),
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

func envNames(env map[string]string) []string {
	names := make([]string, 0, len(env))
	for n := range env {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
