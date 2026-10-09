package runtime

import (
	"fmt"
	"os"
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

	command := spec.Command
	// normalized in Execute; kept here for tests that call dockerRunArgs directly

	return []string{
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
		spec.Image,
		"-c",
		command,
	}
}
