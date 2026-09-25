package runtime

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type RuntimeSpec struct {
	Image          string
	Command        string
	NetworkEnabled bool
}

var runtimes = map[string]RuntimeSpec{
	"node": {
		Image:          "node:18-alpine",
		Command:        "node ./artifact",
		NetworkEnabled: false,
	},
	"go": {
		Image:          "golang:1.24-alpine",
		Command:        "./artifact",
		NetworkEnabled: false,
	},
	"python": {
		Image:          "python:3.12-alpine",
		Command:        "python ./artifact",
		NetworkEnabled: false,
	},
	"k6": {
		Image:          "grafana/k6:latest",
		Command:        "k6 run ./artifact",
		NetworkEnabled: true,
	},
	"playwright": {
		Image:          "mcr.microsoft.com/playwright:v1.49.1-jammy",
		Command:        "npx playwright test",
		NetworkEnabled: true,
	},
}

func GetRuntime(name string) (RuntimeSpec, bool) {
	spec, ok := runtimes[name]
	return spec, ok
}

type Result struct {
	Output   []byte
	ExitCode int
}

func Execute(
	workspace string,
	spec RuntimeSpec,
	timeout time.Duration,
) (Result, error) {
	spec.Command = strings.ReplaceAll(spec.Command, "./binary", "./artifact")
	args := dockerRunArgs(workspace, spec)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return Result{ExitCode: -1}, fmt.Errorf("create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return Result{ExitCode: -1}, fmt.Errorf("create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return Result{ExitCode: -1}, fmt.Errorf("start docker: %w", err)
	}

	var (
		buf bytes.Buffer
		mu  sync.Mutex
		wg  sync.WaitGroup
	)

	readOutput := func(r *bufio.Scanner, stream string) {
		defer wg.Done()
		for r.Scan() {
			mu.Lock()
			fmt.Fprintf(
				&buf,
				"%s [%s] %s\n",
				time.Now().UTC().Format(time.RFC3339),
				stream,
				r.Text(),
			)
			mu.Unlock()
		}
	}

	wg.Add(2)
	go readOutput(bufio.NewScanner(stdout), "stdout")
	go readOutput(bufio.NewScanner(stderr), "stderr")

	waitErr := cmd.Wait()
	wg.Wait()

	result := Result{Output: buf.Bytes()}

	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		result.ExitCode = -1
		return result, fmt.Errorf("execution timed out after %s", timeout)
	}

	var exitErr *exec.ExitError
	if errors.As(waitErr, &exitErr) {
		result.ExitCode = exitErr.ExitCode()
		return result, nil
	}

	if waitErr != nil {
		result.ExitCode = -1
		return result, fmt.Errorf("docker execution failed: %w", waitErr)
	}

	result.ExitCode = 0
	return result, nil
}
