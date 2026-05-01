package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"

	"github.com/alexanderritik/mini-lambda/runtime"
)

func IsHealth(h http.ResponseWriter, r *http.Request) {
	h.WriteHeader(http.StatusOK)
	h.Write([]byte("ok"))

}

type RunRequest struct {
	Filename string `json:"filename"`
	Runtime  string `json:"runtime"`
	Timeout  int    `json:"timeout"`
}

func Run(h http.ResponseWriter, r *http.Request) {
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

	rt := runtime.GetRuntime(req.Runtime)
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

func UploadBinary(h http.ResponseWriter, r *http.Request) {
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
