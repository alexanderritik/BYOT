package runtime

import (
	"context"
	"os"
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
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	args := []string{"run", "--rm", "-v", wd + "/uploads:/app", image}
	if command != "" {
		args = append(args, command)
	}
	args = append(args, "/app/"+filename)

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
	return out, err
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
