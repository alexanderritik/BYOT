package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/alexanderritik/mini-lambda/runtime"
	"github.com/alexanderritik/mini-lambda/storage"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

const maxFileSize = 10 << 20

type Handler struct {
	storage storage.Storage
}

func NewHandler(storage storage.Storage) *Handler {
	return &Handler{storage: storage}
}

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

type RunRequest struct {
	Filename string `json:"filename"`
	Runtime  string `json:"runtime"`
	Timeout  int    `json:"timeout"`
}

func (hl *Handler) IsHealth(h http.ResponseWriter, r *http.Request) {
	jsonResponse(h, http.StatusOK, map[string]string{"status": "ok"})
}

func (hl *Handler) Run(h http.ResponseWriter, r *http.Request) {

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
	logger := log.With().
		Str("request_id", uuid.NewString()).
		Str("filename", req.Filename).
		Logger()

	logger.Info().Str("runtime", req.Runtime).Msg("function execution requested")

	// 1. Download from MinIO
	reader, err := hl.storage.DownloadBlob(req.Filename + "/binary")
	if err != nil {
		jsonResponse(h, http.StatusInternalServerError, map[string]string{"error": "failed to download binary"})
		return
	}

	// 2. Save to /tmp
	tmpPath := "/tmp/" + req.Filename
	dst, err := os.Create(tmpPath)
	if err != nil {
		jsonResponse(h, http.StatusInternalServerError, map[string]string{"error": "failed to create temp file"})
		return
	}
	io.Copy(dst, reader)
	dst.Close()

	// 3. Make executable + cleanup after
	os.Chmod(tmpPath, 0755)
	defer os.Remove(tmpPath)
	rt := runtime.GetRuntime(req.Runtime)
	if rt == nil {
		jsonResponse(h, http.StatusBadRequest, map[string]string{"error": "unsupported runtime"})
		return
	}

	start := time.Now()
	output, err := rt.Run(req.Filename, req.Timeout)
	duration := time.Since(start)
	if err != nil {
		logger.Error().Err(err).Int64("duration_ms", duration.Milliseconds()).Msg("function execution failed")
		jsonResponse(h, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	// Upload logs to MinIO
	logReader := bytes.NewReader(output)
	hl.storage.UploadLog(req.Filename, logReader, int64(len(output)))

	logger.Info().Int64("duration_ms", duration.Milliseconds()).Msg("function execution completed")
	jsonResponse(h, http.StatusOK, map[string]string{"output": string(output)})
}

func (hl *Handler) UploadBinary(h http.ResponseWriter, r *http.Request) {
	logger := log.With().
		Str("request_id", uuid.NewString()).
		Logger()

	if r.Method != "POST" {
		jsonResponse(h, http.StatusMethodNotAllowed, map[string]string{"error": "POST only accepted"})
		return
	}

	file, header, err := r.FormFile("binary")
	if err != nil {
		logger.Error().Err(err).Msg("failed to read uploaded file")
		jsonResponse(h, http.StatusInternalServerError, map[string]string{"error": "failed to read file"})
		return
	}

	logger = logger.With().
		Str("original_filename", header.Filename).
		Int64("size_bytes", header.Size).
		Logger()

	logger.Info().Msg("binary upload requested")

	if header.Size >= maxFileSize {
		logger.Warn().Msg("file too large, rejected")
		jsonResponse(h, http.StatusRequestEntityTooLarge, map[string]string{"error": "file too large"})
		return
	}

	fileName := uuid.NewString()
	dst, err := hl.storage.UploadBinary(fileName, file, header.Size)
	if err != nil {
		logger.Error().Err(err).Msg("failed to create file on disk")
		jsonResponse(h, http.StatusInternalServerError, map[string]string{"error": "failed to create file"})
		return
	}

	logger.Info().Str("uuid", fileName).Msg("binary uploaded successfully")
	jsonResponse(h, http.StatusOK, map[string]string{
		"id":      dst,
		"message": "binary uploaded successfully",
	})
}
