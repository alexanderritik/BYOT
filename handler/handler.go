package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"

	"github.com/alexanderritik/mini-lambda/runtime"
	"github.com/google/uuid"
)

const maxFileSize = 10 << 20

func jsonResponse(h http.ResponseWriter, status int, v any) {
	h.Header().Set("Content-Type", "application/json")
	h.WriteHeader(status)
	val, err := json.Marshal(v)
	if err != nil {
		h.Write([]byte(`{"error":"internal error"}`))
		return
	}
	h.Write(val)
}

func IsHealth(h http.ResponseWriter, r *http.Request) {
	jsonResponse(h, http.StatusOK, map[string]string{"status": "ok"})
}

type RunRequest struct {
	Filename string `json:"filename"`
	Runtime  string `json:"runtime"`
	Timeout  int    `json:"timeout"`
}

func Run(h http.ResponseWriter, r *http.Request) {
	var req RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(h, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.Filename == "" || req.Runtime == "" {
		jsonResponse(h, http.StatusBadRequest, map[string]string{"error": "filename and runtime are required"})
		return
	}
	if req.Timeout == 0 {
		req.Timeout = 30
	}
	rt := runtime.GetRuntime(req.Runtime)
	if rt == nil {
		jsonResponse(h, http.StatusBadRequest, map[string]string{"error": "unsupported runtime"})
		return
	}
	output, err := rt.Run(req.Filename, req.Timeout)
	if err != nil {
		jsonResponse(h, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	jsonResponse(h, http.StatusOK, map[string]string{"output": string(output)})
}

func UploadBinary(h http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		jsonResponse(h, http.StatusMethodNotAllowed, map[string]string{"error": "POST only accepted"})
		return
	}
	file, header, err := r.FormFile("binary")
	if err != nil {
		jsonResponse(h, http.StatusInternalServerError, map[string]string{"error": "failed to read file"})
		return
	}
	if header.Size >= maxFileSize {
		jsonResponse(h, http.StatusRequestEntityTooLarge, map[string]string{"error": "file too large"})
		return
	}
	fileName := uuid.NewString()
	dst, err := os.Create("uploads/" + fileName)
	if err != nil {
		jsonResponse(h, http.StatusInternalServerError, map[string]string{"error": "failed to create file"})
		return
	}
	defer dst.Close()
	io.Copy(dst, file)
	exec.Command("chmod", "+x", "uploads/"+fileName).Run()
	jsonResponse(h, http.StatusOK, map[string]string{
		"id":      fileName,
		"message": "binary uploaded successfully",
	})
}
