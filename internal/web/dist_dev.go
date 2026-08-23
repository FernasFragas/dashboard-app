//go:build !embed_frontend

package web

import "net/http"

// Handler returns a clear 404 in development builds. Vite serves the frontend during make dev.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		http.Error(w, "frontend is not embedded; run make dev or build with make build", http.StatusNotFound)
	})
}

// Embedded reports whether the production frontend assets are compiled into this binary.
func Embedded() bool { return false }
