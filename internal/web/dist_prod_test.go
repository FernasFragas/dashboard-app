//go:build embed_frontend

package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEmbeddedHandlerServesIndexForDeepLink(t *testing.T) {
	if !Embedded() {
		t.Fatalf("Embedded = false, want true")
	}

	res := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/goals", nil)
	Handler().ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", res.Code)
	}
	if body := res.Body.String(); !strings.Contains(body, `<div id="root"></div>`) {
		t.Fatalf("embedded index body does not contain app root")
	}
}
