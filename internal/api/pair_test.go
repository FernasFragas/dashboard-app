package api

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestPairCodeRedeemsOnceAndBootstrapsTokenAndRoute(t *testing.T) {
	srv := newTestAPI(t, "secret")

	rec := srv.request(http.MethodPost, "/api/pair", map[string]string{
		"route": "/goals?project=synapse",
	})
	assertStatus(t, rec, http.StatusCreated)

	var body createPairResponse
	decodeBody(t, rec, &body)
	code := pairCodeFromURL(t, body.URL)

	var storedHash string
	if err := srv.store.DB().QueryRowContext(context.Background(),
		`SELECT code_hash FROM pairing_codes`).Scan(&storedHash); err != nil {
		t.Fatalf("read stored pairing hash: %v", err)
	}
	if storedHash == code || strings.Contains(storedHash, code) {
		t.Fatalf("stored hash %q contains plaintext code %q", storedHash, code)
	}

	redeem := srv.requestNoToken(http.MethodGet, "/pair/"+code, nil)
	assertStatus(t, redeem, http.StatusOK)
	if html := redeem.Body.String(); !strings.Contains(html, `localStorage.setItem("dashboard.token", token)`) ||
		!strings.Contains(html, `"secret"`) ||
		!strings.Contains(html, `"/goals?project=synapse"`) {
		t.Fatalf("bootstrap html did not include token handoff and route: %s", html)
	}

	second := srv.requestNoToken(http.MethodGet, "/pair/"+code, nil)
	assertStatus(t, second, http.StatusNotFound)
}

func TestPairExpiredUnknownAndUsedResponsesMatch(t *testing.T) {
	srv := newTestAPI(t, "secret")

	if err := srv.store.SavePairingCode(context.Background(),
		hashPairCode("expired"), "/goals", apiNow.Add(-time.Second)); err != nil {
		t.Fatalf("save expired pair code: %v", err)
	}

	expired := srv.requestNoToken(http.MethodGet, "/pair/expired", nil)
	unknown := srv.requestNoToken(http.MethodGet, "/pair/unknown", nil)
	assertStatus(t, expired, http.StatusNotFound)
	assertStatus(t, unknown, http.StatusNotFound)

	if expired.Body.String() != unknown.Body.String() {
		t.Fatalf("expired and unknown responses differ:\nexpired=%q\nunknown=%q",
			expired.Body.String(), unknown.Body.String())
	}
}

func TestPairEndpointsAuthAndSVG(t *testing.T) {
	srv := newTestAPI(t, "secret")

	assertStatus(t, srv.requestNoToken(http.MethodGet, "/api/dashboard", nil), http.StatusUnauthorized)

	rec := srv.request(http.MethodPost, "/api/pair", map[string]string{"route": "/review"})
	assertStatus(t, rec, http.StatusCreated)

	var body createPairResponse
	decodeBody(t, rec, &body)
	svg := srv.request(http.MethodGet, body.SVGURL, nil)
	assertStatus(t, svg, http.StatusOK)

	if contentType := svg.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "image/svg+xml") {
		t.Fatalf("svg content type = %q, want image/svg+xml", contentType)
	}
	if !strings.Contains(svg.Body.String(), `<svg`) || !strings.Contains(svg.Body.String(), `<path`) {
		t.Fatalf("svg body does not look like qr svg: %s", svg.Body.String())
	}
}

func TestPairRequestLogRedactsCodes(t *testing.T) {
	var logs bytes.Buffer
	srv := newSeededTestAPIWithConfig(t, apiTestConfig{
		token:     "secret",
		publicURL: "http://dash.test:8484",
		logger: slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		})),
	})

	code := "sensitivepaircode"
	rec := srv.requestNoToken(http.MethodGet, "/pair/"+code, nil)
	assertStatus(t, rec, http.StatusNotFound)

	if strings.Contains(logs.String(), code) {
		t.Fatalf("request log leaked code: %s", logs.String())
	}
	if !strings.Contains(logs.String(), "/pair/<redacted>") {
		t.Fatalf("request log did not contain redacted path: %s", logs.String())
	}
}

func TestPairRateLimitTripsAndRecovers(t *testing.T) {
	now := apiNow
	srv := newSeededTestAPIWithConfig(t, apiTestConfig{
		token:     "secret",
		publicURL: "http://dash.test:8484",
		now:       func() time.Time { return now },
	})

	rec := srv.request(http.MethodPost, "/api/pair", map[string]string{"route": "/skills"})
	assertStatus(t, rec, http.StatusCreated)
	var body createPairResponse
	decodeBody(t, rec, &body)
	code := pairCodeFromURL(t, body.URL)

	for i := 0; i < pairFailLimit; i++ {
		assertStatus(t, srv.requestNoToken(http.MethodGet, "/pair/bad-code", nil), http.StatusNotFound)
	}

	blocked := srv.requestNoToken(http.MethodGet, "/pair/"+code, nil)
	assertStatus(t, blocked, http.StatusNotFound)

	now = now.Add(pairBlockedFor + time.Second)
	recovered := srv.requestNoToken(http.MethodGet, "/pair/"+code, nil)
	assertStatus(t, recovered, http.StatusOK)
}

func TestCreatePairSweepsExpiredRows(t *testing.T) {
	srv := newTestAPI(t, "secret")
	if err := srv.store.SavePairingCode(context.Background(),
		hashPairCode("expired"), "/goals", apiNow.Add(-time.Second)); err != nil {
		t.Fatalf("save expired pair code: %v", err)
	}

	rec := srv.request(http.MethodPost, "/api/pair", map[string]string{"route": "/"})
	assertStatus(t, rec, http.StatusCreated)

	var count int
	if err := srv.store.DB().QueryRowContext(context.Background(),
		`SELECT count(*) FROM pairing_codes WHERE code_hash = ?`,
		hashPairCode("expired"),
	).Scan(&count); err != nil {
		t.Fatalf("count expired rows: %v", err)
	}
	if count != 0 {
		t.Fatalf("expired pairing rows = %d, want swept", count)
	}
}

func TestNormalizePairRoute(t *testing.T) {
	for _, route := range []string{"/", "/goals?project=synapse", "/review#daily"} {
		if _, err := normalizePairRoute(route); err != nil {
			t.Fatalf("normalize %q: %v", route, err)
		}
	}

	for _, route := range []string{"https://dash.test/goals", "//dash.test/goals"} {
		if _, err := normalizePairRoute(route); err == nil {
			t.Fatalf("normalize %q succeeded, want error", route)
		}
	}
}

func pairCodeFromURL(t *testing.T, raw string) string {
	t.Helper()

	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse pair url: %v", err)
	}
	code, ok := strings.CutPrefix(u.Path, "/pair/")
	if !ok || code == "" {
		t.Fatalf("pair url path = %q, want /pair/<code>", u.Path)
	}
	return code
}
