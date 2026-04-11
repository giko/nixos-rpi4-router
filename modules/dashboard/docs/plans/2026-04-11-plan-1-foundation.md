# Router Dashboard — Plan 1: Foundation

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship a deployable dashboard skeleton with one `/api/health` endpoint, firewall-gated access, and a working end-to-end build-and-deploy pipeline (source → CI → GitHub Release → flake → Pi).

**Architecture:** Go `net/http` binary with `//go:embed`ded React+shadcn/ui SPA. Distributed as a pre-built `aarch64-linux` binary via GitHub Release, consumed by `fetchurl` in a NixOS module gated on `allowedSources` + non-bootstrap `version.json`. GitHub Actions builds the binary on source changes, publishes a release, rebases+retries a `version.json` bump, and verifies the published URL against the committed hash. A companion workflow runs `nix flake check` on every Nix-file change.

**Tech Stack:** Go 1.22+, React 18, TypeScript, Vite, Tailwind CSS v3, shadcn/ui, TanStack Query v5, NixOS 25.11 modules, GitHub Actions.

**Reference spec:** `modules/dashboard/docs/spec.md` (this file lives at `modules/dashboard/docs/plans/2026-04-11-plan-1-foundation.md`, same repo)

**This is Plan 1 of 3:**

| Plan | Scope | Ships |
|---|---|---|
| **1 (this file) — Foundation** | Scaffolding, `/api/health`, NixOS module, CI workflows, bootstrap deploy | A working-but-empty dashboard accessible from admin IPs |
| **2 — Backend + core views** | All 13 collectors, 13 endpoints, shared frontend primitives, overview/pools/clients/adguard views | The dashboard with real data on the designer-delivered screens |
| **3 — Remaining views + polish** | VPN tunnels, traffic, firewall, QoS, system pages | v1 feature-complete |

**Plan 1 is NOT:**
- Not any real collectors (just `/api/health`)
- Not the full UI (just a "health: OK" card)
- Not rate limiting (defence-in-depth lands in Plan 2 with the real API surface)
- Not any production hardening beyond what the NixOS module already specifies

---

## Cross-repo note

Work spans **two git repositories**:

- **`/Users/giko/Documents/nixos-rpi4-router/`** — public flake. Owns: `modules/dashboard/{backend,frontend,*.nix,version.json}`, `.github/workflows/`. Most tasks commit here.
- **`/Users/giko/Documents/router/`** — local deployment repo. Owns: `configuration.nix` (where the dashboard is enabled), `flake.lock` (pinning the public flake). Only Task 20 touches this repo.

Each task's **Files:** section lists the absolute path so there is no ambiguity.

---

## File structure — Phase 1

Files introduced by this plan, grouped by directory:

```
/Users/giko/Documents/nixos-rpi4-router/
├── flake.nix                                                        # [modify] expose nixosModules.dashboard
├── modules/dashboard/
│   ├── default.nix                                                  # [create] NixOS module (options, assertions, firewall gate, systemd)
│   ├── package.nix                                                  # [create] fetchurl-based derivation
│   ├── version.json                                                 # [create] bootstrap placeholder { version: "bootstrap", ... }
│   ├── backend/
│   │   ├── go.mod                                                   # [create] module github.com/giko/nixos-rpi4-router/modules/dashboard/backend
│   │   ├── go.sum                                                   # [create] (empty initially — no external deps in Phase 1)
│   │   ├── cmd/dashboard/main.go                                    # [create] entry point
│   │   └── internal/
│   │       ├── config/
│   │       │   ├── config.go                                        # [create] flag parsing
│   │       │   └── config_test.go                                   # [create] flag parsing test
│   │       ├── server/
│   │       │   ├── server.go                                        # [create] mux + /api/health handler
│   │       │   └── server_test.go                                   # [create] /api/health test
│   │       └── spa/
│   │           ├── spa.go                                           # [create] //go:embed + SPA fallback handler
│   │           ├── spa_test.go                                      # [create] SPA handler test
│   │           └── dist/
│   │               └── index.html                                   # [create] placeholder (CI overwrites)
│   └── frontend/
│       ├── package.json                                             # [create] react + @tanstack/react-query + vite + tailwind
│       ├── package-lock.json                                        # [create] from npm install
│       ├── tsconfig.json                                            # [create]
│       ├── tsconfig.node.json                                       # [create]
│       ├── vite.config.ts                                           # [create] proxy /api → 127.0.0.1:9090 for dev
│       ├── tailwind.config.ts                                       # [create] Silent Sentinel tokens
│       ├── postcss.config.js                                        # [create]
│       ├── index.html                                               # [create] <html class="dark">
│       └── src/
│           ├── main.tsx                                             # [create] React entry + inline QueryClient
│           ├── App.tsx                                              # [create] single /api/health card (no router)
│           └── index.css                                            # [create] Tailwind base + CSS vars for Silent Sentinel palette
└── .github/workflows/
    ├── build-dashboard.yml                                          # [create] full CI pipeline
    └── nix-evaluation-check.yml                                     # [create] `nix flake check` on Nix changes

/Users/giko/Documents/router/
└── configuration.nix                                                # [modify] enable router.dashboard with allowedSources
```

---

## Tasks

### Task 1: Scaffold backend Go module

**Goal:** A `go.mod` + empty `main.go` that builds, proving the Go toolchain + module path work before writing any real code.

**Files:**
- Create: `/Users/giko/Documents/nixos-rpi4-router/modules/dashboard/backend/go.mod`
- Create: `/Users/giko/Documents/nixos-rpi4-router/modules/dashboard/backend/cmd/dashboard/main.go`

- [ ] **Step 1: Create the Go module**

Run from `/Users/giko/Documents/nixos-rpi4-router/modules/dashboard/backend`:

```bash
mkdir -p cmd/dashboard
cd /Users/giko/Documents/nixos-rpi4-router/modules/dashboard/backend
go mod init github.com/giko/nixos-rpi4-router/modules/dashboard/backend
```

Expected `go.mod`:

```
module github.com/giko/nixos-rpi4-router/modules/dashboard/backend

go 1.22
```

- [ ] **Step 2: Write a no-op `main.go`**

Create `cmd/dashboard/main.go`:

```go
package main

func main() {
}
```

- [ ] **Step 3: Verify it builds**

```bash
cd /Users/giko/Documents/nixos-rpi4-router/modules/dashboard/backend
go build ./cmd/dashboard
```

Expected: exit 0, produces `./dashboard` binary. Delete it: `rm dashboard`.

- [ ] **Step 4: Do NOT commit yet**

Tasks 1-7 all land in one commit at Task 8. Leave the working tree dirty.

---

### Task 2: Add config flag parsing

**Goal:** `internal/config.Config` with flag-parsed fields (bind, adguard URL, log level). TDD: write the test, verify it fails, implement, verify it passes.

**Files:**
- Create: `/Users/giko/Documents/nixos-rpi4-router/modules/dashboard/backend/internal/config/config.go`
- Create: `/Users/giko/Documents/nixos-rpi4-router/modules/dashboard/backend/internal/config/config_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/config/config_test.go`:

```go
package config

import (
	"testing"
)

func TestFromFlagsDefaults(t *testing.T) {
	cfg, err := FromFlags(nil)
	if err != nil {
		t.Fatalf("FromFlags(nil) error: %v", err)
	}
	if cfg.Bind != "127.0.0.1:9090" {
		t.Errorf("Bind = %q, want 127.0.0.1:9090", cfg.Bind)
	}
	if cfg.AdguardURL != "http://127.0.0.1:3000" {
		t.Errorf("AdguardURL = %q, want http://127.0.0.1:3000", cfg.AdguardURL)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel = %q, want info", cfg.LogLevel)
	}
}

func TestFromFlagsOverrides(t *testing.T) {
	cfg, err := FromFlags([]string{
		"--bind", "192.168.1.1:9090",
		"--adguard-url", "http://adguard.local",
		"--log-level", "debug",
	})
	if err != nil {
		t.Fatalf("FromFlags error: %v", err)
	}
	if cfg.Bind != "192.168.1.1:9090" {
		t.Errorf("Bind = %q", cfg.Bind)
	}
	if cfg.AdguardURL != "http://adguard.local" {
		t.Errorf("AdguardURL = %q", cfg.AdguardURL)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q", cfg.LogLevel)
	}
}
```

- [ ] **Step 2: Run the test, confirm it fails**

```bash
cd /Users/giko/Documents/nixos-rpi4-router/modules/dashboard/backend
go test ./internal/config/...
```

Expected: `FAIL — internal/config: no Go files` or `undefined: FromFlags`. Either is acceptable — it proves the test is running and the implementation doesn't exist yet.

- [ ] **Step 3: Implement `config.go`**

Create `internal/config/config.go`:

```go
// Package config parses command-line flags into a Config struct
// used by the dashboard process.
package config

import (
	"flag"
	"fmt"
)

// Config holds the runtime configuration for the dashboard binary.
type Config struct {
	Bind       string // address:port to bind the HTTP server to
	AdguardURL string // base URL for the AdGuard Home REST API
	LogLevel   string // slog level: debug | info | warn | error
}

// FromFlags parses args into a Config. Pass os.Args[1:] or nil for defaults.
func FromFlags(args []string) (*Config, error) {
	fs := flag.NewFlagSet("dashboard", flag.ContinueOnError)
	bind := fs.String("bind", "127.0.0.1:9090", "host:port to bind on")
	adguard := fs.String("adguard-url", "http://127.0.0.1:3000", "AdGuard Home base URL")
	level := fs.String("log-level", "info", "slog level: debug|info|warn|error")
	if err := fs.Parse(args); err != nil {
		return nil, fmt.Errorf("parse flags: %w", err)
	}
	return &Config{
		Bind:       *bind,
		AdguardURL: *adguard,
		LogLevel:   *level,
	}, nil
}
```

- [ ] **Step 4: Run the test, confirm it passes**

```bash
cd /Users/giko/Documents/nixos-rpi4-router/modules/dashboard/backend
go test ./internal/config/... -v
```

Expected: `PASS` for both `TestFromFlagsDefaults` and `TestFromFlagsOverrides`.

---

### Task 3: Embed static assets + SPA fallback (TDD)

**Goal:** A Go HTTP handler that serves `//go:embed`ded SPA assets, with a fallback that returns `index.html` for any path that doesn't correspond to a real file (so React's client-side router can handle `/vpn/pools`, `/clients/1.2.3.4`, etc.).

**File layout note.** `//go:embed` does not support parent-path (`..`) segments, so the embedding Go file must sit next to its asset directory. We put both in a dedicated `internal/spa/` package with a `dist/` subdirectory. A `dist/index.html` placeholder is committed so the embed directive compiles cleanly in local dev before the frontend has ever been built; the CI pipeline overwrites `dist/` with the real Vite output before `go build`.

**Files:**
- Create: `/Users/giko/Documents/nixos-rpi4-router/modules/dashboard/backend/internal/spa/spa.go`
- Create: `/Users/giko/Documents/nixos-rpi4-router/modules/dashboard/backend/internal/spa/spa_test.go`
- Create: `/Users/giko/Documents/nixos-rpi4-router/modules/dashboard/backend/internal/spa/dist/index.html` (placeholder)

- [ ] **Step 1: Create the placeholder `dist/index.html`**

```bash
mkdir -p /Users/giko/Documents/nixos-rpi4-router/modules/dashboard/backend/internal/spa/dist
```

Create `internal/spa/dist/index.html`:

```html
<!doctype html>
<html lang="en">
  <head><meta charset="utf-8"><title>SENTINEL OS</title></head>
  <body><div id="root">placeholder</div></body>
</html>
```

- [ ] **Step 2: Write the failing test**

Create `internal/spa/spa_test.go`:

```go
package spa

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerServesIndex(t *testing.T) {
	h := Handler()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	if !strings.Contains(string(body), "SENTINEL OS") {
		t.Errorf("body missing SENTINEL OS; got %q", body)
	}
}

func TestHandlerFallsBackToIndexForSpaRoute(t *testing.T) {
	h := Handler()
	// A non-existent path like /vpn/pools should still serve index.html
	// so the React client-side router can pick it up.
	req := httptest.NewRequest(http.MethodGet, "/vpn/pools", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	if !strings.Contains(string(body), "SENTINEL OS") {
		t.Errorf("SPA fallback did not serve index.html; got %q", body)
	}
}
```

- [ ] **Step 3: Run the test, confirm it fails**

```bash
cd /Users/giko/Documents/nixos-rpi4-router/modules/dashboard/backend
go test ./internal/spa/...
```

Expected: `undefined: Handler`. Good.

- [ ] **Step 4: Implement `spa.go`**

Create `internal/spa/spa.go`:

```go
// Package spa holds the embedded React SPA build output and exposes an
// http.Handler that serves it with an SPA-style fallback to index.html.
// The dist/ subdirectory is populated from modules/dashboard/frontend/dist
// during the CI build, before `go build` is invoked. A committed placeholder
// index.html keeps the embed directive compiling in local dev.
package spa

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed all:dist
var fsys embed.FS

// Handler returns an http.Handler that serves embedded SPA assets. Requests
// for paths that do not correspond to a real file are served the embedded
// index.html so that the React client-side router can resolve them.
func Handler() http.Handler {
	sub, err := fs.Sub(fsys, "dist")
	if err != nil {
		// Impossible in practice — the embed directive guarantees the path.
		panic(err)
	}
	fileServer := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := fs.Stat(sub, path); err != nil {
			// Not a real file — serve index.html for SPA client-side routing.
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/"
			fileServer.ServeHTTP(w, r2)
			return
		}
		fileServer.ServeHTTP(w, r)
	})
}
```

- [ ] **Step 5: Run the test, confirm it passes**

```bash
cd /Users/giko/Documents/nixos-rpi4-router/modules/dashboard/backend
go test ./internal/spa/... -v
```

Expected: `PASS` for both `TestHandlerServesIndex` and `TestHandlerFallsBackToIndexForSpaRoute`.

---

### Task 4: `/api/health` handler (TDD)

**Goal:** An `http.Handler` at `GET /api/health` that returns `{ok: true, version, started_at}` as JSON. This is the only endpoint Plan 1 ships.

**Files:**
- Create: `/Users/giko/Documents/nixos-rpi4-router/modules/dashboard/backend/internal/server/server.go`
- Create: `/Users/giko/Documents/nixos-rpi4-router/modules/dashboard/backend/internal/server/server_test.go`

- [ ] **Step 1: Write the failing test**

Create `internal/server/server_test.go`:

```go
package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/config"
)

func TestHealthEndpoint(t *testing.T) {
	h := New(&config.Config{})
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
```

- [ ] **Step 2: Run, confirm it fails**

```bash
cd /Users/giko/Documents/nixos-rpi4-router/modules/dashboard/backend
go test ./internal/server/...
```

Expected: `undefined: New`. Good.

- [ ] **Step 3: Implement `server.go`**

Create `internal/server/server.go`:

```go
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

func readVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return "dev"
}

// New returns the top-level http.Handler with all routes wired.
func New(_ *config.Config) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", handleHealth)
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

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
```

- [ ] **Step 4: Run, confirm it passes**

```bash
cd /Users/giko/Documents/nixos-rpi4-router/modules/dashboard/backend
go test ./... -v
```

Expected: `PASS` for `config`, `spa`, and `server` packages.

---

### Task 5: `main.go` wires config + server + graceful shutdown

**Goal:** A runnable binary. `main` parses flags, starts the server, handles SIGINT/SIGTERM for graceful shutdown.

**Files:**
- Modify: `/Users/giko/Documents/nixos-rpi4-router/modules/dashboard/backend/cmd/dashboard/main.go`

- [ ] **Step 1: Replace the no-op main with real wiring**

Overwrite `cmd/dashboard/main.go`:

```go
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/config"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/server"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.FromFlags(os.Args[1:])
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}

	httpServer := &http.Server{
		Addr:              cfg.Bind,
		Handler:           server.New(cfg),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		slog.Info("dashboard starting", "bind", cfg.Bind)
		errCh <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("listen", "err", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		slog.Info("dashboard stopping")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("shutdown", "err", err)
			os.Exit(1)
		}
	}
}
```

- [ ] **Step 2: Run smoke test**

```bash
cd /Users/giko/Documents/nixos-rpi4-router/modules/dashboard/backend
go build -o /tmp/dashboard ./cmd/dashboard
/tmp/dashboard --bind 127.0.0.1:19090 &
PID=$!
sleep 1
curl -s http://127.0.0.1:19090/api/health
echo
kill $PID
rm /tmp/dashboard
```

Expected output (timestamps will differ):

```json
{"ok":true,"version":"dev","started_at":"2026-04-11T..."}
```

The binary should exit cleanly within ~1 second of SIGTERM.

- [ ] **Step 3: Also verify the SPA fallback via the running binary**

Re-run it, then:

```bash
curl -s http://127.0.0.1:19090/
curl -s http://127.0.0.1:19090/vpn/pools
```

Both should return the placeholder `index.html` containing `SENTINEL OS`.

---

### Task 6: Scaffold frontend (Vite + React + TS + Tailwind)

**Goal:** A minimal Vite + React + TypeScript + Tailwind CSS app that builds to `dist/` and can be served standalone via `npm run dev` for iteration.

**Files:**
- Create: `/Users/giko/Documents/nixos-rpi4-router/modules/dashboard/frontend/package.json`
- Create: `/Users/giko/Documents/nixos-rpi4-router/modules/dashboard/frontend/tsconfig.json`
- Create: `/Users/giko/Documents/nixos-rpi4-router/modules/dashboard/frontend/tsconfig.node.json`
- Create: `/Users/giko/Documents/nixos-rpi4-router/modules/dashboard/frontend/vite.config.ts`
- Create: `/Users/giko/Documents/nixos-rpi4-router/modules/dashboard/frontend/tailwind.config.ts`
- Create: `/Users/giko/Documents/nixos-rpi4-router/modules/dashboard/frontend/postcss.config.js`
- Create: `/Users/giko/Documents/nixos-rpi4-router/modules/dashboard/frontend/index.html`
- Create: `/Users/giko/Documents/nixos-rpi4-router/modules/dashboard/frontend/src/main.tsx`
- Create: `/Users/giko/Documents/nixos-rpi4-router/modules/dashboard/frontend/src/App.tsx`
- Create: `/Users/giko/Documents/nixos-rpi4-router/modules/dashboard/frontend/src/index.css`

- [ ] **Step 1: Create `package.json`**

```json
{
  "name": "router-dashboard-frontend",
  "version": "0.1.0",
  "private": true,
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc -b && vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "react": "^18.3.1",
    "react-dom": "^18.3.1",
    "@tanstack/react-query": "^5.56.2"
  },
  "devDependencies": {
    "@types/react": "^18.3.11",
    "@types/react-dom": "^18.3.0",
    "@vitejs/plugin-react": "^4.3.2",
    "autoprefixer": "^10.4.20",
    "postcss": "^8.4.47",
    "tailwindcss": "^3.4.13",
    "typescript": "^5.6.3",
    "vite": "^5.4.9"
  }
}
```

- [ ] **Step 2: Create `tsconfig.json`**

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "lib": ["ES2022", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "skipLibCheck": true,
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "isolatedModules": true,
    "moduleDetection": "force",
    "noEmit": true,
    "jsx": "react-jsx",
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true,
    "baseUrl": ".",
    "paths": { "@/*": ["src/*"] }
  },
  "include": ["src"],
  "references": [{ "path": "./tsconfig.node.json" }]
}
```

- [ ] **Step 3: Create `tsconfig.node.json`**

```json
{
  "compilerOptions": {
    "target": "ES2022",
    "lib": ["ES2023"],
    "module": "ESNext",
    "skipLibCheck": true,
    "moduleResolution": "bundler",
    "allowSyntheticDefaultImports": true,
    "strict": true,
    "noEmit": true
  },
  "include": ["vite.config.ts", "tailwind.config.ts"]
}
```

- [ ] **Step 4: Create `vite.config.ts`**

```ts
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import path from "node:path";

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: { "@": path.resolve(__dirname, "src") },
  },
  server: {
    port: 5173,
    proxy: {
      "/api": {
        target: "http://127.0.0.1:9090",
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
});
```

- [ ] **Step 5: Create `tailwind.config.ts` (Silent Sentinel tokens placeholder)**

```ts
import type { Config } from "tailwindcss";

const config: Config = {
  darkMode: ["class"],
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        // Silent Sentinel palette — mapped onto shadcn's CSS-variable contract
        // in src/index.css. Tailwind references the variables, shadcn uses
        // `hsl(var(--foo))` everywhere.
        border: "hsl(var(--border))",
        input: "hsl(var(--input))",
        ring: "hsl(var(--ring))",
        background: "hsl(var(--background))",
        foreground: "hsl(var(--foreground))",
        primary: {
          DEFAULT: "hsl(var(--primary))",
          foreground: "hsl(var(--primary-foreground))",
        },
        secondary: {
          DEFAULT: "hsl(var(--secondary))",
          foreground: "hsl(var(--secondary-foreground))",
        },
        muted: {
          DEFAULT: "hsl(var(--muted))",
          foreground: "hsl(var(--muted-foreground))",
        },
        accent: {
          DEFAULT: "hsl(var(--accent))",
          foreground: "hsl(var(--accent-foreground))",
        },
        destructive: {
          DEFAULT: "hsl(var(--destructive))",
          foreground: "hsl(var(--destructive-foreground))",
        },
        card: {
          DEFAULT: "hsl(var(--card))",
          foreground: "hsl(var(--card-foreground))",
        },
      },
      fontFamily: {
        sans: ["Inter", "system-ui", "sans-serif"],
        mono: ["JetBrains Mono", "Fira Code", "monospace"],
      },
      borderRadius: {
        lg: "var(--radius)",
        md: "calc(var(--radius) - 2px)",
        sm: "calc(var(--radius) - 4px)",
      },
    },
  },
  plugins: [],
};

export default config;
```

- [ ] **Step 6: Create `postcss.config.js`**

```js
export default {
  plugins: {
    tailwindcss: {},
    autoprefixer: {},
  },
};
```

- [ ] **Step 7: Create `index.html`**

```html
<!doctype html>
<html lang="en" class="dark">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>SENTINEL OS</title>
  </head>
  <body class="bg-background text-foreground">
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

- [ ] **Step 8: Create `src/index.css` (Silent Sentinel CSS variables)**

```css
@tailwind base;
@tailwind components;
@tailwind utilities;

@layer base {
  :root {
    /* Light mode is unused in Plan 1; placeholder so shadcn doesn't choke. */
    --background: 0 0% 100%;
    --foreground: 0 0% 9%;
    --card: 0 0% 100%;
    --card-foreground: 0 0% 9%;
    --primary: 0 0% 9%;
    --primary-foreground: 0 0% 98%;
    --secondary: 0 0% 96%;
    --secondary-foreground: 0 0% 9%;
    --muted: 0 0% 96%;
    --muted-foreground: 0 0% 45%;
    --accent: 0 0% 96%;
    --accent-foreground: 0 0% 9%;
    --destructive: 0 84% 60%;
    --destructive-foreground: 0 0% 98%;
    --border: 0 0% 89%;
    --input: 0 0% 89%;
    --ring: 0 0% 63%;
    --radius: 0.5rem;
  }

  .dark {
    /* Silent Sentinel dark palette — maps hex tokens from design-system.md
       onto shadcn's HSL convention. Exact values derived from
       modules/dashboard/designs/design-system.md section 2. */
    --background: 0 0% 5%;           /* #0e0e0e surface */
    --foreground: 20 7% 90%;          /* #e7e5e4 on_surface */
    --card: 0 0% 10%;                 /* #191a1a surface_container */
    --card-foreground: 20 7% 90%;
    --primary: 239 100% 88%;          /* #c0c1ff primary */
    --primary-foreground: 240 66% 25%;
    --secondary: 0 0% 12%;            /* #1f2020 surface_container_high */
    --secondary-foreground: 20 7% 90%;
    --muted: 0 0% 7%;                 /* #131313 surface_container_low */
    --muted-foreground: 0 1% 67%;     /* #acabaa on_surface_variant */
    --accent: 0 0% 15%;               /* #252626 surface_container_highest */
    --accent-foreground: 20 7% 90%;
    --destructive: 352 72% 70%;       /* #ec7c8a error */
    --destructive-foreground: 20 7% 90%;
    --border: 0 0% 28%;               /* #484848 outline_variant (used at 15% opacity) */
    --input: 0 0% 15%;
    --ring: 239 100% 88%;
    --radius: 0.375rem;
  }
}

@layer base {
  body {
    @apply bg-background text-foreground;
    font-feature-settings: "rlig" 1, "calt" 1;
  }
  /* Tabular numerals for counters so they don't jitter while updating. */
  code,
  .font-mono {
    font-variant-numeric: tabular-nums;
  }
}
```

- [ ] **Step 9: Create `src/main.tsx`**

```tsx
import React from "react";
import ReactDOM from "react-dom/client";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { App } from "./App";
import "./index.css";

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 2_000,
      refetchOnWindowFocus: false,
      retry: 2,
    },
  },
});

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <QueryClientProvider client={queryClient}>
      <App />
    </QueryClientProvider>
  </React.StrictMode>,
);
```

- [ ] **Step 10: Create `src/App.tsx`**

```tsx
import { useQuery } from "@tanstack/react-query";

type HealthResponse = {
  ok: boolean;
  version: string;
  started_at: string;
};

async function fetchHealth(): Promise<HealthResponse> {
  const res = await fetch("/api/health");
  if (!res.ok) {
    throw new Error(`health endpoint returned ${res.status}`);
  }
  return res.json();
}

export function App() {
  const { data, isLoading, error } = useQuery({
    queryKey: ["health"],
    queryFn: fetchHealth,
    refetchInterval: 5_000,
  });

  return (
    <main className="min-h-screen flex items-center justify-center p-8">
      <div className="bg-card border border-border/20 rounded-md p-8 min-w-80">
        <h1 className="text-xs tracking-widest uppercase text-muted-foreground mb-2">
          SENTINEL OS
        </h1>
        <p className="text-2xl font-semibold mb-4">Router Dashboard</p>
        {isLoading && (
          <p className="text-muted-foreground font-mono text-sm">…loading health…</p>
        )}
        {error && (
          <p className="text-destructive font-mono text-sm">
            health: {(error as Error).message}
          </p>
        )}
        {data && (
          <div className="font-mono text-sm space-y-1">
            <p>
              <span className="text-muted-foreground">status: </span>
              <span className="text-primary">{data.ok ? "OK" : "FAIL"}</span>
            </p>
            <p>
              <span className="text-muted-foreground">version: </span>
              {data.version}
            </p>
            <p>
              <span className="text-muted-foreground">started: </span>
              {new Date(data.started_at).toLocaleString()}
            </p>
          </div>
        )}
      </div>
    </main>
  );
}
```

- [ ] **Step 11: Install dependencies and build**

```bash
cd /Users/giko/Documents/nixos-rpi4-router/modules/dashboard/frontend
npm install
npm run build
ls dist/
```

Expected: `dist/` contains `index.html`, `assets/index-<hash>.js`, `assets/index-<hash>.css`, and possibly a favicon/vite.svg. If the build fails, fix before proceeding.

- [ ] **Step 12: Verify the build output embeds correctly**

```bash
cp -r /Users/giko/Documents/nixos-rpi4-router/modules/dashboard/frontend/dist/* \
      /Users/giko/Documents/nixos-rpi4-router/modules/dashboard/backend/internal/spa/dist/
cd /Users/giko/Documents/nixos-rpi4-router/modules/dashboard/backend
go test ./internal/spa/... -v
go build -o /tmp/dashboard ./cmd/dashboard
/tmp/dashboard --bind 127.0.0.1:19090 &
PID=$!
sleep 1
curl -s http://127.0.0.1:19090/ | head -c 200
echo
kill $PID
rm /tmp/dashboard
```

Expected: the curl returns the full Vite-built `index.html` (starts with `<!doctype html>`, references `/assets/index-...js`). Not the placeholder.

- [ ] **Step 13: Clean up the embedded dist, leaving only the placeholder**

CI rebuilds `dist/` before `go build`, so the checked-in version should be the placeholder only:

```bash
rm -rf /Users/giko/Documents/nixos-rpi4-router/modules/dashboard/backend/internal/spa/dist/*
cat > /Users/giko/Documents/nixos-rpi4-router/modules/dashboard/backend/internal/spa/dist/index.html <<'EOF'
<!doctype html>
<html lang="en">
  <head><meta charset="utf-8"><title>SENTINEL OS</title></head>
  <body><div id="root">placeholder</div></body>
</html>
EOF
```

---

### Task 7: `.gitignore` for the dashboard subtree

**Goal:** Don't commit generated files (node_modules, Vite's real dist output, Go binary artifacts).

**Files:**
- Create: `/Users/giko/Documents/nixos-rpi4-router/modules/dashboard/.gitignore`

- [ ] **Step 1: Create the file**

```gitignore
# Frontend build artifacts
frontend/node_modules/
frontend/dist/

# Backend build artifacts
backend/dashboard
backend/cover.out

# Embedded frontend — the checked-in placeholder stays, but CI overwrites it.
# We only ignore files other than index.html so the placeholder survives.
backend/internal/spa/dist/assets/
backend/internal/spa/dist/*.svg
backend/internal/spa/dist/*.ico
```

---

### Task 8: Commit checkpoint 1 — backend + frontend scaffolding

**Goal:** Land Tasks 1-7 as one clean commit in the public flake.

- [ ] **Step 1: Verify the working tree**

```bash
cd /Users/giko/Documents/nixos-rpi4-router
git status --short
```

Expected: new files under `modules/dashboard/{backend,frontend}/`. Roughly:

```
A  modules/dashboard/.gitignore
A  modules/dashboard/backend/cmd/dashboard/main.go
A  modules/dashboard/backend/go.mod
A  modules/dashboard/backend/go.sum
A  modules/dashboard/backend/internal/config/config.go
A  modules/dashboard/backend/internal/config/config_test.go
A  modules/dashboard/backend/internal/server/server.go
A  modules/dashboard/backend/internal/server/server_test.go
A  modules/dashboard/backend/internal/spa/dist/index.html
A  modules/dashboard/backend/internal/spa/spa.go
A  modules/dashboard/backend/internal/spa/spa_test.go
A  modules/dashboard/frontend/index.html
A  modules/dashboard/frontend/package-lock.json
A  modules/dashboard/frontend/package.json
A  modules/dashboard/frontend/postcss.config.js
A  modules/dashboard/frontend/src/App.tsx
A  modules/dashboard/frontend/src/index.css
A  modules/dashboard/frontend/src/main.tsx
A  modules/dashboard/frontend/tailwind.config.ts
A  modules/dashboard/frontend/tsconfig.json
A  modules/dashboard/frontend/tsconfig.node.json
A  modules/dashboard/frontend/vite.config.ts
```

- [ ] **Step 2: Run the full Go test suite one more time**

```bash
cd /Users/giko/Documents/nixos-rpi4-router/modules/dashboard/backend
go test ./...
```

Expected: all `PASS`.

- [ ] **Step 3: Commit**

```bash
cd /Users/giko/Documents/nixos-rpi4-router
git add modules/dashboard/.gitignore \
        modules/dashboard/backend \
        modules/dashboard/frontend
git commit -m "dashboard: scaffold go backend + react frontend"
```

Commit message rationale: short, past-tense style matches existing commits (`Add per-connection VPN pooling with wg-pool-health watchdog`, etc.). No personal names, no Co-Authored-By trailer.

- [ ] **Step 4: Do NOT push yet**

The next commits land on top. Push happens at Task 17.

---

### Task 9: Bootstrap `version.json`

**Goal:** A placeholder `version.json` so the NixOS module's `mkIf` guard can no-op before CI has published a real release.

**Files:**
- Create: `/Users/giko/Documents/nixos-rpi4-router/modules/dashboard/version.json`

- [ ] **Step 1: Create the file**

```json
{
  "version": "bootstrap",
  "url": "",
  "hash": ""
}
```

The empty `url` and `hash` are fine because `lib.mkIf` in `default.nix` (Task 11) short-circuits the `fetchurl` call entirely when `version == "bootstrap"`.

---

### Task 10: `package.nix`

**Goal:** A Nix derivation that fetches the published binary, installs it to `$out/bin/dashboard`, and refuses to evaluate on non-`aarch64-linux` platforms.

**Files:**
- Create: `/Users/giko/Documents/nixos-rpi4-router/modules/dashboard/package.nix`

- [ ] **Step 1: Write the derivation**

```nix
# Pre-built dashboard binary. The binary is produced by
# `.github/workflows/build-dashboard.yml`, published as a GitHub Release
# asset, and pinned here via `version.json`. CI auto-commits bumps to
# `version.json` after each successful release.
{ stdenvNoCC
, fetchurl
, lib
}:

let
  v = builtins.fromJSON (builtins.readFile ./version.json);
in
stdenvNoCC.mkDerivation {
  pname = "router-dashboard";
  version = v.version;

  # The placeholder "bootstrap" version evaluates lazily — it is never
  # realized because the NixOS module's mkIf guard prevents the package
  # from being used until a real version lands.
  src =
    if v.version == "bootstrap"
    then throw "router-dashboard is in bootstrap state; CI has not published a release yet"
    else fetchurl { url = v.url; hash = v.hash; };

  dontUnpack = true;

  installPhase = ''
    runHook preInstall
    install -Dm755 $src $out/bin/dashboard
    runHook postInstall
  '';

  meta = with lib; {
    description = "Read-only web dashboard for the nixos-rpi4-router (see modules/dashboard/docs/spec.md)";
    platforms = [ "aarch64-linux" ];
    license = licenses.mit;
    maintainers = [ ];
  };
}
```

---

### Task 11: `default.nix` — NixOS module

**Goal:** The NixOS module with all options, assertions, firewall gate, and systemd unit from spec §6.

**Files:**
- Create: `/Users/giko/Documents/nixos-rpi4-router/modules/dashboard/default.nix`

- [ ] **Step 1: Write the module**

```nix
{ config, lib, pkgs, ... }:

let
  cfg = config.router.dashboard;
  v = builtins.fromJSON (builtins.readFile ./version.json);
in
{
  options.router.dashboard = {
    enable = lib.mkEnableOption "the router dashboard";

    allowedSources = lib.mkOption {
      type = lib.types.listOf lib.types.str;
      default = [ ];
      example = [ "192.168.1.117" "192.168.1.119" ];
      description = ''
        Source IPv4 addresses or CIDR ranges permitted to reach the dashboard
        port. Anything not listed here is dropped by an nftables rule generated
        in the router's input chain, before any HTTP handler runs.

        Must be non-empty for the module to activate (enforced by assertion).
        This is the primary trust boundary for the dashboard — pick only admin
        devices you actually use to view it, and give them static DHCP leases
        via `router.dhcp.staticLeases` so their IPs are stable.

        IPv6 is not currently supported (the router has IPv6 disabled end-to-end).
      '';
    };

    port = lib.mkOption {
      type = lib.types.port;
      default = 9090;
      description = "TCP port to bind the dashboard HTTP server on.";
    };

    bindAddress = lib.mkOption {
      type = lib.types.str;
      default = (builtins.head config.router.lan.addresses).address;
      description = "Address to bind. Defaults to the primary LAN address.";
    };

    adguardUrl = lib.mkOption {
      type = lib.types.str;
      default = "http://127.0.0.1:3000";
      description = "Base URL for the AdGuard Home REST API.";
    };

    package = lib.mkOption {
      type = lib.types.package;
      default = pkgs.callPackage ./package.nix { };
      description = "The dashboard package to run.";
    };

    logLevel = lib.mkOption {
      type = lib.types.enum [ "debug" "info" "warn" "error" ];
      default = "info";
      description = "slog level.";
    };
  };

  config = lib.mkIf (cfg.enable && v.version != "bootstrap") {
    assertions = [
      {
        assertion = cfg.allowedSources != [ ];
        message = ''
          router.dashboard.enable = true requires router.dashboard.allowedSources
          to be non-empty. The dashboard aggregates sensitive data (per-client DNS
          history, MAC/IP mapping, routing state) and must not be reachable
          LAN-wide. Declare the IPs or CIDR ranges of admin devices that should
          reach the dashboard — everything else is dropped at the firewall.
        '';
      }
      {
        assertion = !(lib.any (pf: pf.externalPort == cfg.port) config.router.portForwards);
        message = "router.dashboard.port (${toString cfg.port}) collides with a router.portForwards entry.";
      }
    ];

    router.nftables.extraInputRules =
      let
        srcSet = lib.concatStringsSep ", " cfg.allowedSources;
        lanIf = config.router.lan.interface;
      in
      ''
        iifname "${lanIf}" tcp dport ${toString cfg.port} ip saddr { ${srcSet} } accept
        iifname "${lanIf}" tcp dport ${toString cfg.port} drop
      '';

    systemd.services.router-dashboard = {
      description = "Router observability dashboard";
      wantedBy = [ "multi-user.target" ];
      after = [
        "network-online.target"
        "nftables.service"
        "adguardhome.service"
      ];
      wants = [ "network-online.target" ];
      path = [
        pkgs.nftables
        pkgs.wireguard-tools
        pkgs.iproute2
        pkgs.conntrack-tools
        pkgs.iputils
      ];

      serviceConfig = {
        ExecStart = "${cfg.package}/bin/dashboard"
          + " --bind=${cfg.bindAddress}:${toString cfg.port}"
          + " --adguard-url=${cfg.adguardUrl}"
          + " --log-level=${cfg.logLevel}";
        Restart = "on-failure";
        RestartSec = 3;

        DynamicUser = true;
        AmbientCapabilities = [ "CAP_NET_ADMIN" "CAP_NET_RAW" ];
        CapabilityBoundingSet = [ "CAP_NET_ADMIN" "CAP_NET_RAW" ];

        NoNewPrivileges = true;
        ProtectSystem = "strict";
        ProtectHome = true;
        ProtectKernelTunables = true;
        ProtectKernelModules = true;
        ProtectControlGroups = true;
        RestrictNamespaces = true;
        RestrictRealtime = true;
        LockPersonality = true;
        MemoryDenyWriteExecute = true;
        SystemCallArchitectures = "native";

        ReadOnlyPaths = [
          "/run/wg-pool-health"
          "/var/lib/dnsmasq"
          "/sys/class/thermal"
          "/sys/devices/virtual/thermal"
        ];
      };
    };
  };
}
```

---

### Task 12: Wire `default.nix` into the top-level flake

**Goal:** Expose `nixosModules.dashboard` from `flake.nix`. Do NOT add it to `nixosModules.default` — opt-in only.

**Files:**
- Modify: `/Users/giko/Documents/nixos-rpi4-router/flake.nix`

- [ ] **Step 1: Read the current `flake.nix`**

```bash
cat /Users/giko/Documents/nixos-rpi4-router/flake.nix
```

Expected current content (approximately):

```nix
{
  description = "NixOS router modules for Raspberry Pi 4 (2-port, WireGuard, AdGuard, QoS)";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.11";

  outputs = { self, nixpkgs, ... }: {
    nixosModules = {
      default     = import ./modules;
      kernel      = import ./modules/kernel.nix;
      interfaces  = import ./modules/interfaces.nix;
      wireguard   = import ./modules/wireguard.nix;
      nftables    = import ./modules/nftables.nix;
      pbr         = import ./modules/pbr.nix;
      dns         = import ./modules/dns.nix;
      dhcp        = import ./modules/dhcp.nix;
      qos         = import ./modules/qos.nix;
      upnp        = import ./modules/upnp.nix;
      performance = import ./modules/performance.nix;
      services    = import ./modules/services.nix;
    };
  };
}
```

- [ ] **Step 2: Add the dashboard entry**

Edit `flake.nix` to insert one line in the `nixosModules` attrset:

```nix
      dashboard   = import ./modules/dashboard;
```

Place it alphabetically after `dhcp` (or wherever the other `dashboard` would naturally sort). The new `nixosModules` attrset becomes:

```nix
    nixosModules = {
      default     = import ./modules;
      kernel      = import ./modules/kernel.nix;
      interfaces  = import ./modules/interfaces.nix;
      wireguard   = import ./modules/wireguard.nix;
      nftables    = import ./modules/nftables.nix;
      pbr         = import ./modules/pbr.nix;
      dns         = import ./modules/dns.nix;
      dhcp        = import ./modules/dhcp.nix;
      dashboard   = import ./modules/dashboard;
      qos         = import ./modules/qos.nix;
      upnp        = import ./modules/upnp.nix;
      performance = import ./modules/performance.nix;
      services    = import ./modules/services.nix;
    };
```

Notice that `default` (which aggregates all modules) is NOT touched. Users who import `nixosModules.default` do not get the dashboard automatically; they need to explicitly add `nixosModules.dashboard` to their module list.

- [ ] **Step 3: Verify the flake evaluates**

```bash
cd /Users/giko/Documents/nixos-rpi4-router
nix --extra-experimental-features 'nix-command flakes' flake check --no-build
```

Expected: success, no warnings about missing options. The bootstrap `version.json` makes the dashboard module a no-op under any configuration, so `flake check` passes.

- [ ] **Step 4: Verify `nix flake show` lists it**

```bash
cd /Users/giko/Documents/nixos-rpi4-router
nix --extra-experimental-features 'nix-command flakes' flake show 2>&1 | grep dashboard
```

Expected: one line referencing `nixosModules.dashboard`.

---

### Task 13: Commit checkpoint 2 — Nix module wiring

**Goal:** Land Tasks 9-12 as one commit in the public flake.

- [ ] **Step 1: Verify state**

```bash
cd /Users/giko/Documents/nixos-rpi4-router
git status --short
```

Expected:

```
M  flake.nix
A  modules/dashboard/default.nix
A  modules/dashboard/package.nix
A  modules/dashboard/version.json
```

- [ ] **Step 2: Commit**

```bash
git add flake.nix modules/dashboard/default.nix modules/dashboard/package.nix modules/dashboard/version.json
git commit -m "dashboard: add nixos module, package.nix, and bootstrap version.json"
```

---

### Task 14: `.github/workflows/nix-evaluation-check.yml`

**Goal:** A lightweight companion CI workflow that runs `nix flake check --no-build` on every Nix-file or `flake.lock` change. Catches breakage in Nix wiring even when no binary-source files changed.

**Files:**
- Create: `/Users/giko/Documents/nixos-rpi4-router/.github/workflows/nix-evaluation-check.yml`

- [ ] **Step 1: Create the workflow file**

```yaml
name: nix-evaluation-check

on:
  push:
    branches: [main]
    paths:
      - '**/*.nix'
      - 'flake.lock'
      - 'modules/dashboard/version.json'
  pull_request:
    paths:
      - '**/*.nix'
      - 'flake.lock'
      - 'modules/dashboard/version.json'

permissions:
  contents: read

jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - uses: cachix/install-nix-action@v27
        with:
          extra_nix_config: |
            experimental-features = nix-command flakes

      - name: Evaluate flake
        run: nix flake check --no-build
```

`--no-build` means the job evaluates option types, runs assertions, and renders the module graph — but does not attempt to fetch or build any derivations. That's enough to catch broken option types, missing references, and assertion failures.

---

### Task 15: `.github/workflows/build-dashboard.yml`

**Goal:** The main CI workflow: build binary, upload release, push version.json with rebase+retry, verify.

**Files:**
- Create: `/Users/giko/Documents/nixos-rpi4-router/.github/workflows/build-dashboard.yml`

- [ ] **Step 1: Create the workflow**

```yaml
name: build-dashboard

on:
  push:
    branches: [main]
    paths:
      - 'modules/dashboard/frontend/**'
      - 'modules/dashboard/backend/**'
      - '.github/workflows/build-dashboard.yml'
  workflow_dispatch:

concurrency:
  group: build-dashboard
  cancel-in-progress: false

permissions:
  contents: write

jobs:
  build:
    runs-on: ubuntu-latest

    steps:
      - name: Checkout (full history)
        uses: actions/checkout@v4
        with:
          fetch-depth: 0
          token: ${{ secrets.GITHUB_TOKEN }}

      - name: Set up Node.js
        uses: actions/setup-node@v4
        with:
          node-version: '20'
          cache: npm
          cache-dependency-path: modules/dashboard/frontend/package-lock.json

      - name: Build frontend
        working-directory: modules/dashboard/frontend
        run: |
          npm ci
          npm run build

      - name: Copy frontend dist into backend embed directory
        run: |
          rm -rf modules/dashboard/backend/internal/spa/dist
          mkdir -p modules/dashboard/backend/internal/spa/dist
          cp -r modules/dashboard/frontend/dist/* modules/dashboard/backend/internal/spa/dist/

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.22'
          cache-dependency-path: modules/dashboard/backend/go.sum

      - name: Cross-build dashboard binary for aarch64-linux
        working-directory: modules/dashboard/backend
        env:
          GOOS: linux
          GOARCH: arm64
          CGO_ENABLED: '0'
        run: |
          go build -ldflags='-s -w' -o dashboard ./cmd/dashboard

      - name: Compute version and hash
        id: meta
        run: |
          VERSION="$(date -u +%Y%m%d)-${GITHUB_SHA::7}"
          cd modules/dashboard/backend
          HASH="sha256-$(openssl dgst -sha256 -binary dashboard | base64)"
          echo "version=$VERSION" >> "$GITHUB_OUTPUT"
          echo "hash=$HASH" >> "$GITHUB_OUTPUT"
          echo "tag=dashboard-$VERSION" >> "$GITHUB_OUTPUT"

      - name: Upload release asset
        id: release
        uses: softprops/action-gh-release@v2
        with:
          tag_name: ${{ steps.meta.outputs.tag }}
          name: Dashboard ${{ steps.meta.outputs.version }}
          files: modules/dashboard/backend/dashboard
          fail_on_unmatched_files: true

      - name: Resolve release asset URL
        id: asset
        run: |
          URL="https://github.com/${{ github.repository }}/releases/download/${{ steps.meta.outputs.tag }}/dashboard"
          echo "url=$URL" >> "$GITHUB_OUTPUT"

      - name: Commit version.json with rebase+retry
        env:
          VERSION: ${{ steps.meta.outputs.version }}
          URL: ${{ steps.asset.outputs.url }}
          HASH: ${{ steps.meta.outputs.hash }}
        run: |
          set -eu
          git config user.name 'github-actions[bot]'
          git config user.email 'github-actions[bot]@users.noreply.github.com'
          for attempt in 1 2 3; do
            git fetch origin main
            git reset --hard origin/main
            cat > modules/dashboard/version.json <<EOF
          {
            "version": "$VERSION",
            "url": "$URL",
            "hash": "$HASH"
          }
          EOF
            git add modules/dashboard/version.json
            git commit -m "dashboard: release $VERSION"
            if git push origin main; then
              echo "Pushed on attempt $attempt"
              exit 0
            fi
            echo "Push attempt $attempt failed; retrying..."
            sleep 2
          done
          echo "::error::version.json push failed after 3 attempts" >&2
          exit 1

      - name: Verify published asset matches committed hash
        env:
          URL: ${{ steps.asset.outputs.url }}
          EXPECTED: ${{ steps.meta.outputs.hash }}
        run: |
          set -eu
          curl -fsSL -o /tmp/dashboard.verify "$URL"
          ACTUAL="sha256-$(openssl dgst -sha256 -binary /tmp/dashboard.verify | base64)"
          if [ "$ACTUAL" != "$EXPECTED" ]; then
            echo "::error::verify mismatch" >&2
            echo "expected: $EXPECTED" >&2
            echo "actual:   $ACTUAL"   >&2
            exit 1
          fi
          echo "verify: hash matches expected ($EXPECTED)"
```

---

### Task 16: Commit checkpoint 3 — CI workflows

**Goal:** Land both workflow files as one commit.

- [ ] **Step 1: Verify state**

```bash
cd /Users/giko/Documents/nixos-rpi4-router
git status --short
```

Expected:

```
A  .github/workflows/build-dashboard.yml
A  .github/workflows/nix-evaluation-check.yml
```

- [ ] **Step 2: Commit**

```bash
git add .github/workflows/build-dashboard.yml .github/workflows/nix-evaluation-check.yml
git commit -m "dashboard: add CI workflows for build + release + nix flake check"
```

---

### Task 17: Push public flake to GitHub

**Goal:** Trigger `nix-evaluation-check.yml` on the existing commits and confirm the flake evaluates cleanly in CI.

- [ ] **Step 1: Push**

```bash
cd /Users/giko/Documents/nixos-rpi4-router
git push origin main
```

- [ ] **Step 2: Watch the nix-evaluation-check run**

```bash
gh run watch -R giko/nixos-rpi4-router
```

Expected: `nix-evaluation-check` runs and completes successfully. `build-dashboard` does NOT run on this push (no source-level files changed — the push only touches `.github/workflows/` and `modules/dashboard/{default.nix,package.nix,version.json}`, none of which match `build-dashboard.yml`'s trigger paths).

Actually wait — the push also includes `modules/dashboard/frontend/**` and `modules/dashboard/backend/**` from Task 8. The three commits are being pushed together. `build-dashboard` will trigger on the scaffolding commit. That's expected and fine — the first real release will be produced automatically.

If `build-dashboard` triggers on this push, it'll build against the bootstrap `version.json`, publish a release, and auto-commit a real `version.json`. **That's exactly what we want for Task 18.**

- [ ] **Step 3: If build-dashboard triggered automatically, skip to Task 18**

Otherwise, proceed to Task 18 to trigger it manually via `workflow_dispatch`.

---

### Task 18: Trigger the first dashboard release (workflow_dispatch)

**Goal:** Produce the first real release + the first non-bootstrap `version.json` auto-commit.

**Context:** If Task 17's push already triggered `build-dashboard` (because it touched `modules/dashboard/{frontend,backend}/**`), this task is effectively "wait for and verify the already-running workflow." If not, manually dispatch it.

- [ ] **Step 1: Check if build-dashboard ran after Task 17's push**

```bash
gh run list -R giko/nixos-rpi4-router --workflow build-dashboard.yml --limit 3
```

Expected one of:
- **(a) A run is in-progress or just completed.** Skip to Step 3.
- **(b) No run exists for main.** Proceed to Step 2.

- [ ] **Step 2: Manually trigger via workflow_dispatch**

```bash
gh workflow run build-dashboard.yml -R giko/nixos-rpi4-router --ref main
gh run watch -R giko/nixos-rpi4-router
```

- [ ] **Step 3: Verify the run succeeded**

```bash
gh run list -R giko/nixos-rpi4-router --workflow build-dashboard.yml --limit 1
```

Expected: a single `completed / success` row. If it's `failure`, open the run log (`gh run view <id> --log-failed -R giko/nixos-rpi4-router`) and fix the underlying issue before proceeding.

- [ ] **Step 4: Verify the release was published**

```bash
gh release list -R giko/nixos-rpi4-router | grep dashboard-
```

Expected: at least one `dashboard-<version>` release, with one asset named `dashboard`.

- [ ] **Step 5: Verify `version.json` was auto-committed**

```bash
cd /Users/giko/Documents/nixos-rpi4-router
git pull origin main
cat modules/dashboard/version.json
```

Expected JSON with `version != "bootstrap"`, a real URL, and a real SHA256 hash.

- [ ] **Step 6: Verify the binary fetched from the release URL works**

```bash
URL="$(python3 -c 'import json; print(json.load(open("/Users/giko/Documents/nixos-rpi4-router/modules/dashboard/version.json"))["url"])')"
curl -fsSL "$URL" -o /tmp/dashboard-verify
file /tmp/dashboard-verify
# Expected: ELF 64-bit LSB executable, ARM aarch64, statically linked
rm /tmp/dashboard-verify
```

---

### Task 19: Enable dashboard in local `configuration.nix`

**Goal:** Turn the dashboard on in the actual deployment. Populate `allowedSources` with admin IPs.

**Files:**
- Modify: `/Users/giko/Documents/router/configuration.nix`

**Prerequisites:**
- `/Users/giko/Documents/router/flake.nix` already imports `nixosModules.default` from the public flake. This task also needs to import `nixosModules.dashboard` because the default module does not include it.
- The admin IPs you want in `allowedSources` should already have static DHCP leases in `dhcp.staticLeases`. In the current config, `192.168.1.117` (LAPTOP) and `192.168.1.119` (pi5) both qualify. Desktop (`giko-pc`, MAC `44:fa:66:65:86:43`) is currently dynamic — bump it to static before relying on it.

- [ ] **Step 1: Read the local flake.nix to confirm how modules are imported**

```bash
cat /Users/giko/Documents/router/flake.nix
```

Identify the `modules = [ ... ];` list in the `nixosConfigurations.router` definition. It likely contains `rpi4-router.nixosModules.default`.

- [ ] **Step 2: Add the dashboard module to the modules list**

Edit `/Users/giko/Documents/router/flake.nix`:

```nix
modules = [
  rpi4-router.nixosModules.default
  rpi4-router.nixosModules.dashboard   # <-- add this line
  ./configuration.nix
  ./hardware-configuration.nix
];
```

(Adjust to match the exact structure of the file.)

- [ ] **Step 3: Add dashboard config to `configuration.nix`**

Insert a new top-level block inside `router = { ... };`:

```nix
    dashboard = {
      enable = true;
      allowedSources = [
        "192.168.1.117"   # LAPTOP (static lease)
        "192.168.1.119"   # pi5 (static lease)
      ];
    };
```

Place it alphabetically near `dhcp` / `dns` / existing subsystems.

- [ ] **Step 4: If a desktop IP is needed, first add a static lease**

If `giko-pc` (or another admin device) should also be in the allowlist, add it to `dhcp.staticLeases` first — the current dynamic IP is not stable:

```nix
    dhcp.staticLeases = [
      # ... existing leases ...
      { mac = "44:fa:66:65:86:43"; name = "giko-pc"; ip = "192.168.1.150"; }
    ];
```

Then add `"192.168.1.150"` to `router.dashboard.allowedSources`.

(Skip this step if the laptop + pi5 admin access is sufficient for Phase 1 — the list can grow later.)

- [ ] **Step 5: Update `flake.lock` if needed**

If the `rpi4-router` input is pinned to an older commit, refresh it to pick up the new `nixosModules.dashboard`:

```bash
cd /Users/giko/Documents/router
nix --extra-experimental-features 'nix-command flakes' flake lock --update-input rpi4-router 2>/dev/null || true
```

Or equivalent `nix flake update rpi4-router`. The deploy workflow (in the project's CLAUDE.md) says to update the lock on the Pi then pull back — follow that pattern in Task 20.

---

### Task 20: Deploy to router

**Goal:** rsync the local repo to the Pi, refresh the flake lock, rebuild. Follow the project's documented deploy procedure from `CLAUDE.md`.

- [ ] **Step 1: rsync local repo → Pi**

```bash
cd /Users/giko/Documents/router
rsync -av --no-o --no-g --delete ./ root@192.168.1.1:/etc/nixos/
```

The `--no-o --no-g` flags are critical — without them `rsync` preserves the Mac's UID 501, which trips libgit2's repo-owner safety check when Nix opens `/etc/nixos` as a git tree.

- [ ] **Step 2: Update the `rpi4-router` input on the Pi**

```bash
ssh root@192.168.1.1 "cd /etc/nixos && nix flake update rpi4-router"
```

This refreshes `flake.lock` so it points at the latest public-flake commit, which includes the dashboard module AND the CI-auto-committed `version.json`.

- [ ] **Step 3: Rebuild**

```bash
ssh root@192.168.1.1 "nixos-rebuild switch --flake /etc/nixos#router"
```

Expected: a clean rebuild. The eval-time assertion on `allowedSources != []` should pass (we declared them in Task 19). The `router-dashboard` systemd unit is created and started.

If the rebuild fails with `assertion failed` messages, re-check Task 19 — the most likely cause is an empty `allowedSources` list or a bootstrap `version.json` that didn't get pulled.

- [ ] **Step 4: Pull the updated `flake.lock` back to the Mac**

```bash
rsync -av root@192.168.1.1:/etc/nixos/flake.lock /Users/giko/Documents/router/flake.lock
```

---

### Task 21: Verify end-to-end from an allowed client

**Goal:** Confirm the dashboard is reachable from an admin IP and NOT reachable from a non-admin IP.

- [ ] **Step 1: Verify the service is running on the router**

```bash
ssh root@192.168.1.1 "systemctl is-active router-dashboard"
ssh root@192.168.1.1 "systemctl status router-dashboard --no-pager | head -20"
```

Expected: `active`. No recent errors in the status output.

- [ ] **Step 2: Check the nftables rules landed**

```bash
ssh root@192.168.1.1 "nft list chain inet filter input" | grep -A1 -B1 dport
```

Expected: two rules matching the dashboard port (`tcp dport 9090 ip saddr {...} accept` and `tcp dport 9090 drop`).

- [ ] **Step 3: Probe from an allowed client**

From the LAPTOP (or whatever is in `allowedSources`):

```bash
curl -sS http://192.168.1.1:9090/api/health
```

Expected:

```json
{"ok":true,"version":"<something>","started_at":"2026-04-11T..."}
```

Also open `http://192.168.1.1:9090/` in a browser — you should see the "SENTINEL OS / Router Dashboard / status: OK" card with the real version + started_at from the live service.

- [ ] **Step 4: Probe from a non-admin client (negative test)**

From an iPhone, Sonos-attached phone, or any LAN device NOT in `allowedSources`:

```bash
# From some device on 192.168.1.0/24 that isn't in allowedSources
curl -sS --connect-timeout 5 http://192.168.1.1:9090/api/health
```

Expected: connection times out (not "connection refused" — nftables drops the packets rather than rejecting).

If the non-admin probe unexpectedly succeeds, the firewall gate is broken. Re-check `extraInputRules` in the rendered nft ruleset (`nft list chain inet filter input`) and verify both the `accept` and `drop` rules made it in.

- [ ] **Step 5: Commit the local repo changes**

```bash
cd /Users/giko/Documents/router
git add configuration.nix flake.lock
git commit -m "enable router dashboard module with LAN admin allowlist"
git push origin main
```

---

### Task 22: Smoke-test the deploy loop one more time

**Goal:** Prove that the full deploy pipeline works for *any* change, not just the bootstrap. Push a trivial frontend change, let CI rebuild, pull, deploy, verify.

**Files:**
- Modify: `/Users/giko/Documents/nixos-rpi4-router/modules/dashboard/frontend/src/App.tsx`

- [ ] **Step 1: Make a one-character change**

Edit `src/App.tsx` — change the subtitle text or bump a version:

```diff
-        <p className="text-2xl font-semibold mb-4">Router Dashboard</p>
+        <p className="text-2xl font-semibold mb-4">Router Dashboard v0.1</p>
```

- [ ] **Step 2: Commit + push to public flake**

```bash
cd /Users/giko/Documents/nixos-rpi4-router
git add modules/dashboard/frontend/src/App.tsx
git commit -m "dashboard: smoke-test the deploy pipeline"
git push origin main
```

- [ ] **Step 3: Watch CI**

```bash
gh run watch -R giko/nixos-rpi4-router
```

Expected: `build-dashboard` runs, a new release is published, `version.json` is auto-committed.

- [ ] **Step 4: Pull + deploy**

```bash
cd /Users/giko/Documents/router
rsync -av --no-o --no-g --delete ./ root@192.168.1.1:/etc/nixos/
ssh root@192.168.1.1 "cd /etc/nixos && nix flake update rpi4-router && nixos-rebuild switch --flake /etc/nixos#router"
rsync -av root@192.168.1.1:/etc/nixos/flake.lock ./flake.lock
```

- [ ] **Step 5: Verify the change landed**

Reload the browser tab showing the dashboard. Expect the subtitle to say `Router Dashboard v0.1`.

- [ ] **Step 6: Commit the updated `flake.lock` on the Mac**

```bash
cd /Users/giko/Documents/router
git add flake.lock
git commit -m "flake.lock: pick up dashboard smoke-test release"
git push origin main
```

**Plan 1 is complete** when this task succeeds. The router serves a live dashboard at `http://192.168.1.1:9090/` accessible only from allowlisted admin IPs, and every subsequent source change will flow through the same automated pipeline: commit → CI build → release → auto-bump → deploy.

---

## Self-review (fill in before handoff)

Before executing, the engineer should confirm:

- [ ] **Spec coverage:** Does Plan 1 implement every v1-Foundation item? Cross-check against spec §3 (architecture), §4 (repo layout), §5 (distribution), §6 (NixOS module), §10.3 (release).
- [ ] **No placeholders:** Search this file for `TBD`, `TODO`, `fill in`, `similar to`, `add appropriate`. Should be zero matches.
- [ ] **Paths match:** Every `cd` command and every file path is absolute or clearly anchored. Two different git repos are involved — confirm each step targets the right one.
- [ ] **Tests run locally before commit:** `go test ./...` in `backend/`, `npm run build` in `frontend/`. CI will re-run them, but local green is the commit gate.

---

## Known follow-ups (intentionally deferred to Plan 2)

- The `internal/spa` package is Phase 1's simplest "serve embedded files" implementation. Plan 2 adds cache headers, `ETag`, and maybe an asset-integrity check.
- The Go server currently has zero middleware (no logging, no rate limiting, no panic recovery). Plan 2 introduces the full middleware chain from spec §7.1 and §7.6.
- The frontend has no real pages, no router, no design-system primitives beyond a single ad-hoc card. Plan 2 rebuilds `App.tsx` with React Router, shadcn primitives, the full layout, and wires in the real data hooks.
- Nothing in Plan 1 collects any data. Plan 2 is where the 13 collectors, the `State` cache, and the 13 API endpoints land.

These are NOT things to sneak in during Plan 1 execution. Keep Plan 1 tight — the value of this phase is *proving the pipeline works*, not hitting feature parity.

---

## Execution handoff

Plan complete. When you're ready to execute:

**1. Subagent-Driven (recommended)** — I dispatch a fresh Opus subagent per task, review between tasks, fast iteration. Each subagent gets this plan file path + the spec file path + the specific task number, nothing else. Good isolation, no cross-task context bleed.

**2. Inline Execution** — Execute tasks in this session using `superpowers:executing-plans`, batch execution with checkpoints for review.

Which approach?
