package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/alexanderritik/byot/model"
	"github.com/alexanderritik/byot/repository"
	"github.com/alexanderritik/byot/secrets"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog/log"
)

type envVarRequest struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	IsSecret bool   `json:"is_secret"`
}

// envRoutes handles /tests/{id}/env and /tests/{id}/env/{name}. rest is the
// path after "{id}/env". Always behind authentication.
func (hl *Handler) envRoutes(w http.ResponseWriter, r *http.Request, testID, rest string) {
	if _, err := hl.test.GetByID(r.Context(), testID); err != nil {
		jsonResponse(w, http.StatusNotFound, map[string]string{"error": "test not found"})
		return
	}
	name := strings.Trim(rest, "/")

	switch {
	case name == "" && r.Method == http.MethodGet:
		hl.listEnv(w, r, testID)
	case name == "" && (r.Method == http.MethodPut || r.Method == http.MethodPost):
		hl.putEnv(w, r, testID)
	case name != "" && r.Method == http.MethodDelete:
		hl.deleteEnv(w, r, testID, name)
	default:
		jsonResponse(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (hl *Handler) listEnv(w http.ResponseWriter, r *http.Request, testID string) {
	vars, err := hl.env.List(r.Context(), testID)
	if err != nil {
		log.Error().Err(err).Msg("list env vars")
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "failed to list environment"})
		return
	}
	for i := range vars {
		if vars[i].IsSecret {
			vars[i].Value = "" // write-only: ciphertext is never returned
		}
	}
	jsonResponse(w, http.StatusOK, map[string]any{
		"vars":            vars,
		"secrets_enabled": hl.secrets.Enabled(),
	})
}

func (hl *Handler) putEnv(w http.ResponseWriter, r *http.Request, testID string) {
	var req envVarRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024)).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if err := secrets.ValidateName(req.Name); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := secrets.ValidateValue(req.Value); err != nil {
		jsonResponse(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	v := model.EnvVar{Name: req.Name, Value: req.Value, IsSecret: req.IsSecret}
	if req.IsSecret {
		if !hl.secrets.Enabled() {
			jsonResponse(w, http.StatusServiceUnavailable, map[string]string{"error": "secrets are disabled: SECRETS_KEY is not configured on the server"})
			return
		}
		enc, err := hl.secrets.Encrypt(req.Value)
		if err != nil {
			log.Error().Err(err).Msg("encrypt secret")
			jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "failed to encrypt secret"})
			return
		}
		v.Value = enc
	}

	if err := hl.env.Upsert(r.Context(), testID, v, secrets.MaxVars); err != nil {
		if errors.Is(err, repository.ErrTooManyEnvVars) {
			jsonResponse(w, http.StatusBadRequest, map[string]string{"error": "too many environment variables (max 50)"})
			return
		}
		log.Error().Err(err).Msg("save env var")
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "failed to save variable"})
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{"name": req.Name, "is_secret": req.IsSecret})
}

func (hl *Handler) deleteEnv(w http.ResponseWriter, r *http.Request, testID, name string) {
	if err := hl.env.Delete(r.Context(), testID, name); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			jsonResponse(w, http.StatusNotFound, map[string]string{"error": "variable not found"})
			return
		}
		log.Error().Err(err).Msg("delete env var")
		jsonResponse(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete variable"})
		return
	}
	jsonResponse(w, http.StatusOK, map[string]string{"message": "deleted"})
}
