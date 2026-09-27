package runtime

import "fmt"

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
