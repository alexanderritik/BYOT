package runtime

import "fmt"

func dockerRunArgs(workspace string, spec RuntimeSpec) []string {
	network := "none"
	if spec.NetworkEnabled {
		network = "bridge"
	}

	command := spec.Command
	// normalized in Execute; kept here for tests that call dockerRunArgs directly

	return []string{
		"run",
		"--rm",
		"--network=" + network,
		"--memory=512m",
		"--cpus=1",
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
