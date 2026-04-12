package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/config"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/state"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/topology"
)

func TestHealthEndpoint(t *testing.T) {
	h := New(&config.Config{}, state.New(), &topology.Topology{})
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var body struct {
		OK        bool   `json:"ok"`
		Version   string `json:"version"`
		StartedAt string `json:"started_at"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !body.OK {
		t.Errorf("ok = false, want true")
	}
	if body.Version == "" {
		t.Errorf("version empty")
	}
	if body.StartedAt == "" {
		t.Errorf("started_at empty")
	}
}

func TestUnknownAPIPathReturnsJSON404(t *testing.T) {
	h := New(&config.Config{}, state.New(), &topology.Topology{})
	req := httptest.NewRequest(http.MethodGet, "/api/does-not-exist", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"error"`) {
		t.Errorf("body missing error field: %q", body)
	}
	// Critically: this must NOT be the SPA shell. If it is, the route
	// ordering in New() is wrong.
	if strings.Contains(body, "SENTINEL OS") {
		t.Errorf("got SPA shell HTML for /api/* path — route ordering bug")
	}
}

func TestRootServesSPA(t *testing.T) {
	h := New(&config.Config{}, state.New(), &topology.Topology{})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "SENTINEL OS") {
		t.Errorf("body did not contain SPA shell; got %q", rec.Body.String())
	}
}

// TestApiRootPathReturnsJSON404 covers the bare /api path (no trailing
// slash). Without an explicit /api registration, Go 1.22 ServeMux falls
// through to the / catch-all, which serves the SPA shell — a real bug
// for any client or probe that hits the API base URL.
func TestApiRootPathReturnsJSON404(t *testing.T) {
	h := New(&config.Config{}, state.New(), &topology.Topology{})
	req := httptest.NewRequest(http.MethodGet, "/api", nil)
	// Worst case: browser-style Accept header. With the SPA handler reachable,
	// this would return HTML + 200. With the /api exact-path registration,
	// it must return JSON + 404 regardless.
	req.Header.Set("Accept", "text/html,application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	body := rec.Body.String()
	if strings.Contains(body, "SENTINEL OS") {
		t.Errorf("got SPA shell HTML for /api exact path — Go 1.22 ServeMux bare-path bug")
	}
}
