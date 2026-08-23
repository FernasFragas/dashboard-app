//go:build embed_frontend

package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed dist
var distFS embed.FS

// Handler serves the embedded production frontend.
func Handler() http.Handler {
	fsys, err := fs.Sub(distFS, "dist")
	if err != nil {
		panic("embedded frontend missing: " + err.Error())
	}
	return NewHandler(fsys)
}

// Embedded reports whether the production frontend assets are compiled into this binary.
func Embedded() bool { return true }
