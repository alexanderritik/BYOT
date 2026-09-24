package handler

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/alexanderritik/mini-lambda/model"
	"github.com/alexanderritik/mini-lambda/queue"
	"github.com/alexanderritik/mini-lambda/repository"
	"github.com/alexanderritik/mini-lambda/storage"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

const maxFileSize = 10 << 20

type Handler struct {
	storage     storage.Storage
	test        repository.TestRepository
	testRunRepo repository.TestRunRepository
	queue       *queue.Queue
}

func NewHandler(storage storage.Storage, test repository.TestRepository, testRun repository.TestRunRepository, q *queue.Queue) *Handler {
	return &Handler{
		storage:     storage,
		test:        test,
		testRunRepo: testRun,
		queue:       q,
	}
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
	TestId string `json:"testId"`
}

func (hl *Handler) IsHealth(h http.ResponseWriter, r *http.Request) {
	jsonResponse(h, http.StatusOK, map[string]string{"status": "ok"})
}

func (hl *Handler) JobStatus(h http.ResponseWriter, r *http.Request) {
	jobID := r.URL.Path[len("/status/"):]
	if jobID == "" {
		jsonResponse(h, http.StatusBadRequest, map[string]string{"error": "job ID is missing"})
		return
	}

	job, err := hl.queue.GetStatus(r.Context(), jobID)
	if err != nil {
		jsonResponse(h, http.StatusNotFound, map[string]string{"error": "job not found"})
		return
	}

	response := map[string]interface{}{
		"job_id":      job.UUID,
		"test_id":     job.TestID,
		"status":      job.Status,
		"queued_at":   job.QueuedAt,
		"started_at":  job.StartedAt,
		"finished_at": job.FinishedAt,
		"worker_id":   job.WorkerID,
	}
	if job.ErrorMessage != nil {
		response["error"] = *job.ErrorMessage
	}

	jsonResponse(h, http.StatusOK, response)
}

func (hl *Handler) Run(h http.ResponseWriter, r *http.Request) {

	var req RunRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(h, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.TestId == "" {
		jsonResponse(h, http.StatusBadRequest, map[string]string{"error": "Test UUID is missing"})
		return
	}
	logger := log.With().
		Str("request_id", uuid.NewString()).
		Str("filename", req.TestId).
		Logger()

	// Verify test exists
	testRes, err := hl.test.GetByID(r.Context(), req.TestId)
	if err != nil {
		jsonResponse(h, http.StatusNotFound, map[string]string{
			"error": "Request Test Id not found",
		})
		return
	}

	logger.Info().Str("runtime", testRes.Runtime).Msg("function execution requested")

	// Enqueue job instead of executing synchronously
	job, err := hl.queue.Enqueue(r.Context(), testRes.UUID)
	if err != nil {
		jsonResponse(h, http.StatusInternalServerError, map[string]string{"error": "failed to enqueue job"})
		return
	}

	// Return 202 Accepted with job ID
	jsonResponse(h, http.StatusAccepted, map[string]string{
		"job_id":  job.UUID,
		"status":  "queued",
		"message": "job queued for execution",
	})
}

func (hl *Handler) UploadBinary(h http.ResponseWriter, r *http.Request) {
	logger := log.With().
		Str("request_id", uuid.NewString()).
		Logger()

	if r.Method != "POST" {
		jsonResponse(h, http.StatusMethodNotAllowed, map[string]string{"error": "POST only accepted"})
		return
	}
	runtime := r.FormValue("runtime")
	severity := r.FormValue("severity")
	command := r.FormValue("command")
	timeoutStr := r.FormValue("timeout")
	timeout, err := strconv.Atoi(timeoutStr)
	if err != nil || timeout == 0 {
		timeout = 30 // default
	}
	if runtime == "" || severity == "" || command == "" {
		logger.Error().Msg("We required Runtime, Severity, and Command in input.")
		jsonResponse(h, http.StatusBadRequest, map[string]string{"error": "required Runtime, Severity, and Command in input"})
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
	dst, err := hl.storage.UploadArtifact(fileName, file, header.Size)
	if err != nil {
		logger.Error().Err(err).Msg("failed to create file on disk")
		jsonResponse(h, http.StatusInternalServerError, map[string]string{"error": "failed to create file"})
		return
	}

	logger.Info().Str("uuid", fileName).Msg("binary uploaded successfully")

	test := &model.Test{
		UUID:             fileName,
		Name:             header.Filename,
		OriginalFilename: header.Filename,
		Runtime:          runtime,
		Command:          command,
		Severity:         severity,
		ArtifactKey:      dst,
		TimeoutSeconds:   timeout,
	}
	if err := hl.test.Create(r.Context(), test); err != nil {
		jsonResponse(h, http.StatusInternalServerError, map[string]string{
			"error": "binary uploaded failed",
		})
		return
	}
	jsonResponse(h, http.StatusOK, map[string]string{
		"id":      dst,
		"message": "binary uploaded successfully",
	})
}
