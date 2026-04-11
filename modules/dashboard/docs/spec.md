# Router Dashboard — Design Spec

> **Status:** Draft, pending review
> **Date:** 2026-04-11
> **Updated:** 2026-04-11 — post Codex adversarial review (see §14.1 for resolution summary)
> **Target repo:** `github.com/giko/nixos-rpi4-router` (public flake)
> **Deploy target:** Raspberry Pi 4, `aarch64-linux`, NixOS 25.11+
> **Supersedes:** nothing — greenfield

## 1. Overview

### 1.1 What this is

A single-page web dashboard for a NixOS-based Raspberry Pi 4 home router. It exposes live, read-only observability into every subsystem the router runs: WireGuard VPN tunnels, per-source VPN pools with round-robin load balancing, DHCP clients, AdGuard Home DNS, network interface traffic, firewall / PBR rules, QoS shaping, UPnP, and host system health.

The dashboard is implemented as a Go HTTP service that serves both a JSON API and an embedded React SPA bundle (shipped as one binary via `//go:embed`). It runs as a systemd service on the router itself. The public flake (`github.com/giko/nixos-rpi4-router`) grows a new optional module (`router.dashboard`) that pulls the pre-built binary from a GitHub Release and wires it into `systemd.services.router-dashboard`.

### 1.2 Goals

- **Answer "is anything broken?" in <2 seconds.** Home screen surfaces tunnel health, service status, critical alerts, and WAN throughput at a glance.
- **Answer "what is each device doing?"** Clients view shows every device, its current route (WAN or VPN tunnel), recent DNS activity, and live throughput.
- **Answer "is privacy working?"** AdGuard view shows block rate, top blocked domains, per-client query logs.
- **Cover all eight subsystems** of the router (scope: everything unified — see §12).
- **Read-only in v1**, but never paint the design into a corner that mutations in v2+ can't escape (see §13).
- **Zero router-side build cost.** The Pi downloads a pre-built binary; it never touches Node, Go, or Vite.
- **Reproducible via the flake.** Every deploy points at a specific release asset pinned by URL + SHA256.

### 1.3 Non-goals

- Authentication, user management, RBAC
- Multi-tenant / multi-household support
- Mobile-first UX (mobile *friendly*, desktop is primary)
- Remote access (LAN-only; use the existing WireGuard if you want it elsewhere)
- Configuration editing (router config is managed via the flake, not the dashboard)
- Historical time-series storage (a future Prometheus export is an explicit v2+ possibility)
- Internationalization (English only)
- Branding beyond the "SENTINEL OS" visual identity already delivered by the designer

### 1.4 Success criteria

- Opening the dashboard on a laptop on the home LAN loads the overview in under 500 ms (binary is resident, static assets are embedded).
- Every subsystem listed in §12 has a working endpoint and a working page.
- A `nixos-rebuild switch` after a dashboard release completes in <10 s of dashboard-specific work (fetch + install + restart).
- An external user who adopts the public flake can enable the dashboard with a single config line: `router.dashboard.enable = true;`.

## 2. Users & usage

### 2.1 Audience

- **Primary:** The router owner (giko). Technical, comfortable in a terminal, knows what a fwmark is.
- **Secondary:** Nobody else in v1 — dashboard is not exposed to household members or external admins.

### 2.2 Access model

- Bound to the primary LAN address (`192.168.1.1`) on a configurable port (default `9090`).
- **Source-IP allowlist enforced at the firewall, not in the application.** The dashboard module declares a `router.dashboard.allowedSources` list (IPs or CIDR ranges) and injects nftables rules via `router.nftables.extraInputRules` that accept the dashboard port only from those sources and drop everything else on that port. Traffic from non-admin LAN clients is dropped in-kernel before any Go handler runs. The list must be non-empty for the module to activate (enforced by an eval-time assertion) — this prevents accidentally shipping a wide-open dashboard.
- **Why firewall-level and not app auth?** The data this dashboard aggregates (per-client DNS query history, MAC/IP mapping, routing state, firewall rules, port forwards) is a high-value target for a compromised IoT device on the LAN. Treating "LAN-trusted" as a blanket policy would silently leak browsing metadata and internal topology to any local attacker. A firewall-level allowlist puts the trust boundary one layer before HTTP, is zero-UX for the sole owner, is declarative in Nix, is enforced in-kernel, and reuses the same pattern the router already applies via `router.nftables.allowedMacs`. Application-level basic auth is tracked as v2+ defense-in-depth (see §13).
- **Which devices?** Only admin hosts the owner actually uses to view the dashboard — typically a desktop and a laptop. Admin devices SHOULD have static DHCP leases (via `router.dhcp.staticLeases`) so their IPs are stable; otherwise a lease rotation silently locks you out until you re-`nixos-rebuild` with the new IP. Clients on `192.168.20.0/24` (the server subnet) can also be listed if needed; by default none are.
- No TLS. Plain HTTP. Justified by firewall-gated access + local network scope. WAN-originated traffic is already dropped by the existing `input` chain policy.
- A second eval-time assertion verifies that the configured port does not collide with a declared `router.portForwards` entry.

## 3. Architecture

### 3.1 System shape

```
           ┌────────────────────────────────────────────────────┐
           │                   Raspberry Pi 4                   │
           │                                                     │
  LAN ──▶  │  dashboard (Go)   ──reads──▶  /proc/net/dev         │
           │    ├─ collector              /run/wg-pool-health    │
           │    ├─ http API               /var/lib/dnsmasq       │
           │    └─ embedded SPA           exec: nft, wg, ip,     │
           │                                    tc, conntrack   │
           │                              http://127.0.0.1:3000  │
           │                                  (AdGuard)          │
           └────────────────────────────────────────────────────┘
                          (single static binary,
                           ~15 MB, embeds frontend)
```

The dashboard is **one process, one binary, one systemd unit**. No sidecar, no proxy, no scheduled collectors. All collection happens in goroutines inside the dashboard process.

### 3.2 Component responsibilities

- **Go HTTP server** — serves `/api/*` (JSON) and `/*` (embedded SPA assets, SPA fallback for client-side routing).
- **Collector** — a set of goroutines, one per data-source-tier, that read state files / exec commands / call the AdGuard API on fixed cadences and cache parsed results in a central `State` struct guarded by a `sync.RWMutex`. API handlers only touch the cache — they never block on I/O.
- **Embedded frontend** — Vite build output (`dist/`) copied into `backend/internal/static/` before compile and served via `//go:embed static/*`. One binary to deploy.
- **NixOS module** (`modules/dashboard/default.nix`) — declares options, renders the systemd service, wires the dashboard package from `version.json`.
- **`version.json`** (`modules/dashboard/version.json`) — source of truth for which release the flake deploys. Bumped automatically by CI after each build.

### 3.3 Data flow

```
   Collector tick (every N sec, per tier)
        │
        ▼
   read/exec/http   ──▶  parse  ──▶  State (RWMutex)
                                         ▲
                                         │ RLock
                               API handler responds
                                         ▲
                                         │ JSON
                               TanStack Query (2-5s poll)
                                         ▲
                                         │
                                  React component re-render
```

All network effects from `nft`, `wg`, `ip`, `conntrack`, `tc`, AdGuard, and `/proc` are pulled into the dashboard's own process. The frontend never talks to anything except `/api/*` on the dashboard's own port.

## 4. Repository layout

Added under `nixos-rpi4-router/modules/dashboard/`:

```
modules/dashboard/
├── default.nix             # NixOS module (options + systemd unit)
├── package.nix             # Nix derivation that fetches the release binary
├── version.json            # {version, url, hash} — updated by CI
├── designs/                # Designer output (already exists)
│   ├── 01-overview.{html,png}
│   ├── 02-clients.{html,png}
│   ├── 03-vpn-pools.{html,png}
│   ├── 04-adguard.{html,png}
│   ├── design-system.md
│   └── README.md
├── docs/
│   ├── design-brief.md     # The brief given to the designer
│   └── spec.md             # This document
├── backend/
│   ├── go.mod
│   ├── go.sum
│   ├── cmd/dashboard/main.go
│   └── internal/
│       ├── server/         # http handlers, routing
│       ├── collector/      # goroutines + state cache
│       ├── sources/        # one file per data source (wg, nft, adguard, ...)
│       ├── model/          # shared struct types
│       └── static/         # frontend dist, populated by CI before `go build`
└── frontend/
    ├── package.json
    ├── package-lock.json
    ├── vite.config.ts
    ├── tsconfig.json
    ├── tailwind.config.ts
    ├── index.html
    └── src/
        ├── main.tsx
        ├── App.tsx
        ├── lib/            # query-client setup, fetch helpers, utilities
        ├── components/     # shared components (cards, tables, badges, layout)
        ├── ui/             # shadcn components (generated via CLI)
        ├── pages/          # one file per route
        └── styles.css
```

Existing router modules (`nftables.nix`, `pbr.nix`, etc.) stay put. The new module is opt-in via `router.dashboard.enable = true;` and has no effect unless explicitly enabled.

The top-level `flake.nix` exposes a new `nixosModule.dashboard` and (optionally) a new `packages.aarch64-linux.router-dashboard`. The `default` module (aggregated under `modules/default.nix`) **does not** import `dashboard` automatically — users opt in explicitly, so existing consumers of the flake see no behavior change.

## 5. Distribution model

### 5.1 The release artifact

A single statically-linked Go binary for `aarch64-linux`, named `dashboard`, produced by `GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags='-s -w'`. The Vite build output is embedded via `//go:embed internal/static/*`. Uncompressed size: ~15 MB. No runtime dependencies beyond a modern Linux kernel and the commands it execs (`nft`, `wg`, etc. — all already installed by the router modules).

The binary is uploaded as an asset on a GitHub Release in the same repo, tagged `dashboard-<version>` where `<version>` is `YYYYMMDD-<shortsha>`.

### 5.2 CI pipeline (GitHub Actions)

Two workflows live under `.github/workflows/`:

1. **`build-dashboard.yml`** — builds the binary and publishes a release. Triggered only on source-level changes.
2. **`nix-evaluation-check.yml`** — runs `nix flake check` on every Nix-file or `flake.lock` change. Zero binary build; validates that the flake still evaluates, `default.nix`/`package.nix` still parse, and assertions still pass. Catches module-level breakage that the binary-build workflow wouldn't trigger on.

#### 5.2.1 `build-dashboard.yml`

**Trigger:**

```yaml
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
```

Critically, `modules/dashboard/version.json` is *not* in the path list — so the bump commit that CI makes after a successful build doesn't trigger a rebuild loop. Other `modules/dashboard/*.nix` files are *also* not in the list, because the binary doesn't need to rebuild when only Nix wiring changes — `nix-evaluation-check.yml` covers those.

`concurrency.cancel-in-progress: false` serializes dashboard builds across the repo: only one `build-dashboard` run is active at a time, and subsequent runs queue. This closes the race where two close-together pushes can produce two releases whose `version.json` pushes interleave and leave the repo pointing at the older artifact. **Do not** set `cancel-in-progress: true` here — a cancelled in-flight push can leave a partial commit in a corrupt state.

**Steps (high-level):**

1. `actions/checkout@v4` with `fetch-depth: 0` (full history, needed for rebase in step 10).
2. `actions/setup-node@v4` with `node-version: '20'` and npm cache keyed on `frontend/package-lock.json`.
3. `npm ci && npm run build` in `frontend/` → produces `frontend/dist/`.
4. `cp -r frontend/dist/* backend/internal/static/`.
5. `actions/setup-go@v5` with `go-version: '1.22'`.
6. `GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags='-s -w' -o dashboard ./cmd/dashboard` in `backend/`.
7. Compute `VERSION="$(date -u +%Y%m%d)-${GITHUB_SHA::7}"`.
8. Compute `HASH="sha256-$(openssl dgst -sha256 -binary dashboard | base64)"`.
9. **Upload release asset** with `softprops/action-gh-release@v2` — tag `dashboard-${VERSION}`, asset is `backend/dashboard`. Uses `fail_on_unmatched_files: true` so a silent upload failure errors out.
10. **Push `version.json` with rebase+retry.** This is the most failure-prone step — it races with any other push to `main`. Wrap in a retry loop:
    ```bash
    write_version_json() {
      cat > modules/dashboard/version.json <<EOF
    { "version": "${VERSION}", "url": "${URL}", "hash": "${HASH}" }
    EOF
    }
    for i in 1 2 3; do
      git fetch origin main
      git reset --hard origin/main
      write_version_json
      git add modules/dashboard/version.json
      git -c user.name='github-actions[bot]' \
          -c user.email='github-actions[bot]@users.noreply.github.com' \
          commit -m "dashboard: release ${VERSION}" --allow-empty=false
      if git push origin main; then
        break
      fi
      [ $i -lt 3 ] && sleep 2 || { echo "version.json push failed after 3 attempts" >&2; exit 1; }
    done
    ```
    The `git reset --hard origin/main` rewrites our local tree to whatever is currently at HEAD, then we reapply the version bump on top. This is safer than `rebase` for a one-file change and survives any intervening commit to `main`.
11. **Verify step.** After the push succeeds, fetch the published URL and sha256-check it against the committed hash:
    ```bash
    curl -fsSLo /tmp/dashboard.verify "${URL}"
    ACTUAL="sha256-$(openssl dgst -sha256 -binary /tmp/dashboard.verify | base64)"
    if [ "$ACTUAL" != "$HASH" ]; then
      echo "::error::verify mismatch — committed hash does not match released asset" >&2
      echo "expected: $HASH" >&2
      echo "got:      $ACTUAL" >&2
      exit 2
    fi
    ```
    A non-zero exit on verify fails the workflow loudly and turns up as a red badge in the GH Actions UI, which is the signal for manual recovery.

**Partial-failure recovery.** If step 10 fails after 3 attempts, the release asset exists but `version.json` is stale. If step 11 fails, both exist but are inconsistent. Both cases are diagnosable from the GH Actions logs and recovered by:
- **Option A:** Re-run the failed workflow (`gh run rerun <id>`). The build is deterministic, the same artifact rebuilds, and the retry loop tries the push again.
- **Option B:** Manually delete the orphaned release (`gh release delete dashboard-${VERSION} --yes --cleanup-tag`) and re-run the workflow.

The spec intentionally does not implement auto-rollback logic — both failure modes are rare, both are already visible in CI, and the wrong recovery (e.g., auto-deleting a release the user wanted to keep) is worse than a manual retry.

#### 5.2.2 `nix-evaluation-check.yml`

Lightweight companion that runs on every push touching Nix-level files:

```yaml
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

jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: cachix/install-nix-action@v27
        with:
          extra_nix_config: experimental-features = nix-command flakes
      - run: nix flake check --no-build
```

`--no-build` means this job does not attempt to fetch any release artifacts — it only evaluates the flake, runs assertions, and type-checks module options. This catches `default.nix`/`package.nix`/flake breakage even when no binary-source files changed, including the case where CI's own `version.json` bump produces an invalid JSON. It does not replace the build workflow; it runs alongside it.

### 5.3 `version.json` contract

```json
{
  "version": "20260411-abc1234",
  "url": "https://github.com/giko/nixos-rpi4-router/releases/download/dashboard-20260411-abc1234/dashboard",
  "hash": "sha256-kQw+x9Fv..."
}
```

The file lives at `modules/dashboard/version.json`. It is the single source of truth for which release the flake deploys. CI writes it; humans only touch it to seed the first bootstrap version (see §10.3).

### 5.4 Flake consumption

`modules/dashboard/package.nix`:

```nix
{ stdenvNoCC, fetchurl, lib }:
let
  v = builtins.fromJSON (builtins.readFile ./version.json);
in
stdenvNoCC.mkDerivation {
  pname = "router-dashboard";
  inherit (v) version;
  src = fetchurl { url = v.url; hash = v.hash; };
  dontUnpack = true;
  installPhase = ''
    install -Dm755 $src $out/bin/dashboard
  '';
  meta = {
    description = "Read-only web dashboard for the nixos-rpi4-router";
    platforms = [ "aarch64-linux" ];
  };
}
```

On `nixos-rebuild switch`, Nix reads `version.json`, computes the derivation hash (which depends on the `fetchurl` inputs), finds nothing in `/nix/store`, pulls ~15 MB from GitHub, verifies the SHA256, installs to `$out/bin/dashboard`. Subsequent rebuilds without a version bump are cache-hit noops.

## 6. NixOS module

### 6.1 Options

`modules/dashboard/default.nix` declares:

```nix
options.router.dashboard = {
  enable = mkEnableOption "the router dashboard";

  allowedSources = mkOption {
    type = types.listOf types.str;
    default = [];
    example = [ "192.168.1.154" "192.168.1.117" "192.168.1.119" ];
    description = ''
      Source IPv4 addresses or CIDR ranges permitted to reach the dashboard
      port. Anything not listed here is dropped by an nftables rule generated
      in the router's input chain, before any HTTP handler runs.

      Must be non-empty for the module to activate (enforced by assertion).
      This is the primary trust boundary for the dashboard — pick only
      admin devices you actually use to view it, and give them static DHCP
      leases via `router.dhcp.staticLeases` so their IPs are stable.

      IPv6 is not currently supported (the router has IPv6 disabled end-to-end).
    '';
  };

  port = mkOption {
    type = types.port;
    default = 9090;
    description = "TCP port to bind the dashboard HTTP server on.";
  };

  bindAddress = mkOption {
    type = types.str;
    default = (builtins.head config.router.lan.addresses).address;
    description = "Address to bind. Defaults to the primary LAN address.";
  };

  adguardUrl = mkOption {
    type = types.str;
    default = "http://127.0.0.1:3000";
    description = "Base URL for the AdGuard Home REST API.";
  };

  package = mkOption {
    type = types.package;
    default = pkgs.callPackage ./package.nix { };
    description = "The dashboard package to run.";
  };

  logLevel = mkOption {
    type = types.enum [ "debug" "info" "warn" "error" ];
    default = "info";
  };
};
```

No other user-facing knobs in v1. If a knob becomes useful we can add it; it's easier than removing one later.

### 6.2 Config block — assertions, firewall gate, systemd unit

The module's top-level `config` is wrapped in `lib.mkIf (cfg.enable && v.version != "bootstrap")` — an enabled module with a bootstrap `version.json` is a silent no-op. Inside that block:

**Assertions** (both fire at eval time):

```nix
assertions = [
  {
    assertion = cfg.allowedSources != [];
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
```

**Firewall gate.** Dashboard access is not sandboxed inside the app — it's enforced in-kernel via `router.nftables.extraInputRules`, an existing option that appends raw lines to the `input` chain *before* the generic `iifname "eth0" accept`:

```nix
router.nftables.extraInputRules = let
  srcSet = lib.concatStringsSep ", " cfg.allowedSources;
  lanIf = config.router.lan.interface;
in ''
  iifname "${lanIf}" tcp dport ${toString cfg.port} ip saddr { ${srcSet} } accept
  iifname "${lanIf}" tcp dport ${toString cfg.port} drop
'';
```

Order-dependency: because these rules land before the generic LAN accept, dashboard-port packets from listed sources are accepted and everything else on that port is dropped in-kernel. Non-dashboard traffic from the same listed clients still falls through to the generic accept. The gate is scoped to one port only.

**Systemd unit:**

```nix
systemd.services.router-dashboard = {
  description = "Router observability dashboard";
  wantedBy = [ "multi-user.target" ];
  after = [
    "network-online.target"
    "nftables.service"
    "adguardhome.service"
    "wireguard.target"
  ];
  wants = [ "network-online.target" ];
  path = [ pkgs.nftables pkgs.wireguard-tools pkgs.iproute2 pkgs.conntrack-tools pkgs.iputils ];

  serviceConfig = {
    ExecStart = "${cfg.package}/bin/dashboard" +
                " --bind=${cfg.bindAddress}:${toString cfg.port}" +
                " --adguard-url=${cfg.adguardUrl}" +
                " --log-level=${cfg.logLevel}";
    Restart = "on-failure";
    RestartSec = 3;

    # Run as a transient dynamic user — but grant the caps we need.
    DynamicUser = true;
    AmbientCapabilities = [ "CAP_NET_ADMIN" "CAP_NET_RAW" ];
    CapabilityBoundingSet = [ "CAP_NET_ADMIN" "CAP_NET_RAW" ];

    # Sandbox.
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

    # The only paths the dashboard needs to read.
    ReadOnlyPaths = [
      "/run/wg-pool-health"
      "/var/lib/dnsmasq"
      "/sys/class/thermal"
      "/sys/devices/virtual/thermal"
    ];
  };
};
```

### 6.3 Security hardening rationale

- **DynamicUser + ambient caps:** the dashboard execs `nft`, `wg`, `ip`, `conntrack`, `tc` — all of which need `CAP_NET_ADMIN` (or `NET_RAW` for some tc queries). Giving these caps *ambiently* lets the process run as a transient non-root user while still having the minimum needed privileges. Cleaner than running as root with `ReadWriteOnly`.
- **`ProtectSystem=strict` + narrow `ReadOnlyPaths`:** the dashboard reads state files from well-known locations; everything else is inaccessible.
- **No mutation endpoints in v1** means we don't need `CAP_NET_BIND_SERVICE` (port 9090 is unprivileged).
- **No AdGuard auth token:** the AdGuard API is bound to `0.0.0.0:3000` on the router and unauthenticated. The dashboard's localhost request to it succeeds without credentials.

## 7. Backend design

### 7.1 HTTP server

Go 1.22+ `net/http` with the new `ServeMux` path-parameter syntax — no external router library. Single server, single mux:

```
GET  /api/health
GET  /api/system
GET  /api/tunnels
GET  /api/pools
GET  /api/clients
GET  /api/clients/{ip}
GET  /api/adguard/stats
GET  /api/adguard/querylog
GET  /api/traffic
GET  /api/firewall/rules
GET  /api/firewall/counters
GET  /api/qos
GET  /api/upnp
GET  /{...}       (SPA fallback — serves embedded index.html for anything else)
```

Middleware: request logging, panic recovery, gzip encoding, CORS off. All responses are `application/json; charset=utf-8` except the SPA fallback.

### 7.2 Data collection pattern

Three tiers of collectors run as goroutines; all write into a single `State` struct behind a `sync.RWMutex`:

| Tier | Cadence | Sources |
|---|---|---|
| **Hot** | 2 s | `/proc/net/dev`, `wg show dump`, `/run/wg-pool-health/state.json`, `/proc/stat`, `/proc/meminfo`, `/proc/uptime`, `/sys/class/thermal/thermal_zone0/temp` |
| **Medium** | 5 s | `systemctl is-active ...` (bulk), `nft list counter ...` (chain counters), `/var/lib/dnsmasq/dnsmasq.leases`, AdGuard `/control/stats` |
| **Cold** | 30 s | `conntrack -L` summary, `ip rule show`, `ip route show table all`, `tc -s qdisc show dev eth1`, `tc -s qdisc show dev ifb4eth1`, `cat /var/lib/miniupnpd/upnp.leases` |

**Principles:**
- Every collector has an independent timer (`time.Ticker`); they don't block each other.
- A collector failure is logged but doesn't crash the process. The previous good value stays in `State` with a "stale_since" timestamp; the API exposes this for UI staleness indicators.
- Handlers only do `state.RLock(); copy out; state.RUnlock()`. Per-request latency is <1 ms regardless of endpoint.
- **No handler ever triggers exec work or external I/O.** Per-client detail endpoints (`/api/clients/{ip}`) serve a filtered subset of the same cached state that `/api/clients` returns — they never run `conntrack -L`, never `exec` anything. Staleness is bounded by whichever collector tier populated the underlying field (30 s for conntrack-derived data; the response `stale` flag surfaces this to the UI). This is a hard rule: any request path that triggers exec work is an easy self-DoS vector under even modest polling frequencies, and the spec refuses to allow it.
- **AdGuard query log is the one exception** — `/api/adguard/querylog?client=X&limit=N` proxies to AdGuard's REST API on localhost, which is itself a cheap DB query (<50 ms, bounded). To prevent dogpiling when multiple browser tabs poll it, responses are cached in the dashboard for 3 s keyed by the full query string; concurrent requests share an in-flight result via `singleflight`.

**Expensive sources to watch:**
- `conntrack -L` on a router with ~1000 flows is ~500 ms of CPU. Keep it in the cold tier; never run it per-request for list endpoints.
- AdGuard `/control/querylog?limit=500` is ~50 ms. Frontend-driven (client filter, domain search) — fetched on demand via `/api/adguard/querylog`.

### 7.3 Data source map

| Endpoint field | Where it comes from |
|---|---|
| `system.cpu.*` | `/proc/stat` (diff between ticks) |
| `system.memory.*` | `/proc/meminfo` |
| `system.temperature_c` | `/sys/class/thermal/thermal_zone0/temp` / 1000 |
| `system.throttled` | `vcgencmd get_throttled` (exec) |
| `system.uptime_seconds` | `/proc/uptime` |
| `system.services[]` | `systemctl is-active` for a pinned list (nftables, adguardhome, wireguard-*, dnsmasq, wg-pool-health, flow-offload, cake-qos, chrony) |
| `tunnels[].latest_handshake_seconds_ago`, `rx_bytes`, `tx_bytes`, `endpoint` | `wg show <iface> dump` |
| `tunnels[].healthy`, `consecutive_failures` | `/run/wg-pool-health/state.json` |
| `tunnels[].exit_ip` | Cached result of `curl --interface wg_<name> https://ifconfig.co` from the cold tier (refreshed every 5 min, not 30 s — too expensive otherwise) |
| `tunnels[].routing_table`, `fwmark` | Static, from the module config (inferred at start) |
| `pools[].members[].flow_count` | `conntrack -L -m <fwmark>` counted (cold tier) |
| `pools[].failsafe_drop_active` | `nft list chain ip mangle prerouting` — presence of the drop rule |
| `clients[]` | Merge of: static leases (module config), dnsmasq leases file, conntrack flow origin IPs, `ip neigh show` for MAC-to-IP. Derived: which tunnel the client's new flows land on (from fwmark 0-mask match in conntrack) |
| `clients[].allowlist_status` | `router.nftables.allowedMacs` (from module config, cross-referenced with client MAC) |
| `adguard.*` | HTTP GET `http://127.0.0.1:3000/control/stats` + `/control/querylog` + `/control/clients` |
| `traffic.interfaces[]` | `/proc/net/dev` diffed over 2-second tick for rate; totals straight from the counter. Per-interface 60-sample sparkline ring buffer kept in memory |
| `firewall.port_forwards` | Module config (`router.portForwards`) |
| `firewall.pbr.*` | Module config (`router.pbr.*`) |
| `firewall.counters` | `nft --json list ruleset` parsed for `counter` statements |
| `qos.wan_egress.*` | `tc -s qdisc show dev eth1` parsed |
| `qos.wan_ingress.*` | `tc -s qdisc show dev ifb4eth1` parsed |
| `upnp.leases` | `/var/lib/miniupnpd/upnp.leases` or equivalent |

**NB:** the dashboard never writes anything. No state is persisted between restarts except the samples accumulated in memory (which are rebuilt on restart).

### 7.4 API contract

Full shapes are in `modules/dashboard/docs/api.md` (to be written as part of implementation — this spec sketches the top-level shapes only). Every endpoint returns:

```json
{
  "data": { ... },
  "updated_at": "2026-04-11T14:23:45.123Z",
  "stale": false
}
```

The `stale` flag is `true` if the collector for any subfield of `data` has failed to refresh within 2× its normal cadence. The frontend uses it to show a subtle "stale" badge on the affected card instead of failing loudly.

**Example endpoints (fields only, nested types omitted for brevity — full shapes in `api.md`):**

- `GET /api/health` → `{ok, version, started_at}`
- `GET /api/system` → `{cpu, memory, temperature_c, throttled, uptime_seconds, services}`
- `GET /api/tunnels` → `{tunnels: [{name, healthy, public_key, endpoint, latest_handshake_seconds_ago, rx_bytes, tx_bytes, exit_ip, routing_table, fwmark}]}`
- `GET /api/pools` → `{pools: [{name, members, client_ips, failsafe_drop_active}]}`
- `GET /api/clients` → `{clients: [{hostname, ip, mac, lease_type, last_seen, route, current_tunnel, allowlist_status, flow_count, dns_queries_1h}]}`
- `GET /api/clients/{ip}` → `{...base fields..., recent_queries, flows, blocked_queries_1h}`
- `GET /api/adguard/stats` → `{queries_24h, blocked_24h, block_rate, top_blocked, top_clients, query_density_24h}`
- `GET /api/adguard/querylog?limit=N&client=&domain=` → `{queries: [...]}`
- `GET /api/traffic` → `{interfaces: [{name, rx_bps, tx_bps, rx_bytes_total, tx_bytes_total, samples_60s}]}`
- `GET /api/firewall/rules` → `{port_forwards, pbr, allowed_macs, blocked_forward_count_1h}`
- `GET /api/firewall/counters` → `{chains: [...]}`
- `GET /api/qos` → `{wan_egress, wan_ingress}`
- `GET /api/upnp` → `{leases: [...]}`

### 7.5 Error handling

- **Collector failures:** logged at `warn`, cached value kept, `stale` flag set on the affected subset of state. Never 500s the API.
- **Unparseable command output:** logged at `error` with the raw stdout, skipped, stale flag set. Never panic.
- **Frontend API errors:** handler returns 503 with `{error, retry_after_seconds}` only for fundamentally broken state (e.g. `State` struct unreadable, which shouldn't happen).

### 7.6 Rate limiting

Defense-in-depth on top of the firewall allowlist. A middleware in the HTTP chain applies two independent token-bucket limits, both using `golang.org/x/time/rate`:

| Scope | Rate | Burst | Rationale |
|---|---|---|---|
| Per-source-IP | 20 req/s | 40 | A single runaway browser tab on an *allowed* client can't DoS the collector |
| Global | 200 req/s | 400 | Catches pathological cases (e.g. a client stuck in a reconnect loop) |

Exceeded requests return `HTTP 429 Too Many Requests` with `Retry-After: 1` and a small JSON error body; TanStack Query's built-in retry-with-backoff handles the client side transparently. `/api/health` is exempt so monitoring probes don't burn bucket tokens.

The primary gate is still the nftables allowlist from §2.2 — rate limiting exists to contain app-level bugs on *allowed* clients, not to stop unauthorized traffic (which never reaches the process at all).

### 7.7 Observability

- **Logging:** structured JSON (`slog`) to stdout. systemd captures it to the journal.
- **Metrics:** `/metrics` Prometheus endpoint behind an option (`router.dashboard.metrics.enable`, default `false`) — out of scope for v1 but the hook is wired.
- **Health endpoint:** `/api/health` returns 200 if the process is up and at least the hot collector has run once.

## 8. Frontend design

### 8.1 Stack

- **React 18** + TypeScript
- **Vite** build pipeline
- **Tailwind CSS** (already implied by shadcn/ui)
- **shadcn/ui** component library (copy-paste primitives via the CLI into `src/ui/`)
- **TanStack Query** v5 for data fetching (stale-while-revalidate, polling, retries)
- **Recharts** for line/bar/area charts (shadcn's default chart lib)
- **React Router** v6 for client-side routing

No state management library (Redux, Zustand, etc.) — TanStack Query covers server state, local component state is fine for UI state (toggles, filters, sort).

### 8.2 Routing

Client-side routes, all served by the SPA fallback in Go:

```
/                           Overview (home)
/vpn/tunnels                VPN tunnels list
/vpn/tunnels/:name          Single tunnel detail (v1.1)
/vpn/pools                  VPN pools list
/vpn/pools/:name            Single pool detail
/clients                    Clients list
/clients/:ip                Single client detail
/adguard                    AdGuard DNS stats + query log
/traffic                    Interface traffic graphs
/firewall                   Firewall / PBR rules + counters + UPnP leases
/qos                        QoS stats
/system                     System health
```

### 8.3 Page ↔ endpoint mapping

| Route | Query keys | Endpoint |
|---|---|---|
| `/` | `['system'], ['tunnels'], ['pools'], ['traffic'], ['adguard', 'stats']` | composite |
| `/vpn/tunnels` | `['tunnels']` | `/api/tunnels` |
| `/vpn/pools` | `['pools']` | `/api/pools` |
| `/vpn/pools/:name` | `['pools', name], ['clients']` | merged |
| `/clients` | `['clients']` | `/api/clients` |
| `/clients/:ip` | `['clients', ip]` | `/api/clients/{ip}` |
| `/adguard` | `['adguard', 'stats'], ['adguard', 'querylog', filters]` | `/api/adguard/stats` + `/api/adguard/querylog` |
| `/traffic` | `['traffic']` | `/api/traffic` |
| `/firewall` | `['firewall', 'rules'], ['firewall', 'counters'], ['upnp']` | `/api/firewall/rules` + `/api/firewall/counters` + `/api/upnp` |
| `/qos` | `['qos']` | `/api/qos` |
| `/system` | `['system']` | `/api/system` |

### 8.4 TanStack Query defaults

```ts
const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 2_000,            // UI considers data fresh for 2s
      gcTime: 5 * 60_000,
      refetchOnWindowFocus: false,
      retry: 2,
      retryDelay: (attempt) => Math.min(1000 * 2 ** attempt, 10_000),
    },
  },
});
```

Per-query `refetchInterval` is set from a small map keyed on endpoint:

| Endpoint | `refetchInterval` |
|---|---|
| `/api/system` | 2 s (CPU/mem/temp tick) |
| `/api/tunnels` | 5 s |
| `/api/pools` | 5 s |
| `/api/clients` | 5 s |
| `/api/clients/{ip}` | 3 s (while the detail page is open) |
| `/api/adguard/stats` | 5 s |
| `/api/adguard/querylog` | 3 s |
| `/api/traffic` | 2 s (smooth rate graphs) |
| `/api/firewall/rules` | 30 s (rarely changes) |
| `/api/firewall/counters` | 5 s |
| `/api/qos` | 5 s |
| `/api/upnp` | 15 s |

### 8.5 Shared components

Lives in `src/components/`:

- **`<StatusBadge>`** — healthy/degraded/failed, emerald/amber/rose (uses design system tokens).
- **`<MonoText>`** — wraps `span` with `font-mono tabular-nums`, used for IPs/MACs/counters.
- **`<TunnelBadge>` / `<ClientBadge>` / `<PoolBadge>`** — clickable chips that navigate to the relevant detail route. This is how cross-linking happens.
- **`<StaleIndicator>`** — shows a subtle badge on cards whose backing collector is stale.
- **`<Sparkline>`** — thin wrapper around Recharts for in-card mini-charts.
- **`<DataTable>`** — generic, sortable, filterable table wrapping shadcn's `Table`.
- **`<Sidebar>`** / **`<Layout>`** — the SENTINEL OS chrome from the designs (dark, no borders, mono labels).

### 8.6 Design system reference

The visual language is defined in `modules/dashboard/designs/design-system.md` ("The Silent Sentinel"). Rules:

- Dark-first, no pure white (`on_surface = #e7e5e4`)
- No 1px solid borders for sectioning — use `surface_container_*` tiers
- Inter for UI, JetBrains Mono for all technical data
- Semantic color accents only (emerald / amber / rose)
- `tabular-nums` on every counter
- shadcn primitives, not bespoke components

All Tailwind tokens (colors, radii) come from a single `tailwind.config.ts` that maps the Silent Sentinel palette onto shadcn's CSS variable convention.

## 9. Security & threat model

### 9.1 Trust boundaries (layered)

1. **Firewall allowlist** (`router.dashboard.allowedSources` → nftables input rule). **Primary gate.** Enforced in-kernel before any HTTP handler runs. Eval-time assertion forbids enabling the module with an empty allowlist. Untrusted LAN devices cannot reach the dashboard port at all.
2. **No-exec-in-handler invariant** (§7.2). Handlers only touch the in-memory `State` cache; they never fork, never read `/proc`, never call `nft`/`wg`/`conntrack`. A bug in a handler cannot be turned into CPU exhaustion via polling.
3. **Rate limiting** (§7.6, per-IP + global token buckets). Contains app-level bugs on *allowed* clients — a runaway browser tab cannot saturate the collector.
4. **Systemd sandbox** (§6.3: `DynamicUser`, `CAP_NET_ADMIN`+`CAP_NET_RAW` only, `ProtectSystem=strict`, `MemoryDenyWriteExecute`, narrow `ReadOnlyPaths`). A bug in the Go process cannot escalate to host compromise.

### 9.2 In-scope threats

- **A compromised IoT/untrusted device on `192.168.1.0/24` enumerates dashboard data.** Mitigated by layer 1: untrusted devices are not in `allowedSources`, so their packets to the dashboard port are dropped in-kernel. They can still reach AdGuard's own UI on `:3000` — that's a pre-existing LAN trust decision that predates this dashboard and is out of scope here, but worth knowing about.
- **A bug in the dashboard triggers unbounded exec / CPU exhaustion.** Prevented by layers 2 and 3.
- **A bug in the dashboard leaks PII through logs.** Logs are structured JSON to the systemd journal, which is root-only readable.
- **A noisy admin browser tab triggers excessive polling.** Layer 3 returns 429 once thresholds are hit; TanStack Query backs off.
- **A malicious asset swap at the release URL.** The flake pins the SHA256; a swapped asset fails Nix's fetch-time hash check.

### 9.3 Out-of-scope threats

- **Host compromise.** If an attacker has root on the router, the dashboard is not a line of defense.
- **WAN-side exposure.** The dashboard binds to LAN only; nftables `input` drops WAN-originated traffic by policy, regardless of the dashboard's firewall rules.
- **Supply-chain attack on the GitHub repo itself.** An attacker who can push to `main` can also update `version.json` to a hash they control. The real trust anchor is the GitHub repo, not the release asset. Mitigations (branch protection, signed commits, required reviews) are orthogonal repo-level policies.
- **DNS-query enumeration via AdGuard's own UI.** AdGuard binds to `0.0.0.0:3000` unauthenticated; any LAN client can already read its query log directly. The dashboard neither creates nor addresses this exposure. Tightening AdGuard itself (binding to `127.0.0.1`, adding auth, etc.) is a separate policy decision.
- **Insider threat from an *allowed* admin device that is itself compromised.** The allowlist trusts the admin host; if that host is compromised, the attacker inherits its trust on the dashboard. Mitigations (basic auth, mTLS, client-cert pinning) are tracked as v2+ defense-in-depth.

## 10. Development & release workflow

### 10.1 Local (no router)

The backend can run on a dev machine with a `--fake-data` flag that populates `State` from fixture JSON instead of execing commands. The frontend's Vite dev server proxies `/api/*` to `http://localhost:9090`. This lets UI iteration happen on the Mac without touching the Pi.

### 10.2 On-router integration

A scripted `make dev-on-router` target (or equivalent) does:

1. Cross-build the binary with `GOOS=linux GOARCH=arm64 go build`
2. `scp` to `/tmp/dashboard-dev` on the router
3. `ssh root@192.168.1.1 systemctl stop router-dashboard && /tmp/dashboard-dev --bind=...`

This is for dev loops only; production deploys go through the flake.

### 10.3 Release

1. Merge to `main` with dashboard changes
2. CI builds, releases, bumps `version.json` (auto-committed)
3. `git pull` in the flake repo on the Mac
4. rsync + `nixos-rebuild switch --flake /etc/nixos#router` on the router
5. Done

**Bootstrap (first release):** Before CI has ever run, no release exists. The dashboard module ships with a placeholder `version.json` (`version = "bootstrap"`, empty `url` and `hash`), and the module's `config` is wrapped in `lib.mkIf (cfg.enable && v.version != "bootstrap")` — so enabling the dashboard before the first CI release is a silent no-op with a trace warning. After the first CI-triggered push to `main`, `version.json` has a real release and the module activates normally. Bootstrapping from scratch is a two-step: (1) merge the dashboard module with the placeholder, (2) wait for CI to build and auto-commit the first real `version.json`, then pull and deploy.

## 11. Testing strategy

v1 targets basic confidence, not comprehensive coverage:

- **Go unit tests** for parsers (`nft --json`, `/proc/net/dev`, `wg show dump`, AdGuard JSON). Fixture-based, no network.
- **Go integration smoke test** — spins up the server with `--fake-data`, hits every endpoint, asserts 200 + schema.
- **Frontend:** Vitest + Testing Library for the 3-4 most complex components (`DataTable`, cross-linking badges, `StaleIndicator`). No full page-level tests.
- **No end-to-end** (no Playwright) in v1. The dashboard is inspected manually on the real router after deploy.

CI runs tests before the release job; failed tests prevent a release.

## 12. v1 scope freeze

**Must ship in v1** (from user's "Everything (full unified)" answer):

- [x] Home/overview page
- [x] VPN tunnels list + per-tunnel detail
- [x] VPN pools list + per-pool detail
- [x] Clients list + per-client detail
- [x] AdGuard stats + live query log
- [x] Interface traffic graphs
- [x] Firewall / NAT / PBR view
- [x] QoS stats view
- [x] System health view
- [x] UPnP leases view
- [x] Dark-first Silent Sentinel design
- [x] TanStack Query polling
- [x] NixOS module with `router.dashboard.enable`
- [x] GH Actions → GH Release → `version.json` → flake deploy

**Explicitly out of v1** (tracked in §13):

- Per-tunnel detail page (deferred to v1.1 — list view is enough)
- Designs for every view (designer delivered 4 of 10; the rest are implemented by pattern from the 4, empty/error states likewise)
- Mobile-specific layouts (responsive only)
- Historical time-series (Prometheus export hook present, disabled by default)
- Any mutation endpoint

## 13. v2+ roadmap (non-binding)

Candidate follow-ups, in rough priority order. Nothing here is a commitment — recorded so future decisions don't paint v1 into a corner:

1. **Mutation actions** (the big one):
   - Toggle a client between WAN and a pool
   - Add/remove a MAC in the allowlist
   - Flush conntrack for a client
   - Restart a tunnel
   - Force AdGuard filter refresh
   - Add/remove a DNS rewrite or blocked domain
2. **Auth** — basic auth with a secret file, required once mutation endpoints exist
3. **Cachix** — only if CI+release becomes the bottleneck (unlikely given how rarely dashboard changes)
4. **Historical metrics** — Prometheus scrape endpoint + bundled Grafana preset
5. **Designs for the remaining 5 views** + empty/error states
6. **Mobile-first re-pass** — the SPA is responsive but the information density isn't tuned for phones
7. **Multi-arch** — x86_64-linux / darwin support for the dashboard binary if a non-Pi consumer ever shows up
8. **External-user `dashboard-from-source` derivation** — `buildNpmPackage` + `buildGoModule` fallback so forks can rebuild without the SHA256 trust anchor

## 14. Open questions

### 14.1 Resolved by Codex adversarial review (2026-04-11)

Four findings from the review were addressed by in-place edits to this spec:

1. **[high] LAN-wide unauthenticated exposure** → access model reworked (§2.2, §6.1 new `allowedSources` option, §6.2 firewall gate, §9 layered trust boundaries). Dashboard now requires an explicit source-IP allowlist enforced via `router.nftables.extraInputRules`; eval-time assertion forbids empty lists. App-level auth remains v2+, but the trust boundary is now the firewall — the compromised-IoT threat is mitigated at layer 1.
2. **[high] Request-handler conntrack scans → self-DoS** → §7.2 now forbids any handler from triggering exec work. Per-client detail endpoints serve filtered subsets of the already-collected cold-tier cache. AdGuard querylog is the one remote call and is coalesced via `singleflight` + 3 s TTL.
3. **[high] Release non-idempotency** → §5.2.1 rewritten with a `concurrency: { group: build-dashboard, cancel-in-progress: false }` guard, a rebase+retry loop around the `version.json` push, and a post-push verify step that refetches the URL and sha256-checks against the committed hash. Partial-failure recovery explicitly documented.
4. **[medium] CI trigger coverage gap** → §5.2.2 adds a companion `nix-evaluation-check.yml` workflow that runs `nix flake check --no-build` on every `**/*.nix` or `flake.lock` change — catching Nix-level breakage that the binary-build workflow wouldn't trigger on.

Additionally, §7.6 was introduced for per-IP + global rate limiting as defense-in-depth on top of the firewall allowlist, even though it wasn't a direct review finding — it closes the same class of "runaway polling" risk at the app layer.

### 14.2 Still open — defaults the user should confirm

1. **Which IPs go in `allowedSources`?** This is the single most important operational detail the user needs to supply before the first deploy. Candidates from the current config (`configuration.nix` DHCP leases and implied admin devices):
   - `192.168.1.154` — giko-pc (MAC `44:fa:66:65:86:43`, currently dynamic; **bump to static** for this to work)
   - `192.168.1.117` — LAPTOP (already a static lease)
   - `192.168.1.119` — pi5 (already a static lease; admin-capable)
   - Optionally MacBookAir / Nikitas-MBP — also need static leases first
   
   v1 default: empty list → module is asserted off. User must populate.
2. **Port.** Default `9090`. Alternatives: `8080` (common, often colliding), `3001` (next to AdGuard), `8443` (suggests TLS we don't have). Leaving at `9090` unless objected.
3. **Log level default.** `info` — could be `warn` if the service turns out to be chatty in practice.
4. **`exit_ip` refresh cadence.** 5 min in the cold tier means the "current exit" shown on the home page can be slightly stale. Could move to "refresh on demand when a pool health event fires" — more code, fresher data. v1 default: 5 min.
5. **How to compute `current_tunnel` for a client.** Parses conntrack ct mark. Served from the 30 s cold-tier snapshot (never on-demand — see §7.2). v1 default: 30 s granularity is accepted.
6. **Per-service pinned list for `system.services[]`.** The spec names 8 services; is there anything else you want visible on the System view? (e.g., `zram-swap.service`, custom `cake-qos.service`, `flow-offload.service`)
7. **Logging destination.** Default: systemd journal via stdout. Alternative: write to a file under `/var/log/router-dashboard/` if you want `tail -f` without `journalctl -f`. v1 default: journal only.
8. **Frontend framework fine print.** React Router v6 vs. v7, TanStack Query v5, Tailwind v4 — pinned in the first `package.json` commit; this spec doesn't freeze the patch version.

---

## Appendix A: Files referenced by this spec

- `modules/dashboard/designs/` — 4 designer exports + design-system.md + README
- `modules/dashboard/docs/design-brief.md` — the brief given to the designer
- `modules/dashboard/docs/spec.md` — this file
- Future: `modules/dashboard/docs/api.md` — endpoint shapes in detail
- Future: `modules/dashboard/{backend,frontend,package.nix,default.nix,version.json}` — implementation
- Future: `.github/workflows/build-dashboard.yml` — binary build + release + version.json bump
- Future: `.github/workflows/nix-evaluation-check.yml` — `nix flake check --no-build` on Nix changes

## Appendix B: Review checklist for this spec

Before moving to the implementation plan, confirm:

- [ ] All 13 endpoints are ones you actually want
- [ ] The collector cadence tiers (2 s / 5 s / 30 s) match your expectations
- [ ] The firewall-allowlist access model (§2.2) is the right trust boundary for v1
- [ ] `router.dashboard.allowedSources` has the right set of admin IPs (§14.2 item 1)
- [ ] The NixOS module option names are ones you're happy to live with
- [ ] The distribution model (CI → GH Release → `version.json` → `fetchurl`) with concurrency+retry+verify (§5.2.1) is the one you want
- [ ] The repo layout fits how you like modules organized
- [ ] The v1 scope freeze covers everything you want, nothing you don't
- [ ] The open questions in §14.2 are resolved (or explicitly deferred)
