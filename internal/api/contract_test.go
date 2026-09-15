package api

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// Contract tests keep docs/API.md honest. The endpoint table is what agents and the frontend read
// first; a route that exists only in code, or only in the doc, sends them the wrong way.

var (
	registeredRouteRe = regexp.MustCompile(`mux\.HandleFunc\("([A-Z]+) (/[^"]*)"`)
	documentedRouteRe = regexp.MustCompile("^\\|\\s*`([A-Z]+) (/[^`]*)`")
)

func TestEveryRouteIsDocumented(t *testing.T) {
	documented := documentedRoutes(t)
	registered := map[string]bool{}

	for _, route := range registeredRoutes(t) {
		registered[route] = true
		if !documented[route] {
			t.Errorf("%s is registered in internal/api/server.go but missing from the endpoint "+
				"table in docs/API.md §2; add a row there and a section in §3", route)
		}
	}

	var stale []string
	for route := range documented {
		if !registered[route] {
			stale = append(stale, route)
		}
	}
	sort.Strings(stale)
	for _, route := range stale {
		t.Errorf("%s is listed in docs/API.md §2 but NewMux does not register it; remove the row "+
			"or add the route", route)
	}
}

func TestGetEndpointsRejectUnknownQueryParameters(t *testing.T) {
	srv := newTestAPI(t, "")
	samples := strings.NewReplacer("{id}", "1", "{week}", "W16", "{code}", "unknown.svg")

	for _, route := range registeredRoutes(t) {
		method, path, _ := strings.Cut(route, " ")
		if method != http.MethodGet || !strings.HasPrefix(path, "/api/") {
			continue
		}

		target := samples.Replace(path)
		if strings.Contains(target, "{") {
			t.Fatalf("%s has a path parameter with no sample value; add one to the replacer in "+
				"this test", route)
		}
		target += "?definitely_unknown=1"

		t.Run(route, func(t *testing.T) {
			rec := srv.request(method, target, nil)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("GET %s = %d, want 400. Unknown query parameters must be rejected "+
					"(docs/API.md §1): call requireKnownQuery(r, <allowed names>) at the top of the "+
					"handler. body=%s", target, rec.Code, rec.Body.String())
			}
		})
	}
}

// registeredRoutes reads the route patterns straight from NewMux's source, so the test cannot
// drift from the mux it guards.
func registeredRoutes(t *testing.T) []string {
	t.Helper()

	body, err := os.ReadFile("server.go")
	if err != nil {
		t.Fatalf("read server.go: %v", err)
	}

	matches := registeredRouteRe.FindAllStringSubmatch(string(body), -1)
	if len(matches) == 0 {
		t.Fatal("no mux.HandleFunc routes found in server.go; update registeredRouteRe if NewMux " +
			"changed how it registers routes")
	}

	routes := make([]string, 0, len(matches))
	for _, m := range matches {
		routes = append(routes, m[1]+" "+m[2])
	}
	return routes
}

func documentedRoutes(t *testing.T) map[string]bool {
	t.Helper()

	body, err := os.ReadFile(filepath.Join("..", "..", "docs", "API.md"))
	if err != nil {
		t.Fatalf("read docs/API.md: %v", err)
	}

	routes := map[string]bool{}
	for _, line := range strings.Split(string(body), "\n") {
		m := documentedRouteRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		// The doc spells the QR route with its required extension; the mux captures it in {code}.
		routes[m[1]+" "+strings.Replace(m[2], "}.svg", "}", 1)] = true
	}

	if len(routes) == 0 {
		t.Fatal("no endpoint rows found in docs/API.md; rows must look like | `GET /api/path` | ... |")
	}
	return routes
}
