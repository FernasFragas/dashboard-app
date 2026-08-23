package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestHandlerServesStaticAssets(t *testing.T) {
	handler := NewHandler(fstest.MapFS{
		"index.html":    {Data: []byte("INDEX")},
		"assets/app.js": {Data: []byte("APP")},
	})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.Code)
	}
	if body := res.Body.String(); body != "APP" {
		t.Fatalf("body = %q, want APP", body)
	}
}

func TestHandlerFallsBackToIndexForSPARoutes(t *testing.T) {
	handler := NewHandler(fstest.MapFS{
		"index.html": {Data: []byte("INDEX")},
	})

	for _, target := range []string{"/", "/goals", "/review/weekly"} {
		t.Run(target, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, target, nil)
			handler.ServeHTTP(res, req)

			if res.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", res.Code)
			}
			if body := res.Body.String(); body != "INDEX" {
				t.Fatalf("body = %q, want INDEX", body)
			}
		})
	}
}

func TestHandlerRejectsWrites(t *testing.T) {
	handler := NewHandler(fstest.MapFS{
		"index.html": {Data: []byte("INDEX")},
	})

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/goals", nil)
	handler.ServeHTTP(res, req)

	if res.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", res.Code)
	}
}
