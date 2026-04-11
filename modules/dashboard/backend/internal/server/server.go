// Package server assembles the dashboard HTTP handlers into a single mux.
package server

import (
	"encoding/json"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/config"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/spa"
)

var (
	startedAt = time.Now().UTC()
	version   = readVersion()
)

// readVersion returns a best-effort identifier of the running binary.
//
// Priority order:
//  1. Module version from a tagged `go install` (`info.Main.Version`) — empty
//     for `go build` from a checkout, which is the normal CI flow.
//  2. VCS revision embedded by `go build -buildvcs=true` (default since
//     Go 1.18). Short-SHA plus a "-dirty" suffix when the working tree was
//     modified at build time. This is what CI produces.
//  3. "dev" as a last resort (non-go-built artifact, stripped binary, etc.).
func readVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "dev"
	}
	if v := info.Main.Version; v != "" && v != "(devel)" {
		return v
	}
	var rev, modified string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			modified = s.Value
		}
	}
	if rev != "" {
		short := rev
		if len(short) > 7 {
			short = short[:7]
		}
		if modified == "true" {
			short += "-dirty"
		}
		return short
	}
	return "dev"
}

// New returns the top-level http.Handler with all routes wired.
//
// Route precedence (Go 1.22 ServeMux):
//
//  1. GET /api/health          — the one real endpoint in Plan 1
//  2. /api                     — exact path, JSON 404 (no trailing slash variant)
//  3. /api/                    — subtree catch-all, JSON 404 for any unknown /api/*
//  4. /                        — SPA fallback via internal/spa.Handler()
//
// Both /api and /api/ are explicitly registered because Go 1.22 ServeMux
// does not unify them when a /-catch-all is also present: without the bare
// /api registration, a GET /api falls through to the SPA handler and
// returns the HTML shell instead of a JSON error.
func New(_ *config.Config) http.Handler {
	mux := http.NewServeMux()

	// Specific API routes first.
	mux.HandleFunc("GET /api/health", handleHealth)

	// Both the exact /api path and the /api/ subtree must be JSON 404 — see
	// comment above New.
	mux.HandleFunc("/api", handleAPINotFound)
	mux.HandleFunc("/api/", handleAPINotFound)

	// Everything else: SPA (client-side router resolves the path).
	mux.Handle("/", spa.Handler())

	return mux
}

type healthResponse struct {
	OK        bool      `json:"ok"`
	Version   string    `json:"version"`
	StartedAt time.Time `json:"started_at"`
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{
		OK:        true,
		Version:   version,
		StartedAt: startedAt,
	})
}

type apiError struct {
	Error string `json:"error"`
}

func handleAPINotFound(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotFound, apiError{
		Error: "no such api endpoint: " + r.URL.Path,
	})
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
