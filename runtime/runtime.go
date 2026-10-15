package runtime

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type RuntimeSpec struct {
	Image           string
	Command         string
	NetworkEnabled  bool
	MemoryMB        *int
	CPUs            *float64
	NetworkOverride *bool
	// Env is passed to the container by name; values travel via the docker
	// CLI's environment, never its arguments.
	Env map[string]string
	// Name lets the container be killed by name; killing only the docker CLI
	// process would leave it running.
	Name string
}

var runtimes = map[string]RuntimeSpec{
	"node": {
		Image:          ImageNode,
		Command:        "node ./artifact",
		NetworkEnabled: false,
	},
	"go": {
		Image:          ImageGo,
		Command:        "./artifact",
		NetworkEnabled: false,
	},
	"python": {
		Image:          ImagePython,
		Command:        "python ./artifact",
		NetworkEnabled: false,
	},
	"k6": {
		Image:          ImageK6,
		Command:        "k6 run ./artifact",
		NetworkEnabled: true,
	},
	// run.sh (staged by StageSupportFiles) handles single files and zip
	// bundles, fits the run inside the timeout and applies thresholds.
	// Extra arguments go to locust: sh /app/.byot/run.sh --tags smoke
	"locust": {
		Image:          ImageLocust,
		Command:        "sh /app/.byot/run.sh",
		NetworkEnabled: true,
	},
	"playwright": {
		Image:          ImagePlaywright,
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
	jobID string,
	workspace string,
	spec RuntimeSpec,
	timeout time.Duration,
) (Result, error) {
	spec.Command = strings.ReplaceAll(spec.Command, "./binary", "./artifact")
	spec.Name = ContainerName(jobID)
	args := dockerRunArgs(workspace, spec)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", args...)
	if len(spec.Env) > 0 {
		cmd.Env = os.Environ()
		for _, name := range envNames(spec.Env) {
			cmd.Env = append(cmd.Env, name+"="+spec.Env[name])
		}
	}

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
	registerActive(jobID, cmd)
	defer unregisterActive(jobID)

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
		KillContainer(spec.Name)
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

// ContainerName is the docker container name used for a job's run.
func ContainerName(jobID string) string {
	if jobID == "" {
		return ""
	}
	return "byot-" + jobID
}

// KillContainer force-removes a job's container (no-op if it doesn't exist).
func KillContainer(name string) {
	if name == "" {
		return
	}
	_ = exec.Command("docker", "rm", "-f", name).Run()
}
