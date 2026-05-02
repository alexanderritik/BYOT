package runtime

import (
	"context"
	"os/exec"
	"time"
)

type Runtime interface {
	Run(filename string, timeout int) ([]byte, error)
}
type GoRuntime struct {
	Image   string // "alpine"
	Command string // "" (just execute directly)
}

type NodeRuntime struct {
	Image   string // "node:18"
	Command string // "node"
}

func dockerRun(image, command, filename string, timeout int) ([]byte, error) {
	args := []string{"run", "--rm", "-v", "/tmp/" + filename + ":/app/binary", image}
	if command != "" {
		args = append(args, command)
	}
	args = append(args, "/app/binary")
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "docker", args...).CombinedOutput()
}

func (g GoRuntime) Run(filename string, timeout int) ([]byte, error) {
	return dockerRun(g.Image, g.Command, filename, timeout)
}
func (g NodeRuntime) Run(filename string, timeout int) ([]byte, error) {
	return dockerRun(g.Image, g.Command, filename, timeout)
}

func GetRuntime(runtime string) Runtime {
	switch runtime {
	case "go":
		return GoRuntime{Image: "alpine", Command: ""}
	case "node":
		return NodeRuntime{}
	default:
		return nil
	}
}
