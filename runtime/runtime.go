package runtime

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"os/exec"
	"sync"
	"time"
)

type Runtime interface {
	// exitCode is the test binary's own exit code (0 = pass, non-zero = fail).
	// err is only set for infra-level failures (docker missing, timeout, etc).
	Run(filename string, timeout int) (output []byte, exitCode int, err error)
}
type GoRuntime struct {
	Image   string // "alpine"
	Command string // "" (just execute directly)
}

type NodeRuntime struct {
	Image   string // "node:18"
	Command string // "node"
}

func dockerRun(image, command, filename string, timeout int) ([]byte, int, error) {
	args := []string{"run", "--rm", "-v", "/tmp/" + filename + ":/app/binary", image}
	if command != "" {
		args = append(args, command)
	}
	args = append(args, "/app/binary")
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	var buf bytes.Buffer
	var mu sync.Mutex
	cmd := exec.CommandContext(ctx, "docker", args...)

	stdoutPipe, _ := cmd.StdoutPipe()
	stderrPipe, _ := cmd.StderrPipe()
	cmd.Start()
	wg.Add(1)
	go func() {
		defer wg.Done() // "I'm done"
		scanner := bufio.NewScanner(stderrPipe)
		for scanner.Scan() {
			mu.Lock()
			buf.WriteString(time.Now().Format(time.RFC3339) + " [stderr] " + scanner.Text() + "\n")
			mu.Unlock()
		}
	}()

	scanner := bufio.NewScanner(stdoutPipe)
	for scanner.Scan() {
		line := scanner.Text()
		mu.Lock()
		buf.WriteString(time.Now().Format(time.RFC3339) + " [stdout] " + line + "\n")
		mu.Unlock()
	}

	// Wait for process to finish
	wg.Wait()
	waitErr := cmd.Wait()

	var exitErr *exec.ExitError
	if errors.As(waitErr, &exitErr) {
		// Non-zero exit is the test binary failing, not an infra error.
		return buf.Bytes(), exitErr.ExitCode(), nil
	}
	if waitErr != nil {
		return buf.Bytes(), -1, waitErr
	}

	return buf.Bytes(), 0, nil
}

func (g GoRuntime) Run(filename string, timeout int) ([]byte, int, error) {
	return dockerRun(g.Image, g.Command, filename, timeout)
}
func (g NodeRuntime) Run(filename string, timeout int) ([]byte, int, error) {
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
