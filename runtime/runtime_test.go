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

func TestDockerRunArgs_Hardening(t *testing.T) {
	args := dockerRunArgs("/tmp/ws", RuntimeSpec{Image: "alpine", Command: "./artifact"})
	for _, want := range []string{"--cap-drop=ALL", "--security-opt=no-new-privileges", "--read-only"} {
		if !contains(args, want) {
			t.Fatalf("expected %s, got %v", want, args)
		}
	}
}

func TestDockerRunArgs_EnvNamesOnly(t *testing.T) {
	args := dockerRunArgs("/tmp/ws", RuntimeSpec{
		Image:   "alpine",
		Command: "./artifact",
		Env:     map[string]string{"API_KEY": "s3cr3t-value", "BASE_URL": "https://x.test"},
	})
	for _, a := range args {
		if a == "s3cr3t-value" || a == "https://x.test" || a == "API_KEY=s3cr3t-value" {
			t.Fatalf("env value leaked into args: %v", args)
		}
	}
	if !contains(args, "API_KEY") || !contains(args, "BASE_URL") {
		t.Fatalf("expected -e NAME flags, got %v", args)
	}
	// options must precede the image
	img := -1
	for i, a := range args {
		if a == "alpine" {
			img = i
		}
	}
	for i, a := range args {
		if a == "API_KEY" && i > img {
			t.Fatalf("-e after image: %v", args)
		}
	}
}
