package handler

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/alexanderritik/mini-lambda/model"
	"github.com/alexanderritik/mini-lambda/queue"
	"github.com/alexanderritik/mini-lambda/repository"
	"github.com/alexanderritik/mini-lambda/schedule"
	"github.com/alexanderritik/mini-lambda/scheduler"
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
	scheduler   *scheduler.Scheduler
}

func NewHandler(storage storage.Storage, test repository.TestRepository, testRun repository.TestRunRepository, q *queue.Queue, sched *scheduler.Scheduler) *Handler {
	return &Handler{
		storage:     storage,
		test:        test,
		testRunRepo: testRun,
		queue:       q,
		scheduler:   sched,
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
		"trigger":     job.Trigger,
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

// Tests routes GET /tests, GET /tests/{id}, GET /tests/{id}/runs, PATCH /tests/{id}/config.
func (hl *Handler) Tests(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/tests" || r.URL.Path == "/tests/" {
		if r.Method != http.MethodGet {
			jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "GET only"})
			return
		}
		hl.listTests(w, r)
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/tests/")
	path = strings.Trim(path, "/")
	if path == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "test ID is missing"})
		return
	}

	if strings.HasSuffix(path, "/schedule/start") {
		testID := strings.TrimSuffix(path, "/schedule/start")
		testID = strings.TrimSuffix(testID, "/")
		if testID == "" {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "test ID is missing"})
			return
		}
		hl.enableSchedule(w, r, testID)
		return
	}

	if strings.HasSuffix(path, "/schedule/stop") {
		testID := strings.TrimSuffix(path, "/schedule/stop")
		testID = strings.TrimSuffix(testID, "/")
		if testID == "" {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "test ID is missing"})
			return
		}
		hl.disableSchedule(w, r, testID)
		return
	}

	if strings.HasSuffix(path, "/runs") {
		testID := strings.TrimSuffix(path, "/runs")
		testID = strings.TrimSuffix(testID, "/")
		if testID == "" {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "test ID is missing"})
			return
		}
		if r.Method != http.MethodGet {
			jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "GET only"})
			return
		}
		hl.listTestRuns(w, r, testID)
		return
	}

	if strings.HasSuffix(path, "/config") {
		testID := strings.TrimSuffix(path, "/config")
		testID = strings.TrimSuffix(testID, "/")
		if testID == "" {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "test ID is missing"})
			return
		}
		hl.updateTestConfig(w, r, testID)
		return
	}

	if r.Method == http.MethodDelete {
		hl.deleteTest(w, r, path)
		return
	}

	if r.Method != http.MethodGet {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "GET, PATCH /tests/{id}/config, or DELETE /tests/{id}"})
		return
	}

	test, err := hl.test.GetByID(r.Context(), path)
	if err != nil {
		jsonResponse(w, http.StatusNotFound, map[string]string{"error": "test not found"})
		return
	}
	jsonResponse(w, http.StatusOK, test)
}

func (hl *Handler) listTests(w http.ResponseWriter, r *http.Request) {
	items, err := hl.test.ListWithLastRun(r.Context())
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "failed to list tests"})
		return
	}
	if items == nil {
		items = []model.TestListItem{}
	}
	jsonResponse(w, http.StatusOK, items)
}

func (hl *Handler) listTestRuns(w http.ResponseWriter, r *http.Request, testID string) {
	if _, err := hl.test.GetByID(r.Context(), testID); err != nil {
		jsonResponse(w, http.StatusNotFound, map[string]string{"error": "test not found"})
		return
	}
	runs, err := hl.testRunRepo.ListByTestID(r.Context(), testID)
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "failed to list runs"})
		return
	}
	if runs == nil {
		runs = []*model.TestRun{}
	}
	jsonResponse(w, http.StatusOK, runs)
}

// RunRoutes handles GET /runs/{id}/log.
func (hl *Handler) RunRoutes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "GET only"})
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/runs/")
	path = strings.Trim(path, "/")
	if !strings.HasSuffix(path, "/log") {
		jsonResponse(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	runID := strings.TrimSuffix(path, "/log")
	runID = strings.TrimSuffix(runID, "/")
	if runID == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "run ID is missing"})
		return
	}

	run, err := hl.testRunRepo.GetByID(r.Context(), runID)
	if err != nil {
		jsonResponse(w, http.StatusNotFound, map[string]string{"error": "run not found"})
		return
	}
	if run.LogURL == "" {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		return
	}

	reader, err := hl.storage.DownloadBlob(run.LogURL)
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "failed to load log"})
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	io.Copy(w, reader)
}

type UpdateTestConfigRequest struct {
	Command         *string `json:"command"`
	Severity        *string `json:"severity"`
	TimeoutSeconds  *int    `json:"timeout_seconds"`
	Cron               *string `json:"cron"`
	ScheduleEnabled    *bool   `json:"schedule_enabled"`
	WebhookURL         *string `json:"webhook_url"`
	FailureThreshold   *int    `json:"failure_threshold"`
	AlertsEnabled      *bool   `json:"alerts_enabled"`
}

func (hl *Handler) updateTestConfig(w http.ResponseWriter, r *http.Request, testID string) {
	if r.Method != http.MethodPatch && r.Method != http.MethodPut {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "PATCH or PUT only"})
		return
	}

	var req UpdateTestConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}

	test, err := hl.test.GetByID(r.Context(), testID)
	if err != nil {
		jsonResponse(w, http.StatusNotFound, map[string]string{"error": "test not found"})
		return
	}

	if req.Command != nil {
		test.Command = *req.Command
	}
	if req.Severity != nil {
		test.Severity = *req.Severity
	}
	if req.TimeoutSeconds != nil {
		if *req.TimeoutSeconds <= 0 {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "timeout_seconds must be positive"})
			return
		}
		test.TimeoutSeconds = *req.TimeoutSeconds
	}
	if req.Cron != nil {
		test.ScheduleCron = *req.Cron
	}
	if req.ScheduleEnabled != nil {
		test.ScheduleEnabled = *req.ScheduleEnabled
	}
	if req.WebhookURL != nil {
		test.WebhookURL = strings.TrimSpace(*req.WebhookURL)
	}
	if req.FailureThreshold != nil {
		if *req.FailureThreshold <= 0 {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "failure_threshold must be positive"})
			return
		}
		test.FailureThreshold = *req.FailureThreshold
	}
	if req.AlertsEnabled != nil {
		test.AlertsEnabled = *req.AlertsEnabled
	}
	if test.FailureThreshold <= 0 {
		test.FailureThreshold = 3
	}

	var nextRun *time.Time
	if test.ScheduleEnabled {
		if test.ScheduleCron == "" {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "cron is required when schedule is enabled"})
			return
		}
		next, err := schedule.NextRun(test.ScheduleCron, time.Now().UTC())
		if err != nil {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "invalid cron expression"})
			return
		}
		nextRun = &next
	} else {
		test.ScheduleCron = ""
		nextRun = nil
	}

	cfg := repository.TestConfigUpdate{
		Command:          test.Command,
		Severity:         test.Severity,
		TimeoutSeconds:   test.TimeoutSeconds,
		ScheduleCron:     test.ScheduleCron,
		ScheduleEnabled:  test.ScheduleEnabled,
		NextRunAt:        nextRun,
		WebhookURL:       test.WebhookURL,
		FailureThreshold: test.FailureThreshold,
		AlertsEnabled:    test.AlertsEnabled,
	}
	if err := hl.test.UpdateConfig(r.Context(), testID, cfg); err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "failed to update test config"})
		return
	}

	updated, err := hl.test.GetByID(r.Context(), testID)
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "failed to load updated test"})
		return
	}
	jsonResponse(w, http.StatusOK, updated)
}

func (hl *Handler) Run(h http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonResponse(h, http.StatusMethodNotAllowed, map[string]string{"error": "POST only"})
		return
	}

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
	job, err := hl.queue.Enqueue(r.Context(), testRes.UUID, queue.TriggerManual)
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

	if r.Method != http.MethodPost {
		jsonResponse(h, http.StatusMethodNotAllowed, map[string]string{"error": "POST only accepted"})
		return
	}
	if err := r.ParseMultipartForm(maxFileSize); err != nil {
		jsonResponse(h, http.StatusBadRequest, map[string]string{"error": "invalid multipart form"})
		return
	}
	runtime := r.FormValue("runtime")
	severity := r.FormValue("severity")
	command := r.FormValue("command")
	cron := strings.TrimSpace(r.FormValue("cron"))
	webhookURL := strings.TrimSpace(r.FormValue("webhook_url"))
	failureThreshold := 3
	if ft := strings.TrimSpace(r.FormValue("failure_threshold")); ft != "" {
		v, err := strconv.Atoi(ft)
		if err != nil || v <= 0 {
			jsonResponse(h, http.StatusBadRequest, map[string]string{"error": "failure_threshold must be a positive integer"})
			return
		}
		failureThreshold = v
	}
	displayName := strings.TrimSpace(r.FormValue("name"))
	timeoutStr := r.FormValue("timeout")
	timeout, err := strconv.Atoi(timeoutStr)
	if err != nil || timeout == 0 {
		timeout = 30 // default
	}
	if runtime == "" || severity == "" {
		jsonResponse(h, http.StatusBadRequest, map[string]string{"error": "runtime and severity are required"})
		return
	}

	file, header, err := r.FormFile("binary")
	if err != nil {
		logger.Error().Err(err).Msg("failed to read uploaded file")
		jsonResponse(h, http.StatusBadRequest, map[string]string{"error": "binary file is required"})
		return
	}
	defer file.Close()

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

	testName := displayName
	if testName == "" {
		testName = header.Filename
	}

	test := &model.Test{
		UUID:             fileName,
		Name:             testName,
		OriginalFilename: header.Filename,
		Runtime:          runtime,
		Command:          command,
		Severity:         severity,
		ArtifactKey:      dst,
		TimeoutSeconds:   timeout,
		WebhookURL:       webhookURL,
		FailureThreshold: failureThreshold,
		AlertsEnabled:    true,
	}
	if cron != "" {
		next, err := schedule.NextRun(cron, time.Now().UTC())
		if err != nil {
			jsonResponse(h, http.StatusBadRequest, map[string]string{"error": "invalid cron expression"})
			return
		}
		test.ScheduleCron = cron
		test.ScheduleEnabled = true
		test.NextRunAt = &next
	}
	if err := hl.test.Create(r.Context(), test); err != nil {
		jsonResponse(h, http.StatusInternalServerError, map[string]string{
			"error": "binary uploaded failed",
		})
		return
	}
	jsonResponse(h, http.StatusOK, map[string]string{
		"id":      fileName,
		"message": "binary uploaded successfully",
	})
}
