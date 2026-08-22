package store

import (
	"io/fs"
	"testing/fstest"
)

// mapFS builds an in-memory migration set for the runner's error-path tests.
func mapFS(files map[string]string) fs.FS {
	m := make(fstest.MapFS, len(files))
	for name, body := range files {
		m[name] = &fstest.MapFile{Data: []byte(body)}
	}

	return m
}
