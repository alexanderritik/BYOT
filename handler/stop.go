package handler

import (
	"encoding/json"
	"net/http"

	"github.com/alexanderritik/mini-lambda/runtime"
)

type StopRequest struct {
	TestId string `json:"testId"`
}

func (hl *Handler) Stop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST only"})
		return
	}

	var req StopRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.TestId == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "testId is required"})
		return
	}

	if _, err := hl.test.GetByID(r.Context(), req.TestId); err != nil {
		jsonResponse(w, http.StatusNotFound, map[string]string{"error": "test not found"})
		return
	}

	active, err := hl.queue.ListActiveForTest(r.Context(), req.TestId)
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "failed to list active jobs"})
		return
	}

	var killed int
	for _, job := range active {
		if job.Status == "running" && runtime.Cancel(job.UUID) {
			killed++
		}
	}

	cancelled, err := hl.queue.CancelActiveForTest(r.Context(), req.TestId, "stopped by user")
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "failed to cancel jobs"})
		return
	}

	jsonResponse(w, http.StatusOK, map[string]interface{}{
		"cancelled": cancelled,
		"killed":    killed,
		"message":   "run stopped",
	})
}

func (hl *Handler) SchedulerStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "GET only"})
		return
	}
	if hl.scheduler == nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "scheduler not configured"})
		return
	}
	jsonResponse(w, http.StatusOK, map[string]bool{"paused": hl.scheduler.IsPaused()})
}

func (hl *Handler) SchedulerStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST only"})
		return
	}
	if hl.scheduler == nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "scheduler not configured"})
		return
	}
	hl.scheduler.Pause()
	jsonResponse(w, http.StatusOK, map[string]string{"status": "paused"})
}

func (hl *Handler) SchedulerStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST only"})
		return
	}
	if hl.scheduler == nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "scheduler not configured"})
		return
	}
	hl.scheduler.Resume()
	jsonResponse(w, http.StatusOK, map[string]string{"status": "running"})
}
