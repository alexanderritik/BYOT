package web

import (
	"net/http"
)

// Register mounts marketing and control-room HTML on the same mux as the API.
func Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		serveHTML(w, r, "index.html")
	})
	mux.HandleFunc("GET /app", func(w http.ResponseWriter, r *http.Request) {
		serveHTML(w, r, "product.html")
	})
	mux.HandleFunc("GET /app/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/app", http.StatusMovedPermanently)
	})
}

func serveHTML(w http.ResponseWriter, r *http.Request, name string) {
	b, err := files.ReadFile(name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(b)
}
