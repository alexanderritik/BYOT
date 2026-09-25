package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/alexanderritik/mini-lambda/runtime"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"
)

type DeleteTestRequest struct {
	Confirm string `json:"confirm"`
}

func (hl *Handler) deleteTest(w http.ResponseWriter, r *http.Request, testID string) {
	if r.Method != http.MethodDelete {
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "DELETE only"})
		return
	}

	var req DeleteTestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	if req.Confirm != "delete" {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": `type "delete" in confirm to proceed`})
		return
	}

	test, err := hl.test.GetByID(r.Context(), testID)
	if err != nil {
		jsonResponse(w, http.StatusNotFound, map[string]string{"error": "test not found"})
		return
	}

	runs, err := hl.testRunRepo.ListByTestID(r.Context(), testID)
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "failed to list runs"})
		return
	}

	active, err := hl.queue.ListActiveForTest(r.Context(), testID)
	if err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "failed to list active jobs"})
		return
	}
	for _, job := range active {
		if job.Status == "running" {
			runtime.Cancel(job.UUID)
		}
	}
	if _, err := hl.queue.CancelActiveForTest(r.Context(), testID, "test deleted"); err != nil {
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "failed to cancel active jobs"})
		return
	}

	if err := hl.test.Delete(r.Context(), testID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			jsonResponse(w, http.StatusNotFound, map[string]string{"error": "test not found"})
			return
		}
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete test"})
		return
	}

	prefix := test.UUID + "/"
	if err := hl.storage.DeletePrefix(prefix); err != nil {
		log.Error().Err(err).Str("test_id", testID).Str("prefix", prefix).Msg("failed to delete minio prefix")
	}

	if test.ArtifactKey != "" && !strings.HasPrefix(test.ArtifactKey, prefix) {
		if err := hl.storage.DeleteObject(test.ArtifactKey); err != nil {
			log.Error().Err(err).Str("object", test.ArtifactKey).Msg("failed to delete artifact object")
		}
	}

	seen := map[string]struct{}{}
	for _, run := range runs {
		if run.LogURL == "" {
			continue
		}
		if strings.HasPrefix(run.LogURL, prefix) {
			continue
		}
		if _, ok := seen[run.LogURL]; ok {
			continue
		}
		seen[run.LogURL] = struct{}{}
		if err := hl.storage.DeleteObject(run.LogURL); err != nil {
			log.Error().Err(err).Str("object", run.LogURL).Msg("failed to delete log object")
		}
	}

	jsonResponse(w, http.StatusOK, map[string]string{
		"message": "test and associated data deleted",
	})
}
