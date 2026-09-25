package runtime

import (
	"testing"
)

func TestDockerRunArgs_NetworkDisabled(t *testing.T) {
	args := dockerRunArgs("/tmp/ws", RuntimeSpec{
		Image:          "alpine",
		Command:        "./artifact",
		NetworkEnabled: false,
	})
	if !contains(args, "--network=none") {
		t.Fatalf("expected --network=none, got %v", args)
	}
}

func TestDockerRunArgs_NetworkEnabled(t *testing.T) {
	args := dockerRunArgs("/tmp/ws", RuntimeSpec{
		Image:          "grafana/k6:latest",
		Command:        "k6 run ./artifact",
		NetworkEnabled: true,
	})
	if !contains(args, "--network=bridge") {
		t.Fatalf("expected --network=bridge, got %v", args)
	}
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
