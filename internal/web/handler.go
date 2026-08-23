// Package web serves the built React application from an embedded filesystem.
package web

import (
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// NewHandler serves static files from fsys and falls back to index.html for SPA routes.
func NewHandler(fsys fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(fsys))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		name := cleanAssetPath(r.URL.Path)
		if !fileExists(fsys, name) {
			name = "index.html"
		}

		clone := r.Clone(r.Context())
		if name == "index.html" {
			clone.URL.Path = "/"
		} else {
			clone.URL.Path = "/" + name
		}
		fileServer.ServeHTTP(w, clone)
	})
}

func cleanAssetPath(urlPath string) string {
	clean := strings.TrimPrefix(path.Clean("/"+urlPath), "/")
	if clean == "" || clean == "." {
		return "index.html"
	}
	return clean
}

func fileExists(fsys fs.FS, name string) bool {
	info, err := fs.Stat(fsys, name)
	return err == nil && !info.IsDir()
}
