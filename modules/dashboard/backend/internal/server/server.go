// Package server assembles the dashboard HTTP handlers into a single mux.
package server

import (
	"encoding/json"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/config"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/adguard"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/spa"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/state"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/topology"
)

// Version is set at build time via -ldflags='-X .../internal/server.Version=...'.
// CI (build-dashboard.yml) passes the computed release version here so
// /api/health reports the exact release identifier. Local `go build` leaves
// it empty, at which point readVersion falls back to BuildInfo/VCS data.
var Version = ""

var (
	startedAt     = time.Now().UTC()
	runningVersion = readVersion()
)

// readVersion returns a best-effort identifier of the running binary.
//
// Priority order:
//  1. `Version` — set at link time by CI's `-ldflags -X`. Always wins when
//     non-empty. CI sets this even though the frontend dist rewrite makes
//     the working tree "dirty" from Go's VCS-stamping perspective.
//  2. Module version from a tagged `go install` (`info.Main.Version`).
//  3. VCS revision embedded by `go build -buildvcs=true` (short-SHA, plus
//     "-dirty" suffix when the working tree was modified). Useful for
//     local dev builds that don't pass -ldflags.
//  4. "dev" as a last resort.
func readVersion() string {
	if Version != "" {
		return Version
	}
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
func New(cfg *config.Config, st *state.State, _ *topology.Topology) http.Handler {
	mux := http.NewServeMux()

	// Specific API routes first.
	mux.HandleFunc("GET /api/health", handleHealth)
	mux.HandleFunc("GET /api/traffic", handleTraffic(st))
	mux.HandleFunc("GET /api/system", handleSystem(st))
	mux.HandleFunc("GET /api/tunnels", handleTunnels(st))
	mux.HandleFunc("GET /api/pools", handlePools(st))
	mux.HandleFunc("GET /api/clients", handleClients(st))
	mux.HandleFunc("GET /api/clients/{ip}", handleClientDetail(st))
	mux.HandleFunc("GET /api/adguard/stats", handleAdguardStats(st))

	qlCache := newQueryLogCache(adguard.NewClient(cfg.AdguardURL, nil))
	mux.HandleFunc("GET /api/adguard/querylog", handleAdguardQueryLog(qlCache))

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
		Version:   runningVersion,
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
