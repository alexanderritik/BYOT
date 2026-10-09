// Package auth gates write endpoints behind Supabase Auth (GoTrue).
//
// Tokens are verified by asking Supabase itself (GET /auth/v1/user), so it
// works with both legacy HS256 and new asymmetric signing keys, and honours
// revoked sessions. Positive results are cached briefly.
package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"
)

const cacheTTL = 60 * time.Second

type Verifier struct {
	url            string
	publishableKey string
	client         *http.Client

	mu    sync.Mutex
	cache map[string]time.Time // token -> expiry of cached approval
}

// New returns nil when Supabase auth is not configured (auth disabled).
func New(supabaseURL, publishableKey string) *Verifier {
	supabaseURL = strings.TrimRight(supabaseURL, "/")
	if supabaseURL == "" || publishableKey == "" {
		return nil
	}
	return &Verifier{
		url:            supabaseURL,
		publishableKey: publishableKey,
		client:         &http.Client{Timeout: 5 * time.Second},
		cache:          map[string]time.Time{},
	}
}

func (v *Verifier) Enabled() bool { return v != nil }

// Config is what the browser needs to talk to Supabase Auth. The publishable key is
// public by design.
func (v *Verifier) PublicConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if v == nil {
		json.NewEncoder(w).Encode(map[string]any{"enabled": false})
		return
	}
	json.NewEncoder(w).Encode(map[string]any{
		"enabled":         true,
		"url":             v.url,
		"publishable_key": v.publishableKey,
	})
}

// Require wraps a handler, returning 401 unless the request carries a valid
// Supabase access token. A nil Verifier lets everything through.
func (v *Verifier) Require(next http.HandlerFunc) http.HandlerFunc {
	if v == nil {
		return next
	}
	return func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token == "" || token == r.Header.Get("Authorization") || !v.valid(r.Context(), token) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]string{"error": "authentication required"})
			return
		}
		next(w, r)
	}
}

func (v *Verifier) valid(ctx context.Context, token string) bool {
	now := time.Now()
	v.mu.Lock()
	if exp, ok := v.cache[token]; ok && now.Before(exp) {
		v.mu.Unlock()
		return true
	}
	v.mu.Unlock()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.url+"/auth/v1/user", nil)
	if err != nil {
		return false
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("apikey", v.publishableKey)
	resp, err := v.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}

	v.mu.Lock()
	for t, exp := range v.cache { // opportunistic cleanup
		if now.After(exp) {
			delete(v.cache, t)
		}
	}
	v.cache[token] = now.Add(cacheTTL)
	v.mu.Unlock()
	return true
}
