package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/FernasFragas/dashboard-app/internal/seed"
)

func TestLoadSeedDocumentFromPlanPath(t *testing.T) {
	doc, err := loadSeedDocument("../../plan/examples/side-plan/side-plan.md")
	if err != nil {
		t.Fatalf("load plan: %v", err)
	}

	if doc.Plan.ID != "side-plan" {
		t.Fatalf("plan id = %q, want side-plan", doc.Plan.ID)
	}
	if len(doc.Projects) != 1 || len(doc.Weeks) != 1 || len(doc.Tasks) != 1 {
		t.Fatalf("doc counts: projects=%d weeks=%d tasks=%d", len(doc.Projects), len(doc.Weeks), len(doc.Tasks))
	}
}

func TestOpenStoreRefusesDifferentPlan(t *testing.T) {
	doc, err := loadSeedDocument("../../plan/examples/side-plan/side-plan.md")
	if err != nil {
		t.Fatalf("load plan: %v", err)
	}

	dataDir := t.TempDir()
	db, err := openStore(dataDir, doc)
	if err != nil {
		t.Fatalf("open first plan: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close first db: %v", err)
	}

	other := doc
	other.Plan.ID = "other-plan"
	other.Plan.Name = "Other Plan"

	db, err = openStore(dataDir, other)
	if db != nil {
		t.Fatalf("unexpected db returned for refused plan")
	}
	if !errors.Is(err, seed.ErrPlanMismatch) {
		t.Fatalf("error = %v, want ErrPlanMismatch", err)
	}
}

func TestHTTPHandlerRoutesAPIBeforeSPA(t *testing.T) {
	apiHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("api"))
	})
	webHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("web"))
	})
	handler := newHTTPHandler(apiHandler, webHandler)

	for _, tc := range []struct {
		path string
		want string
	}{
		{path: "/api/health", want: "api"},
		{path: "/api", want: "api"},
		{path: "/pair/abc", want: "api"},
		{path: "/goals", want: "web"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			handler.ServeHTTP(res, req)

			if body := res.Body.String(); body != tc.want {
				t.Fatalf("body = %q, want %q", body, tc.want)
			}
		})
	}
}

func TestResolvePublicURL(t *testing.T) {
	t.Run("explicit", func(t *testing.T) {
		got, err := resolvePublicURL("https://dash.example.test:8484/", ":8484")
		if err != nil {
			t.Fatalf("resolve public url: %v", err)
		}
		if got != "https://dash.example.test:8484" {
			t.Fatalf("url = %q, want trimmed explicit url", got)
		}
	})

	t.Run("tailnet address", func(t *testing.T) {
		got, err := resolvePublicURL("", "100.64.0.7:8484")
		if err != nil {
			t.Fatalf("resolve public url: %v", err)
		}
		if got != "http://100.64.0.7:8484" {
			t.Fatalf("url = %q, want tailnet url", got)
		}
	})

	t.Run("wildcard gets real host", func(t *testing.T) {
		got, err := resolvePublicURL("", ":8484")
		if err != nil {
			t.Fatalf("resolve public url: %v", err)
		}
		if strings.Contains(got, "://:8484") || !strings.HasPrefix(got, "http://") {
			t.Fatalf("url = %q, want real host with scheme", got)
		}
	})
}
