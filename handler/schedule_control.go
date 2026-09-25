package handler

import (
	"net/http"
	"time"

	"github.com/alexanderritik/mini-lambda/repository"
	"github.com/alexanderritik/mini-lambda/schedule"
)

func (hl *Handler) enableSchedule(w http.ResponseWriter, r *http.Request, testID string) {
	if r.Method != http.MethodPost {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST only"})
		return
	}

	test, err := hl.test.GetByID(r.Context(), testID)
	if err != nil {
		jsonResponse(w, http.StatusNotFound, map[string]string{"error": "test not found"})
		return
	}
	if test.ScheduleCron == "" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{
			"error": "cron is required before enabling schedule — set cron on upload or PATCH /tests/{id}/config",
		})
		return
	}

	next, err := schedule.NextRun(test.ScheduleCron, time.Now().UTC())
	if err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "invalid cron expression"})
		return
	}

	cfg := repository.TestConfigUpdate{
		Command:         test.Command,
		Severity:        test.Severity,
		TimeoutSeconds:  test.TimeoutSeconds,
		ScheduleCron:    test.ScheduleCron,
		ScheduleEnabled: true,
		NextRunAt:       &next,
	}
	if err := hl.test.UpdateConfig(r.Context(), testID, cfg); err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "failed to enable schedule"})
		return
	}

	updated, err := hl.test.GetByID(r.Context(), testID)
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "failed to load test"})
		return
	}
	jsonResponse(w, http.StatusOK, updated)
}

func (hl *Handler) disableSchedule(w http.ResponseWriter, r *http.Request, testID string) {
	if r.Method != http.MethodPost {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "POST only"})
		return
	}

	test, err := hl.test.GetByID(r.Context(), testID)
	if err != nil {
		jsonResponse(w, http.StatusNotFound, map[string]string{"error": "test not found"})
		return
	}

	cfg := repository.TestConfigUpdate{
		Command:         test.Command,
		Severity:        test.Severity,
		TimeoutSeconds:  test.TimeoutSeconds,
		ScheduleCron:    test.ScheduleCron,
		ScheduleEnabled: false,
		NextRunAt:       nil,
	}
	if err := hl.test.UpdateConfig(r.Context(), testID, cfg); err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "failed to disable schedule"})
		return
	}

	updated, err := hl.test.GetByID(r.Context(), testID)
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "failed to load test"})
		return
	}
	jsonResponse(w, http.StatusOK, updated)
}
