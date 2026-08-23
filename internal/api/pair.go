package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	qrcode "github.com/skip2/go-qrcode"

	"github.com/FernasFragas/dashboard-app/internal/store"
)

const (
	pairTTL          = 90 * time.Second
	pairCodeBytes    = 16
	pairFailLimit    = 10
	pairFailWindow   = time.Minute
	pairBlockedFor   = time.Minute
	pairInvalidTitle = "Pairing code expired"
)

type createPairRequest struct {
	Route string `json:"route"`
}

type createPairResponse struct {
	URL              string `json:"url"`
	SVGURL           string `json:"svg_url"`
	ExpiresAt        string `json:"expires_at"`
	ExpiresInSeconds int    `json:"expires_in_seconds"`
}

func (s *Server) createPair(w http.ResponseWriter, r *http.Request) {
	var req createPairRequest
	if err := decodeJSON(r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	route, err := normalizePairRoute(req.Route)
	if err != nil {
		s.writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if _, err := s.store.SweepExpiredPairingCodes(r.Context(), s.now()); err != nil {
		s.handleStoreError(w, err)
		return
	}

	code, err := generatePairCode()
	if err != nil {
		s.logger.Error("generate pairing code", "error", err)
		s.writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	expiresAt := s.now().Add(pairTTL)
	if err := s.store.SavePairingCode(r.Context(), hashPairCode(code), route, expiresAt); err != nil {
		s.handleStoreError(w, err)
		return
	}

	s.writeJSON(w, http.StatusCreated, createPairResponse{
		URL:              s.absoluteURL("/pair/" + code),
		SVGURL:           "/api/pair/" + code + ".svg",
		ExpiresAt:        expiresAt.UTC().Format(time.RFC3339),
		ExpiresInSeconds: int(pairTTL.Seconds()),
	})
}

func (s *Server) pairSVG(w http.ResponseWriter, r *http.Request) {
	raw := r.PathValue("code")
	if !strings.HasSuffix(raw, ".svg") {
		http.NotFound(w, r)
		return
	}

	code := strings.TrimSuffix(raw, ".svg")
	svg, err := qrSVG(s.absoluteURL("/pair/" + code))
	if err != nil {
		s.logger.Error("render pair qr", "error", err)
		s.writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(svg)); err != nil {
		s.logger.Error("write pair qr", "error", err)
	}
}

func (s *Server) redeemPair(w http.ResponseWriter, r *http.Request) {
	now := s.now()
	if !s.pairLimiter.allow(now) {
		s.logger.Warn("pair redemption rate limited")
		s.writeInvalidPair(w)
		return
	}

	route, err := s.store.RedeemPairingCode(r.Context(), hashPairCode(r.PathValue("code")), now)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			if s.pairLimiter.recordFailure(now) {
				s.logger.Warn("pair redemption rate limited")
			}
			s.writeInvalidPair(w)
			return
		}
		s.handleStoreError(w, err)
		return
	}

	s.pairLimiter.recordSuccess()
	s.writePairBootstrap(w, route)
}

func (s *Server) writePairBootstrap(w http.ResponseWriter, route string) {
	tokenJSON, err := json.Marshal(s.token)
	if err != nil {
		s.logger.Error("encode pair token", "error", err)
		s.writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	routeJSON, err := json.Marshal(route)
	if err != nil {
		s.logger.Error("encode pair route", "error", err)
		s.writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	body := fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Pairing dashboard</title>
</head>
<body>
<script>
(() => {
  const token = %s;
  const route = %s;
  if (token !== "") {
    window.localStorage.setItem("dashboard.token", token);
  }
  window.location.replace(route);
})();
</script>
</body>
</html>`, tokenJSON, routeJSON)
	if _, err := w.Write([]byte(body)); err != nil {
		s.logger.Error("write pair bootstrap", "error", err)
	}
}

func (s *Server) writeInvalidPair(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	if _, err := w.Write([]byte(invalidPairHTML)); err != nil {
		s.logger.Error("write invalid pair response", "error", err)
	}
}

func (s *Server) absoluteURL(path string) string {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return s.publicURL + path
}

func normalizePairRoute(route string) (string, error) {
	route = strings.TrimSpace(route)
	if route == "" {
		return "/", nil
	}
	if len(route) > 2048 {
		return "", errors.New("route is too long")
	}
	if !strings.HasPrefix(route, "/") || strings.HasPrefix(route, "//") {
		return "", errors.New("route must be an absolute app path")
	}

	u, err := url.Parse(route)
	if err != nil {
		return "", fmt.Errorf("route is invalid: %w", err)
	}
	if u.IsAbs() || u.Host != "" {
		return "", errors.New("route must stay inside this app")
	}
	return route, nil
}

func generatePairCode() (string, error) {
	var raw [pairCodeBytes]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}

	code := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw[:])
	return strings.ToLower(code), nil
}

func hashPairCode(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

func qrSVG(content string) (string, error) {
	qr, err := qrcode.New(content, qrcode.Medium)
	if err != nil {
		return "", err
	}

	bitmap := qr.Bitmap()
	size := len(bitmap)
	var b strings.Builder
	b.Grow(size * size / 2)
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	b.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" role="img" aria-label="Pair phone QR">`, size, size))
	b.WriteString(`<rect width="100%" height="100%" fill="#ffffff"/>`)
	b.WriteString(`<path fill="#09090b" d="`)
	for y, row := range bitmap {
		for x, dark := range row {
			if dark {
				b.WriteString(fmt.Sprintf("M%d %dh1v1h-1z", x, y))
			}
		}
	}
	b.WriteString(`"/></svg>`)
	return b.String(), nil
}

const invalidPairHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>` + pairInvalidTitle + `</title>
</head>
<body>` + pairInvalidTitle + `</body>
</html>`

type pairRateLimiter struct {
	mu           sync.Mutex
	failures     []time.Time
	blockedUntil time.Time
}

func newPairRateLimiter() *pairRateLimiter {
	return &pairRateLimiter{failures: make([]time.Time, 0, pairFailLimit)}
}

func (l *pairRateLimiter) allow(now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	return !l.blockedUntil.After(now)
}

func (l *pairRateLimiter) recordFailure(now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := now.Add(-pairFailWindow)
	kept := l.failures[:0]
	for _, failure := range l.failures {
		if failure.After(cutoff) {
			kept = append(kept, failure)
		}
	}
	l.failures = append(kept, now)

	if len(l.failures) >= pairFailLimit {
		l.blockedUntil = now.Add(pairBlockedFor)
		l.failures = l.failures[:0]
		return true
	}
	return false
}

func (l *pairRateLimiter) recordSuccess() {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.failures = l.failures[:0]
}
