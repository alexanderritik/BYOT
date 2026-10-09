package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRequire(t *testing.T) {
	sb := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer good" || r.Header.Get("apikey") != "pub" {
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))
	defer sb.Close()

	h := New(sb.URL, "pub").Require(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })

	for _, tc := range []struct {
		header string
		want   int
	}{{"", 401}, {"Bearer bad", 401}, {"good", 401}, {"Bearer good", 200}} {
		req := httptest.NewRequest("POST", "/", nil)
		if tc.header != "" {
			req.Header.Set("Authorization", tc.header)
		}
		rec := httptest.NewRecorder()
		h(rec, req)
		if rec.Code != tc.want {
			t.Errorf("header %q: got %d want %d", tc.header, rec.Code, tc.want)
		}
	}
}

func TestDisabledPassesThrough(t *testing.T) {
	var v *Verifier
	rec := httptest.NewRecorder()
	v.Require(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })(rec, httptest.NewRequest("POST", "/", nil))
	if rec.Code != 204 {
		t.Fatalf("got %d", rec.Code)
	}
}
