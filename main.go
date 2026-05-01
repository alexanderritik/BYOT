package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"
)

func isHealth(h http.ResponseWriter, r *http.Request) {
	h.WriteHeader(http.StatusOK)
	h.Write([]byte("ok"))

}

type RunRequest struct {
	Filename string `json:"filename"`
	Runtime  string `json:"runtime"`
	Timeout  int    `json:"timeout"`
}
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

func getRuntime(runtime string) Runtime {
	switch runtime {
	case "go":
		return GoRuntime{Image: "alpine", Command: ""}
	case "node":
		return NodeRuntime{}
	default:
		return nil
	}
}

func run(h http.ResponseWriter, r *http.Request) {
	var req RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.WriteHeader(http.StatusBadRequest)
		h.Write([]byte("invalid request body"))
		return
	}

	if req.Filename == "" || req.Runtime == "" {
		h.WriteHeader(http.StatusBadRequest)
		h.Write([]byte("filename and runtime is required"))
		return
	}
	if req.Timeout == 0 {
		req.Timeout = 30
	}

	rt := getRuntime(req.Runtime)
	if rt == nil {
		h.WriteHeader(http.StatusBadRequest)
		h.Write([]byte("unsupported runtime"))
		return
	}

	output, err := rt.Run(req.Filename, req.Timeout)
	if err != nil {
		h.WriteHeader(http.StatusInternalServerError)
		h.Write([]byte("failed: " + err.Error()))
		return
	}
	h.WriteHeader(http.StatusOK)
	h.Write([]byte(output))
}

func uploadBinary(h http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		h.WriteHeader(http.StatusMethodNotAllowed)
		h.Write([]byte("POST only accepted"))
		return
	}

	file, header, err := r.FormFile("binary")
	if err != nil {
		h.WriteHeader(http.StatusInternalServerError)
		h.Write([]byte("failed to read file"))
		return
	}

	dst, err := os.Create("uploads/" + header.Filename)
	if err != nil {
		h.WriteHeader(http.StatusInternalServerError)
		h.Write([]byte("failed to create file"))
		return
	}
	defer dst.Close()

	io.Copy(dst, file)

	cmd := exec.Command("chmod", "+x", "uploads/"+header.Filename)
	cmd.Run()
	h.WriteHeader(http.StatusOK)
	h.Write([]byte("binary uploaded: " + header.Filename))
}

func main() {

	http.HandleFunc("/health", isHealth)
	http.HandleFunc("/uploadBinary", uploadBinary)
	http.HandleFunc("/run/", run)
	http.ListenAndServe(":3000", nil)
	//
}
