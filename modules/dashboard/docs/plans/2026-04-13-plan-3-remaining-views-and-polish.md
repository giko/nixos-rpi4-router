# Plan 3 — Remaining Views + Polish Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.
>
> **Workflow note:** After each task lands on `main`, run `/codex:review --wait --base <SHA-before-task>` to validate the change. Fix findings before moving on. This was the agreed cadence for Phase 2 and applies to Phase 3 too.

**Goal:** Build the six remaining placeholder pages (VPN Tunnels list+detail, Traffic, Firewall+UPnP, QoS, System) plus three new backend endpoints they require, taking the dashboard to v1 feature-complete.

**Architecture:** Backend-first — add three sources (`nft`, `tc`, derived UPnP) + their collectors + endpoints, then implement the frontend pages that consume them. Pages copy the established Phase 2 patterns verbatim (TanStack Query + Envelope + DataTable + StaleIndicator + error-then-loading guard). UPnP active mappings are derived from the `inet miniupnpd` nft table because miniupnpd has no on-disk lease file in this configuration. v1 stays read-only.

**Tech Stack:** Go 1.25 (backend), `nft --json` + `tc -s` parsers, React 18 + TypeScript + Vite + Tailwind + TanStack Query + React Router + Recharts (frontend), shadcn/ui primitives. No new dependencies.

**Reference docs:**
- Spec: `modules/dashboard/docs/spec.md` §6–§12
- Design brief: `modules/dashboard/docs/design-brief.md`
- Phase 2 plan (precedent): `modules/dashboard/docs/plans/2026-04-11-plan-2-backend-and-core-views.md`
- Project rules (deploy/git): `/Users/giko/Documents/router/CLAUDE.md`

**Working directory:** `/Users/giko/Documents/nixos-rpi4-router` (public flake repo). Per-repo git identity is `giko <gikon27@proton.me>` — do NOT add `Co-Authored-By` trailers and do NOT mention personal names in commit messages.

**Out-of-scope for Phase 3:**
- Mutations (toggle client routing, restart tunnel, add port forward, etc.) — v2.
- Per-CPU breakdown on System page — v2 (spec §7.4 only contracts aggregate CPU).
- WebSocket push — v2.
- Authentication — v2 (firewall-level allowlist remains the only access control).

---

## File Structure

### Backend additions (new files)

```
modules/dashboard/backend/internal/sources/nft/
├── nft.go                # JSON ruleset parser
├── nft_test.go
└── testdata/
    └── ruleset.json      # Live sample from router

modules/dashboard/backend/internal/sources/tc/
├── tc.go                 # `tc -s qdisc show dev X` parser (CAKE, HTB, fq_codel)
├── tc_test.go
└── testdata/
    ├── cake_egress.txt
    └── htb_ingress.txt

modules/dashboard/backend/internal/model/
├── firewall.go           # Firewall struct: port_forwards, pbr, allowed_macs,
│                         #   blocked_forward_count_1h, chains-with-nested-counters,
│                         #   upnp_leases
└── qos.go                # QoS struct (egress/ingress qdisc stats)

modules/dashboard/backend/internal/collector/
├── firewall.go           # Firewall collector (medium tier, 5s)
├── firewall_test.go
├── qos.go                # QoS collector (medium tier, 5s)
└── qos_test.go

modules/dashboard/backend/internal/server/
├── handlers_firewall.go  # /api/firewall/rules, /api/firewall/counters, /api/upnp
└── handlers_qos.go       # /api/qos
```

### Backend additions (modifications)

```
modules/dashboard/default.nix                         # Export port_forwards + pbr_source_rules + pbr_domain_rules to dashboard-config.json
modules/dashboard/backend/internal/topology/topology.go  # Add PortForwards, PBRSourceRules, PBRDomainRules fields
modules/dashboard/backend/internal/topology/topology_test.go
modules/dashboard/backend/internal/state/state.go    # SetFirewall/SetQoS + Snapshot* methods
modules/dashboard/backend/internal/state/state_test.go
modules/dashboard/backend/internal/server/server.go  # Register /api/firewall/{rules,counters}, /api/upnp, /api/qos
modules/dashboard/backend/cmd/dashboard/main.go      # Wire Firewall + QoS collectors (IFB derived from topo.WANInterface)
```

### Frontend additions (new files)

```
modules/dashboard/frontend/src/pages/
├── VpnTunnels.tsx        # /vpn/tunnels list
├── VpnTunnelDetail.tsx   # /vpn/tunnels/:name
├── Traffic.tsx           # /traffic
├── System.tsx            # /system
├── Firewall.tsx          # /firewall (incl. UPnP section)
└── Qos.tsx               # /qos
```

### Frontend additions (modifications)

```
modules/dashboard/frontend/src/lib/api.ts           # Add FirewallRules, FirewallChain, RuleCounter, UPnPLease, QoS types + api.firewallRules/firewallCounters/upnp/qos calls
modules/dashboard/frontend/src/lib/query-keys.ts    # Add firewallRules/firewallCounters/upnp/qos keys
modules/dashboard/frontend/src/App.tsx              # Replace Phase-3 Placeholder routes with real pages
modules/dashboard/frontend/src/pages/Placeholder.tsx # Delete after all routes consume real pages
```

---

## Conventions used in this plan

- **TDD for parsers and collectors** (write fixture → write failing test → minimal code → green → commit). Skip TDD for trivial wiring (handler delegates to a single state method).
- **Frontend tests for parsers / data-shaping helpers only.** Page tests are not required for Phase 3 (the project has no React Testing Library setup yet); each page gets a manual `npm run dev` smoke check before commit.
- **One commit per task unless the task is large** (page + types). Commit messages prefixed `dashboard:` like Phase 2.
- **No comments unless the WHY is non-obvious.** Don't echo the task numbers in code comments.
- **`go test ./...` in `modules/dashboard/backend/` after every backend task — must be green before commit.**
- **`npx tsc --noEmit` in `modules/dashboard/frontend/` after every frontend task — must be clean before commit.**

---

## Task list

1. nft source: ruleset JSON parser + UPnP-mapping extractor (synthetic fixture — no live ruleset goes in git)
2. tc source: CAKE + HTB + fq_codel qdisc stats parser
3. Topology expansion (`port_forwards`, `pbr_source_rules`, `pbr_domain_rules`) + `Firewall` / `QoS` models
4. Backend state: `SetFirewall` / `SnapshotFirewall` + `SetQoS` / `SnapshotQoS`
5. Firewall collector (uses nft source + topology; 1h forward-drop ring)
6. QoS collector (uses tc source)
7. Handlers: `/api/firewall/rules`, `/api/firewall/counters`, `/api/upnp`, `/api/qos` — per-endpoint stale windows
8. Wire collectors + routes in `main.go` + `server.go`
9. Frontend API types + query keys for firewall, upnp, qos (matches spec §7.4 shapes)
10. VPN Tunnels list page
11. VPN Tunnel detail page
12. Traffic page
13. System page
14. Firewall + UPnP page (port forwards, PBR, allowlist, counters, leases)
15. QoS page
16. Routes wired in `App.tsx`; drop unused Placeholder paths
17. Empty/error/refetch-failed audit across every page
18. Deploy to router via flake-split workflow + smoke test

---

## Task 1: nft source

**Files:**
- Create: `modules/dashboard/backend/internal/sources/nft/nft.go`
- Create: `modules/dashboard/backend/internal/sources/nft/nft_test.go`
- Create: `modules/dashboard/backend/internal/sources/nft/testdata/ruleset.json`

**Why this design:** `nft --json list ruleset` returns a flat array under `nftables[]` where each entry is a `table`, `chain`, `rule`, or `set`. We parse this into a strongly-typed shape suitable for the dashboard: a slice of chains (with table+name+hook+policy+rule count) for the rules list, a slice of named counters (tied back to the rule that owns them) for the counters list, and a separate slice of UPnP mappings extracted from the `inet miniupnpd` table. Splitting at parse time means handlers don't reparse JSON.

- [ ] **Step 1: Create a synthetic fixture (DO NOT commit live `nft --json` output)**

This repo is public. A live `nft --json list ruleset` dump contains real port forwards, private LAN addresses, the actual MAC allowlist, and UPnP peer details — committing it publishes the router's firewall topology. Use a hand-crafted fixture that exercises every parser branch without exposing real data.

Create `modules/dashboard/backend/internal/sources/nft/testdata/ruleset.json` with this content verbatim:

```json
{
  "nftables": [
    { "metainfo": { "version": "1.1.5", "release_name": "Test", "json_schema_version": 1 } },
    { "table": { "family": "inet", "name": "filter", "handle": 1 } },
    { "chain": { "family": "inet", "table": "filter", "name": "input", "handle": 1, "type": "filter", "hook": "input", "prio": 0, "policy": "drop" } },
    { "chain": { "family": "inet", "table": "filter", "name": "forward", "handle": 2, "type": "filter", "hook": "forward", "prio": 0, "policy": "drop" } },
    { "chain": { "family": "inet", "table": "filter", "name": "output", "handle": 3, "type": "filter", "hook": "output", "prio": 0, "policy": "accept" } },
    { "rule": { "family": "inet", "table": "filter", "chain": "input", "handle": 10, "comment": "allow established", "expr": [ { "match": { "op": "in", "left": { "ct": { "key": "state" } }, "right": ["established", "related"] } }, { "accept": null } ] } },
    { "rule": { "family": "inet", "table": "filter", "chain": "input", "handle": 11, "comment": "catch-all drop", "expr": [ { "counter": { "packets": 12345, "bytes": 678910 } }, { "drop": null } ] } },
    { "rule": { "family": "inet", "table": "filter", "chain": "forward", "handle": 20, "comment": "blocked mac drop", "expr": [ { "counter": { "packets": 77, "bytes": 4096 } }, { "drop": null } ] } },
    { "rule": { "family": "inet", "table": "filter", "chain": "forward", "handle": 21, "comment": "forward catch-all", "expr": [ { "counter": { "packets": 0, "bytes": 0 } }, { "drop": null } ] } },
    { "table": { "family": "inet", "name": "miniupnpd", "handle": 2 } },
    { "chain": { "family": "inet", "table": "miniupnpd", "name": "prerouting_miniupnpd", "handle": 1, "type": "nat", "hook": "prerouting", "prio": -100, "policy": "accept" } }
  ]
}
```

This fixture covers: three filter chains (two `drop` policies, one `accept`), three counter rules (one with hits, one with zero), a separate `inet/miniupnpd` table whose prerouting chain has **no** rules (exercises the empty-UPnP branch). Synthetic DNAT mapping is tested inline in `TestParseUPnPSyntheticMapping`.

Verify it parses:

```bash
python3 -c 'import json; print(len(json.load(open("modules/dashboard/backend/internal/sources/nft/testdata/ruleset.json"))["nftables"]))'
```

Expected: `11`.

- [ ] **Step 2: Write the failing parser test**

Create `modules/dashboard/backend/internal/sources/nft/nft_test.go`:

```go
package nft

import (
	"os"
	"testing"
)

func TestParseRulesetFromFixture(t *testing.T) {
	raw, err := os.ReadFile("testdata/ruleset.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	r, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if len(r.Chains) == 0 {
		t.Fatal("expected at least one chain, got 0")
	}

	var sawInputDrop bool
	for _, c := range r.Chains {
		if c.Family == "inet" && c.Table == "filter" && c.Name == "input" {
			if c.Hook != "input" {
				t.Errorf("filter/input hook = %q, want input", c.Hook)
			}
			if c.Policy != "drop" {
				t.Errorf("filter/input policy = %q, want drop", c.Policy)
			}
			sawInputDrop = true
		}
	}
	if !sawInputDrop {
		t.Fatal("did not find inet/filter/input chain")
	}

	if len(r.Counters) == 0 {
		t.Fatal("expected at least one counter, got 0")
	}
	for _, ct := range r.Counters {
		if ct.Bytes == 0 && ct.Packets == 0 {
			continue
		}
		if ct.ChainName == "" {
			t.Errorf("counter without chain name: %+v", ct)
		}
	}
}

func TestParseUPnPEmptyTable(t *testing.T) {
	raw, err := os.ReadFile("testdata/ruleset.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	r, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	// The fixture currently has the miniupnpd table but no active port
	// mappings. Verify Mappings is non-nil and empty.
	if r.UPnPMappings == nil {
		t.Fatal("UPnPMappings should be a non-nil empty slice, got nil")
	}
}

func TestParseUPnPSyntheticMapping(t *testing.T) {
	// Synthetic ruleset with one UPnP-style DNAT rule in the
	// inet miniupnpd prerouting_miniupnpd chain.
	raw := []byte(`{"nftables":[
		{"table":{"family":"inet","name":"miniupnpd","handle":1}},
		{"chain":{"family":"inet","table":"miniupnpd","name":"prerouting_miniupnpd","handle":2,"type":"nat","hook":"prerouting","prio":-100,"policy":"accept"}},
		{"rule":{"family":"inet","table":"miniupnpd","chain":"prerouting_miniupnpd","handle":10,"comment":"plex/0","expr":[
			{"match":{"op":"==","left":{"meta":{"key":"iifname"}},"right":"eth1"}},
			{"match":{"op":"==","left":{"payload":{"protocol":"tcp","field":"dport"}},"right":35978}},
			{"dnat":{"addr":"192.168.20.6","port":32400}}
		]}}
	]}`)
	r, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(r.UPnPMappings) != 1 {
		t.Fatalf("UPnPMappings count = %d, want 1: %+v", len(r.UPnPMappings), r.UPnPMappings)
	}
	m := r.UPnPMappings[0]
	if m.Protocol != "tcp" || m.ExternalPort != 35978 || m.InternalAddr != "192.168.20.6" || m.InternalPort != 32400 {
		t.Errorf("mapping = %+v, want tcp 35978 -> 192.168.20.6:32400", m)
	}
	if m.Description != "plex/0" {
		t.Errorf("description = %q, want plex/0", m.Description)
	}
}
```

- [ ] **Step 3: Run tests to confirm failure**

```bash
cd modules/dashboard/backend && go test ./internal/sources/nft/...
```

Expected: FAIL with "package nft has no Go files" or similar.

- [ ] **Step 4: Implement the parser**

Create `modules/dashboard/backend/internal/sources/nft/nft.go`:

```go
// Package nft parses `nft --json list ruleset` output into a typed
// shape the dashboard's firewall/UPnP collectors consume. The on-wire
// JSON is a flat array under "nftables[]" of heterogeneous entries
// (table / chain / rule / set / counter); we walk it once, building
// the by-chain index and extracting per-rule counters and UPnP DNAT
// mappings in the same pass.
package nft

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// Ruleset is the parsed projection of `nft --json list ruleset`.
type Ruleset struct {
	Chains       []Chain        // every chain across every table
	Counters     []Counter      // every {"counter": {...}} expression encountered, tied back to chain+rule handle
	UPnPMappings []UPnPMapping  // DNAT rules extracted from inet/miniupnpd chains
}

// Chain summarises one nft chain.
type Chain struct {
	Family   string `json:"family"`           // "ip" | "ip6" | "inet" | ...
	Table    string `json:"table"`
	Name     string `json:"name"`
	Type     string `json:"type,omitempty"`   // "filter" | "nat" | ""
	Hook     string `json:"hook,omitempty"`   // "input" | "forward" | "prerouting" | ...
	Priority int    `json:"priority,omitempty"`
	Policy   string `json:"policy,omitempty"` // "accept" | "drop" | ""
	Handle   int    `json:"handle"`
	RuleCount int   `json:"rule_count"`
}

// Counter is one inline counter expression's value, tagged with the
// chain it lives in and the rule's nft handle. The handle is what
// nft uses to identify a rule; it's the only stable cross-reference
// across reloads (rules don't have user-visible ids).
type Counter struct {
	Family    string `json:"family"`
	Table     string `json:"table"`
	ChainName string `json:"chain"`
	Handle    int    `json:"handle"`
	Comment   string `json:"comment,omitempty"`
	Packets   int64  `json:"packets"`
	Bytes     int64  `json:"bytes"`
}

// UPnPMapping is one active port forward established by miniupnpd.
type UPnPMapping struct {
	Protocol     string `json:"protocol"`
	ExternalPort int    `json:"external_port"`
	InternalAddr string `json:"internal_addr"`
	InternalPort int    `json:"internal_port"`
	Description  string `json:"description,omitempty"`
}

// Runner executes nft and returns its stdout. Tests inject a fake.
type Runner func(ctx context.Context, args ...string) ([]byte, error)

// DefaultRunner runs the real nft binary with the given args.
func DefaultRunner(ctx context.Context, args ...string) ([]byte, error) {
	out, err := exec.CommandContext(ctx, "nft", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("nft %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

// Collect runs `nft --json list ruleset` via the given runner and
// returns a parsed Ruleset.
func Collect(ctx context.Context, run Runner) (*Ruleset, error) {
	raw, err := run(ctx, "--json", "list", "ruleset")
	if err != nil {
		return nil, err
	}
	return Parse(raw)
}

// Parse decodes the raw nft JSON envelope and projects it into Ruleset.
func Parse(raw []byte) (*Ruleset, error) {
	var env struct {
		Nftables []json.RawMessage `json:"nftables"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("nft: parse envelope: %w", err)
	}

	out := &Ruleset{
		Chains:       []Chain{},
		Counters:     []Counter{},
		UPnPMappings: []UPnPMapping{},
	}
	chainsByKey := make(map[string]int) // family/table/chain → index in out.Chains

	for _, entry := range env.Nftables {
		var top map[string]json.RawMessage
		if err := json.Unmarshal(entry, &top); err != nil {
			continue
		}
		if raw, ok := top["chain"]; ok {
			var c struct {
				Family string `json:"family"`
				Table  string `json:"table"`
				Name   string `json:"name"`
				Type   string `json:"type"`
				Hook   string `json:"hook"`
				Prio   int    `json:"prio"`
				Policy string `json:"policy"`
				Handle int    `json:"handle"`
			}
			if err := json.Unmarshal(raw, &c); err == nil {
				key := c.Family + "/" + c.Table + "/" + c.Name
				chainsByKey[key] = len(out.Chains)
				out.Chains = append(out.Chains, Chain{
					Family: c.Family, Table: c.Table, Name: c.Name,
					Type: c.Type, Hook: c.Hook, Priority: c.Prio,
					Policy: c.Policy, Handle: c.Handle,
				})
			}
		}
		if raw, ok := top["rule"]; ok {
			var r struct {
				Family  string            `json:"family"`
				Table   string            `json:"table"`
				Chain   string            `json:"chain"`
				Handle  int               `json:"handle"`
				Comment string            `json:"comment"`
				Expr    []json.RawMessage `json:"expr"`
			}
			if err := json.Unmarshal(raw, &r); err == nil {
				key := r.Family + "/" + r.Table + "/" + r.Chain
				if idx, ok := chainsByKey[key]; ok {
					out.Chains[idx].RuleCount++
				}
				extractCounters(r.Family, r.Table, r.Chain, r.Handle, r.Comment, r.Expr, out)
				if r.Family == "inet" && r.Table == "miniupnpd" {
					if mapping, ok := extractUPnPMapping(r.Expr, r.Comment); ok {
						out.UPnPMappings = append(out.UPnPMappings, mapping)
					}
				}
			}
		}
	}
	return out, nil
}

func extractCounters(family, table, chain string, handle int, comment string, expr []json.RawMessage, out *Ruleset) {
	for _, e := range expr {
		var holder struct {
			Counter *struct {
				Packets int64 `json:"packets"`
				Bytes   int64 `json:"bytes"`
			} `json:"counter"`
		}
		if err := json.Unmarshal(e, &holder); err != nil {
			continue
		}
		if holder.Counter == nil {
			continue
		}
		out.Counters = append(out.Counters, Counter{
			Family: family, Table: table, ChainName: chain,
			Handle: handle, Comment: comment,
			Packets: holder.Counter.Packets, Bytes: holder.Counter.Bytes,
		})
	}
}

// extractUPnPMapping walks one rule's expression array looking for
// the (proto + dport) match plus the dnat target that miniupnpd emits
// for an active port forward. Returns false when the rule isn't a
// DNAT (e.g. the chain's policy rule).
func extractUPnPMapping(expr []json.RawMessage, comment string) (UPnPMapping, bool) {
	var m UPnPMapping
	m.Description = comment
	for _, e := range expr {
		var holder map[string]json.RawMessage
		if err := json.Unmarshal(e, &holder); err != nil {
			continue
		}
		if raw, ok := holder["match"]; ok {
			var match struct {
				Op    string          `json:"op"`
				Left  json.RawMessage `json:"left"`
				Right json.RawMessage `json:"right"`
			}
			if err := json.Unmarshal(raw, &match); err == nil {
				var leftPayload struct {
					Payload struct {
						Protocol string `json:"protocol"`
						Field    string `json:"field"`
					} `json:"payload"`
				}
				if err := json.Unmarshal(match.Left, &leftPayload); err == nil {
					if leftPayload.Payload.Field == "dport" {
						m.Protocol = leftPayload.Payload.Protocol
						var port int
						if err := json.Unmarshal(match.Right, &port); err == nil {
							m.ExternalPort = port
						}
					}
				}
			}
		}
		if raw, ok := holder["dnat"]; ok {
			var dnat struct {
				Addr string `json:"addr"`
				Port int    `json:"port"`
			}
			if err := json.Unmarshal(raw, &dnat); err == nil {
				m.InternalAddr = dnat.Addr
				m.InternalPort = dnat.Port
			}
		}
	}
	if m.InternalAddr == "" || m.ExternalPort == 0 {
		return UPnPMapping{}, false
	}
	if m.InternalPort == 0 {
		m.InternalPort = m.ExternalPort
	}
	return m, true
}
```

- [ ] **Step 5: Run tests, expect green**

```bash
cd modules/dashboard/backend && go test ./internal/sources/nft/... -v
```

Expected: all three tests pass.

- [ ] **Step 6: Commit**

```bash
git add modules/dashboard/backend/internal/sources/nft/
git commit -m "dashboard: add nft source — ruleset JSON parser with UPnP extraction"
```

---

## Task 2: tc source

**Files:**
- Create: `modules/dashboard/backend/internal/sources/tc/tc.go`
- Create: `modules/dashboard/backend/internal/sources/tc/tc_test.go`
- Create: `modules/dashboard/backend/internal/sources/tc/testdata/cake_egress.txt`
- Create: `modules/dashboard/backend/internal/sources/tc/testdata/htb_ingress.txt`

**Why this design:** `tc -s qdisc show dev <iface>` returns human-readable text, not JSON. Two distinct shapes matter to us: CAKE (one root qdisc with global stats + per-tin stats — Bulk/Best Effort/Voice) on the WAN egress, and HTB+fq_codel on the ingress IFB device (one HTB outer qdisc + one fq_codel inner qdisc with drops/marks/buffer info). Both share a "Sent X bytes Y pkt (dropped Z, overlimits A, requeues B)" counter line we extract first; then we have qdisc-specific keys.

- [ ] **Step 1: Capture live fixtures**

```bash
ssh root@192.168.1.1 "tc -s qdisc show dev eth1"     > modules/dashboard/backend/internal/sources/tc/testdata/cake_egress.txt
ssh root@192.168.1.1 "tc -s qdisc show dev ifb4eth1" > modules/dashboard/backend/internal/sources/tc/testdata/htb_ingress.txt
```

- [ ] **Step 2: Write failing tests**

Create `modules/dashboard/backend/internal/sources/tc/tc_test.go`:

```go
package tc

import (
	"os"
	"testing"
)

func TestParseCakeFromFixture(t *testing.T) {
	raw, err := os.ReadFile("testdata/cake_egress.txt")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	q, err := ParseCAKE(string(raw))
	if err != nil {
		t.Fatalf("ParseCAKE: %v", err)
	}
	if q.Kind != "cake" {
		t.Errorf("Kind = %q, want cake", q.Kind)
	}
	if q.SentBytes <= 0 {
		t.Errorf("SentBytes = %d, want > 0", q.SentBytes)
	}
	if q.SentPackets <= 0 {
		t.Errorf("SentPackets = %d, want > 0", q.SentPackets)
	}
	if q.BandwidthBps == 0 {
		t.Errorf("BandwidthBps = 0; expected populated from `bandwidth 100Mbit`")
	}
	if len(q.Tins) != 3 {
		t.Errorf("Tin count = %d, want 3 (Bulk/Best Effort/Voice)", len(q.Tins))
	}
	for _, tin := range q.Tins {
		if tin.Name == "" {
			t.Errorf("tin missing name: %+v", tin)
		}
	}
}

func TestParseHTBFromFixture(t *testing.T) {
	raw, err := os.ReadFile("testdata/htb_ingress.txt")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	q, err := ParseHTB(string(raw))
	if err != nil {
		t.Fatalf("ParseHTB: %v", err)
	}
	if q.Kind != "htb+fq_codel" {
		t.Errorf("Kind = %q, want htb+fq_codel", q.Kind)
	}
	if q.SentBytes <= 0 {
		t.Errorf("SentBytes = %d, want > 0", q.SentBytes)
	}
	// The HTB fixture's fq_codel block should populate ECN-mark + new-flow stats.
	if q.NewFlowCount == 0 {
		t.Errorf("NewFlowCount = 0; expected populated from fq_codel")
	}
}
```

- [ ] **Step 3: Run tests to confirm failure**

```bash
cd modules/dashboard/backend && go test ./internal/sources/tc/...
```

Expected: FAIL with "no Go files".

- [ ] **Step 4: Implement the parser**

Create `modules/dashboard/backend/internal/sources/tc/tc.go`:

```go
// Package tc parses `tc -s qdisc show dev <iface>` output for the two
// shaping setups the router uses:
//   * CAKE on WAN egress (eth1)
//   * HTB outer + fq_codel inner on WAN ingress IFB (ifb4eth1)
//
// The output is human-readable text; we look for known keywords and
// pull the adjacent number. Keys we care about are documented at
// https://man7.org/linux/man-pages/man8/tc-cake.8.html and
// https://man7.org/linux/man-pages/man8/tc-fq_codel.8.html.
package tc

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// QdiscStats is the unified shape both parsers emit.
type QdiscStats struct {
	Kind         string `json:"kind"`           // "cake" | "htb+fq_codel"
	BandwidthBps int64  `json:"bandwidth_bps"`  // CAKE: parsed from "bandwidth 100Mbit"
	SentBytes    int64  `json:"sent_bytes"`
	SentPackets  int64  `json:"sent_packets"`
	Dropped      int64  `json:"dropped"`
	Overlimits   int64  `json:"overlimits"`
	Requeues     int64  `json:"requeues"`
	BacklogBytes int64  `json:"backlog_bytes"`
	BacklogPkts  int64  `json:"backlog_pkts"`

	// CAKE-only.
	Tins []CAKETin `json:"tins,omitempty"`

	// HTB+fq_codel only.
	NewFlowCount  int64 `json:"new_flow_count,omitempty"`
	OldFlowsLen   int64 `json:"old_flows_len,omitempty"`
	NewFlowsLen   int64 `json:"new_flows_len,omitempty"`
	ECNMark       int64 `json:"ecn_mark,omitempty"`
	DropOverlimit int64 `json:"drop_overlimit,omitempty"`
}

// CAKETin is one CAKE traffic class.
type CAKETin struct {
	Name           string `json:"name"`
	ThreshKbit     int64  `json:"thresh_kbit"`
	TargetUs       int64  `json:"target_us"`
	IntervalUs     int64  `json:"interval_us"`
	PeakDelayUs    int64  `json:"peak_delay_us"`
	AvgDelayUs     int64  `json:"avg_delay_us"`
	BacklogBytes   int64  `json:"backlog_bytes"`
	Packets        int64  `json:"packets"`
	Bytes          int64  `json:"bytes"`
	Drops          int64  `json:"drops"`
	Marks          int64  `json:"marks"`
}

// Runner runs `tc` with the given args. Tests inject a fake.
type Runner func(ctx context.Context, args ...string) ([]byte, error)

// DefaultRunner exec-invokes /sbin/tc (or whatever is in PATH).
func DefaultRunner(ctx context.Context, args ...string) ([]byte, error) {
	out, err := exec.CommandContext(ctx, "tc", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("tc %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

// CollectCAKE runs `tc -s qdisc show dev <iface>` and returns the
// CAKE-shaped stats from the first qdisc reported (must be the root).
func CollectCAKE(ctx context.Context, run Runner, iface string) (QdiscStats, error) {
	out, err := run(ctx, "-s", "qdisc", "show", "dev", iface)
	if err != nil {
		return QdiscStats{}, err
	}
	return ParseCAKE(string(out))
}

// CollectHTB runs `tc -s qdisc show dev <iface>` and returns the
// merged HTB+fq_codel stats expected on the ingress IFB.
func CollectHTB(ctx context.Context, run Runner, iface string) (QdiscStats, error) {
	out, err := run(ctx, "-s", "qdisc", "show", "dev", iface)
	if err != nil {
		return QdiscStats{}, err
	}
	return ParseHTB(string(out))
}

// ParseCAKE walks the text output and pulls counters + per-tin stats.
func ParseCAKE(raw string) (QdiscStats, error) {
	q := QdiscStats{Kind: "cake"}
	lines := splitLines(raw)
	for i := 0; i < len(lines); i++ {
		ln := strings.TrimSpace(lines[i])
		if strings.HasPrefix(ln, "qdisc cake") {
			q.BandwidthBps = parseBandwidth(ln)
			continue
		}
		if strings.HasPrefix(ln, "Sent ") {
			q.SentBytes, q.SentPackets, q.Dropped, q.Overlimits, q.Requeues = parseSentLine(ln)
		}
		if strings.HasPrefix(ln, "backlog ") {
			q.BacklogBytes, q.BacklogPkts = parseBacklogLine(ln)
		}
	}
	q.Tins = parseCakeTins(lines)
	if q.SentBytes == 0 && q.SentPackets == 0 {
		return QdiscStats{}, fmt.Errorf("tc: no Sent line found in CAKE output")
	}
	return q, nil
}

// ParseHTB extracts HTB outer + fq_codel inner counters.
// The "Sent" line we care about is the LAST one (the fq_codel leaf
// totals); HTB's own Sent line counts the same packets but we want
// the leaf so any HTB-internal direct packets are not double-counted.
func ParseHTB(raw string) (QdiscStats, error) {
	q := QdiscStats{Kind: "htb+fq_codel"}
	lines := splitLines(raw)
	var lastSent string
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "Sent ") {
			lastSent = t
		}
	}
	if lastSent == "" {
		return QdiscStats{}, fmt.Errorf("tc: no Sent line found in HTB output")
	}
	q.SentBytes, q.SentPackets, q.Dropped, q.Overlimits, q.Requeues = parseSentLine(lastSent)

	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "backlog ") {
			q.BacklogBytes, q.BacklogPkts = parseBacklogLine(t)
		}
		if strings.Contains(t, "new_flow_count") || strings.Contains(t, "ecn_mark") {
			q.NewFlowCount = parseKey(t, "new_flow_count")
			q.ECNMark = parseKey(t, "ecn_mark")
			q.DropOverlimit = parseKey(t, "drop_overlimit")
		}
		if strings.Contains(t, "new_flows_len") || strings.Contains(t, "old_flows_len") {
			q.NewFlowsLen = parseKey(t, "new_flows_len")
			q.OldFlowsLen = parseKey(t, "old_flows_len")
		}
	}
	return q, nil
}

// --- helpers ---

func splitLines(s string) []string {
	var out []string
	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {
		out = append(out, sc.Text())
	}
	return out
}

// parseSentLine extracts the standard tc summary:
//   Sent 42167731517 bytes 89707798 pkt (dropped 5613, overlimits 99439519 requeues 20020)
func parseSentLine(line string) (sentB, sentP, dropped, overlimits, requeues int64) {
	fields := strings.Fields(line)
	for i := 0; i < len(fields); i++ {
		switch fields[i] {
		case "Sent":
			if i+1 < len(fields) {
				sentB, _ = strconv.ParseInt(fields[i+1], 10, 64)
			}
		case "bytes":
			if i+1 < len(fields) {
				sentP, _ = strconv.ParseInt(fields[i+1], 10, 64)
			}
		case "(dropped":
			if i+1 < len(fields) {
				dropped, _ = strconv.ParseInt(strings.TrimSuffix(fields[i+1], ","), 10, 64)
			}
		case "overlimits":
			if i+1 < len(fields) {
				overlimits, _ = strconv.ParseInt(fields[i+1], 10, 64)
			}
		case "requeues":
			if i+1 < len(fields) {
				requeues, _ = strconv.ParseInt(strings.TrimSuffix(fields[i+1], ")"), 10, 64)
			}
		}
	}
	return
}

func parseBacklogLine(line string) (bytes, pkts int64) {
	// backlog 0b 0p requeues 0
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return
	}
	bytes, _ = strconv.ParseInt(strings.TrimSuffix(fields[1], "b"), 10, 64)
	pkts, _ = strconv.ParseInt(strings.TrimSuffix(fields[2], "p"), 10, 64)
	return
}

// parseBandwidth pulls "bandwidth NNNMbit" / "NNNKbit" / "NNNGbit" from
// CAKE's qdisc line and returns bits per second.
func parseBandwidth(line string) int64 {
	fields := strings.Fields(line)
	for i, f := range fields {
		if f != "bandwidth" || i+1 >= len(fields) {
			continue
		}
		v := fields[i+1]
		mul := int64(1)
		switch {
		case strings.HasSuffix(v, "Gbit"):
			mul = 1_000_000_000
			v = strings.TrimSuffix(v, "Gbit")
		case strings.HasSuffix(v, "Mbit"):
			mul = 1_000_000
			v = strings.TrimSuffix(v, "Mbit")
		case strings.HasSuffix(v, "Kbit"):
			mul = 1_000
			v = strings.TrimSuffix(v, "Kbit")
		case strings.HasSuffix(v, "bit"):
			v = strings.TrimSuffix(v, "bit")
		}
		n, _ := strconv.ParseInt(v, 10, 64)
		return n * mul
	}
	return 0
}

// parseKey returns the int64 that follows the given key on a tc stat
// line like "  maxpacket 68130 drop_overlimit 0 new_flow_count 9664721 ecn_mark 831".
func parseKey(line, key string) int64 {
	fields := strings.Fields(line)
	for i, f := range fields {
		if f == key && i+1 < len(fields) {
			n, _ := strconv.ParseInt(fields[i+1], 10, 64)
			return n
		}
	}
	return 0
}

// parseCakeTins finds the CAKE per-tin column block. The header row
// names the tins (typically "Bulk Best Effort Voice" for diffserv3),
// then each subsequent row is one stat across the tins. We walk down,
// grabbing the rows we care about.
func parseCakeTins(lines []string) []CAKETin {
	header := -1
	for i, ln := range lines {
		if strings.Contains(ln, "Bulk") && strings.Contains(ln, "Best Effort") {
			header = i
			break
		}
	}
	if header == -1 {
		return nil
	}
	names := strings.Fields(lines[header])
	// "Bulk Best Effort Voice" — note "Best Effort" is 2 fields, so we
	// rejoin: the line has exactly 4 tokens with diffserv3.
	if len(names) == 4 && names[1] == "Best" && names[2] == "Effort" {
		names = []string{"Bulk", "Best Effort", "Voice"}
	}
	tins := make([]CAKETin, len(names))
	for i := range tins {
		tins[i].Name = names[i]
	}
	for j := header + 1; j < len(lines); j++ {
		row := strings.Fields(lines[j])
		if len(row) == 0 {
			break
		}
		key := row[0]
		vals := row[1:]
		if len(vals) != len(tins) {
			continue
		}
		for i, v := range vals {
			n := parseTinValue(v)
			switch key {
			case "thresh":
				tins[i].ThreshKbit = n
			case "target":
				tins[i].TargetUs = n
			case "interval":
				tins[i].IntervalUs = n
			case "pk_delay":
				tins[i].PeakDelayUs = n
			case "av_delay":
				tins[i].AvgDelayUs = n
			case "backlog":
				tins[i].BacklogBytes = n
			case "pkts":
				tins[i].Packets = n
			case "bytes":
				tins[i].Bytes = n
			case "drops":
				tins[i].Drops = n
			case "marks":
				tins[i].Marks = n
			}
		}
	}
	return tins
}

// parseTinValue accepts numbers with units like "5ms", "100ms", "6250Kbit",
// "5002752b", "0", "20.7ms" and returns a microsecond/byte/kbit/raw value
// depending on the suffix; the caller knows which key it asked for.
// We normalise:
//   *ms      -> microseconds (for delay/interval/target rows)
//   *us      -> microseconds
//   *Kbit    -> kbit
//   *Mbit    -> kbit (×1000)
//   *Gbit    -> kbit (×1_000_000)
//   *b       -> bytes
//   bare num -> int (packets/drops/marks)
func parseTinValue(v string) int64 {
	if v == "0" {
		return 0
	}
	switch {
	case strings.HasSuffix(v, "us"):
		n, _ := strconv.ParseFloat(strings.TrimSuffix(v, "us"), 64)
		return int64(n)
	case strings.HasSuffix(v, "ms"):
		n, _ := strconv.ParseFloat(strings.TrimSuffix(v, "ms"), 64)
		return int64(n * 1000)
	case strings.HasSuffix(v, "Gbit"):
		n, _ := strconv.ParseFloat(strings.TrimSuffix(v, "Gbit"), 64)
		return int64(n * 1_000_000)
	case strings.HasSuffix(v, "Mbit"):
		n, _ := strconv.ParseFloat(strings.TrimSuffix(v, "Mbit"), 64)
		return int64(n * 1_000)
	case strings.HasSuffix(v, "Kbit"):
		n, _ := strconv.ParseInt(strings.TrimSuffix(v, "Kbit"), 10, 64)
		return n
	case strings.HasSuffix(v, "b"):
		n, _ := strconv.ParseInt(strings.TrimSuffix(v, "b"), 10, 64)
		return n
	}
	n, _ := strconv.ParseInt(v, 10, 64)
	return n
}
```

- [ ] **Step 5: Run tests, expect green**

```bash
cd modules/dashboard/backend && go test ./internal/sources/tc/... -v
```

Expected: both tests pass.

- [ ] **Step 6: Commit**

```bash
git add modules/dashboard/backend/internal/sources/tc/
git commit -m "dashboard: add tc source — CAKE + HTB+fq_codel qdisc parser"
```

---

## Task 3: Topology expansion + backend models for firewall and qos

**Spec contracts (§7.3 data sources, §7.4 API shapes):**

- `/api/firewall/rules` → `{port_forwards, pbr, allowed_macs, blocked_forward_count_1h}` — static topology config + one rolled-up counter.
- `/api/firewall/counters` → `{chains: [...]}` — dynamic nft counters nested per chain.
- `/api/upnp` → `{leases: [...]}` — miniupnpd-derived port mappings.
- `firewall.port_forwards` comes from `router.portForwards`; `firewall.pbr.*` from `router.pbr.{sourceRules, domainSets, sourceDomainRules, pooledRules}`.

Topology must therefore expose `port_forwards`, `pbr_source_rules`, `pbr_domain_rules`, plus the already-exported `pooled_rules` and `allowed_macs`. The nft-derived dynamic fields (chains, counters, UPnP leases) are populated by the collector.

**Files:**
- Modify: `modules/dashboard/default.nix`
- Modify: `modules/dashboard/backend/internal/topology/topology.go`
- Modify: `modules/dashboard/backend/internal/topology/topology_test.go`
- Create: `modules/dashboard/backend/internal/model/firewall.go`
- Create: `modules/dashboard/backend/internal/model/qos.go`

- [ ] **Step 1: Expose `port_forwards`, `pbr_source_rules`, `pbr_domain_rules` in `modules/dashboard/default.nix`**

Find the existing `dashboardConfigJson = builtins.toJSON {...}` block (around line 15) and add three new keys before `lan_interface`:

```nix
    port_forwards = map (pf: {
      protocol = pf.proto;
      external_port = pf.externalPort;
      destination = pf.destination;
    }) config.router.portForwards;
    pbr_source_rules = map (r: {
      inherit (r) sources tunnel;
    }) config.router.pbr.sourceRules;
    pbr_domain_rules = lib.mapAttrsToList (tunnel: domains: {
      inherit tunnel domains;
    }) config.router.pbr.domainSets;
```

`sourceDomainRules` (source ∧ domain-set rules) is out of scope for v1 — it's a rarely-used combo; we can surface it in v2 if needed.

- [ ] **Step 2: Add matching fields to `modules/dashboard/backend/internal/topology/topology.go`**

In the `Topology` struct, right after `AllowedMACs`:

```go
	PortForwards   []PortForward   `json:"port_forwards"`
	PBRSourceRules []PBRSourceRule `json:"pbr_source_rules"`
	PBRDomainRules []PBRDomainRule `json:"pbr_domain_rules"`
```

At the end of the file add:

```go
// PortForward is a static DNAT rule defined in nix module config.
type PortForward struct {
	Protocol     string `json:"protocol"`
	ExternalPort int    `json:"external_port"`
	Destination  string `json:"destination"` // "ip:port"
}

// PBRSourceRule routes every new connection from `Sources` through `Tunnel`.
type PBRSourceRule struct {
	Sources []string `json:"sources"`
	Tunnel  string   `json:"tunnel"` // tunnel name or "wan"
}

// PBRDomainRule routes traffic to `Domains` through `Tunnel`.
type PBRDomainRule struct {
	Tunnel  string   `json:"tunnel"`
	Domains []string `json:"domains"`
}
```

- [ ] **Step 3: Extend topology tests**

In `modules/dashboard/backend/internal/topology/topology_test.go`, extend the existing `TestLoadRoundTrip` (or the equivalent) fixture to include the new fields, and add assertions. Locate the existing JSON body and replace with:

```go
	body := `{
		"tunnels": [{"name":"wg_sw","interface":"wg_sw","fwmark":"0x20000","routing_table":200}],
		"pools": [{"name":"all","members":["wg_sw"]}],
		"pooled_rules": [{"sources":["192.168.1.10"],"pool":"all"}],
		"static_leases": [{"mac":"aa:bb:cc:dd:ee:ff","ip":"192.168.1.10","name":"laptop"}],
		"allowlist_enabled": true,
		"allowed_macs": ["aa:bb:cc:dd:ee:ff"],
		"lan_interface": "eth0",
		"wan_interface": "eth1",
		"port_forwards": [{"protocol":"tcp","external_port":35978,"destination":"192.168.20.6:32400"}],
		"pbr_source_rules": [{"sources":["192.168.1.225"],"tunnel":"wg_sw"}],
		"pbr_domain_rules": [{"tunnel":"wg_sw","domains":["example.com"]}]
	}`
```

Append assertions:

```go
	if len(topo.PortForwards) != 1 || topo.PortForwards[0].ExternalPort != 35978 {
		t.Errorf("PortForwards = %+v", topo.PortForwards)
	}
	if len(topo.PBRSourceRules) != 1 || topo.PBRSourceRules[0].Tunnel != "wg_sw" {
		t.Errorf("PBRSourceRules = %+v", topo.PBRSourceRules)
	}
	if len(topo.PBRDomainRules) != 1 || topo.PBRDomainRules[0].Domains[0] != "example.com" {
		t.Errorf("PBRDomainRules = %+v", topo.PBRDomainRules)
	}
```

- [ ] **Step 4: Create `model/firewall.go` matching the spec contracts**

```go
package model

// Firewall is the snapshot served by three endpoints:
//   /api/firewall/rules     → {port_forwards, pbr, allowed_macs, blocked_forward_count_1h}
//   /api/firewall/counters  → {chains: [...]}
//   /api/upnp               → {leases: [...]}
//
// A single snapshot holds all three projections because one nft parse
// and one topology load produce everything at once; the handlers carve
// out their own fields.
type Firewall struct {
	// Rules — static config exposed verbatim from topology.
	PortForwards []PortForward `json:"port_forwards"`
	PBR          PBR           `json:"pbr"`
	AllowedMACs  []string      `json:"allowed_macs"`

	// BlockedForwardCount1h is the delta of summed packet counters on
	// the forward chain's drop rules over the trailing ~60 minutes.
	// Rolled forward by the collector's in-memory ring buffer.
	BlockedForwardCount1h int64 `json:"blocked_forward_count_1h"`

	// Chains carries the dynamic counter view: every nft chain with
	// its per-rule counters nested. Served by /api/firewall/counters.
	Chains []FirewallChain `json:"chains"`

	// UPnPLeases are active port mappings derived from the inet/miniupnpd
	// nft table (miniupnpd has no on-disk lease file in this config).
	UPnPLeases []UPnPLease `json:"upnp_leases"`
}

// PortForward mirrors router.portForwards[].
type PortForward struct {
	Protocol     string `json:"protocol"`
	ExternalPort int    `json:"external_port"`
	Destination  string `json:"destination"`
}

// PBR bundles the three PBR rule kinds the dashboard surfaces.
// `pooled_rules` reuses Pool topology so the frontend can render
// which clients feed which pool.
type PBR struct {
	SourceRules []PBRSourceRule `json:"source_rules"`
	DomainRules []PBRDomainRule `json:"domain_rules"`
	PooledRules []PBRPooledRule `json:"pooled_rules"`
}

type PBRSourceRule struct {
	Sources []string `json:"sources"`
	Tunnel  string   `json:"tunnel"`
}

type PBRDomainRule struct {
	Tunnel  string   `json:"tunnel"`
	Domains []string `json:"domains"`
}

type PBRPooledRule struct {
	Sources []string `json:"sources"`
	Pool    string   `json:"pool"`
}

// FirewallChain carries its per-rule counters nested — the /counters
// endpoint serves `{chains: [...]}` exactly like this.
type FirewallChain struct {
	Family   string        `json:"family"`
	Table    string        `json:"table"`
	Name     string        `json:"name"`
	Type     string        `json:"type"`
	Hook     string        `json:"hook"`
	Priority int           `json:"priority"`
	Policy   string        `json:"policy"`
	Handle   int           `json:"handle"`
	Counters []RuleCounter `json:"counters"`
}

// RuleCounter pairs one nft counter expression with the rule handle
// that owns it and the comment on that rule.
type RuleCounter struct {
	Handle  int    `json:"handle"`
	Comment string `json:"comment,omitempty"`
	Packets int64  `json:"packets"`
	Bytes   int64  `json:"bytes"`
}

// UPnPLease is one active port mapping established by miniupnpd.
// Named "lease" in the API surface (§7.4) even though miniupnpd
// can't attach an explicit TTL in this config — the nft rule's
// existence is the lease.
type UPnPLease struct {
	Protocol     string `json:"protocol"`
	ExternalPort int    `json:"external_port"`
	InternalAddr string `json:"internal_addr"`
	InternalPort int    `json:"internal_port"`
	Description  string `json:"description,omitempty"`
}
```

- [ ] **Step 5: Create `model/qos.go`**

```go
package model

// QoS is the snapshot served by /api/qos. Egress is CAKE on the WAN
// physical interface; Ingress is HTB+fq_codel on the WAN ingress IFB.
type QoS struct {
	Egress  *QdiscStats `json:"wan_egress,omitempty"`
	Ingress *QdiscStats `json:"wan_ingress,omitempty"`
}

// QdiscStats is the unified shape both qdiscs serialize to. Fields
// not relevant for a given qdisc kind stay zero.
type QdiscStats struct {
	Kind         string    `json:"kind"`
	BandwidthBps int64     `json:"bandwidth_bps"`
	SentBytes    int64     `json:"sent_bytes"`
	SentPackets  int64     `json:"sent_packets"`
	Dropped      int64     `json:"dropped"`
	Overlimits   int64     `json:"overlimits"`
	Requeues     int64     `json:"requeues"`
	BacklogBytes int64     `json:"backlog_bytes"`
	BacklogPkts  int64     `json:"backlog_pkts"`
	Tins         []CAKETin `json:"tins,omitempty"`
	NewFlowCount  int64    `json:"new_flow_count,omitempty"`
	OldFlowsLen   int64    `json:"old_flows_len,omitempty"`
	NewFlowsLen   int64    `json:"new_flows_len,omitempty"`
	ECNMark       int64    `json:"ecn_mark,omitempty"`
	DropOverlimit int64    `json:"drop_overlimit,omitempty"`
}

// CAKETin is one CAKE traffic class.
type CAKETin struct {
	Name         string `json:"name"`
	ThreshKbit   int64  `json:"thresh_kbit"`
	TargetUs     int64  `json:"target_us"`
	IntervalUs   int64  `json:"interval_us"`
	PeakDelayUs  int64  `json:"peak_delay_us"`
	AvgDelayUs   int64  `json:"avg_delay_us"`
	BacklogBytes int64  `json:"backlog_bytes"`
	Packets      int64  `json:"packets"`
	Bytes        int64  `json:"bytes"`
	Drops        int64  `json:"drops"`
	Marks        int64  `json:"marks"`
}
```

- [ ] **Step 6: Run topology tests + verify all packages compile**

```bash
cd modules/dashboard/backend && go test ./internal/topology/... -v && go build ./internal/model/...
```

Expected: topology tests pass; `go build` produces no output.

- [ ] **Step 7: Regenerate the dashboard config on the live router to verify the nix module emits valid JSON**

(Optional for subagents — the CI `nix flake check` catches most shape errors, but a deploy-time round-trip is the truest validation.)

```bash
ssh root@192.168.1.1 "cat /run/*/dashboard-config.json | python3 -c 'import json,sys; d=json.load(sys.stdin); print(sorted(d.keys()))'"
```

Expected output contains `'port_forwards'`, `'pbr_source_rules'`, `'pbr_domain_rules'` (after the next nixos-rebuild; skip this verify step if rebuild happens only at Task 18).

- [ ] **Step 8: Commit**

```bash
git add modules/dashboard/default.nix \
        modules/dashboard/backend/internal/topology/topology.go \
        modules/dashboard/backend/internal/topology/topology_test.go \
        modules/dashboard/backend/internal/model/firewall.go \
        modules/dashboard/backend/internal/model/qos.go
git commit -m "dashboard: expose port forwards + pbr in topology; add firewall/qos models"
```

---

## Task 4: State methods for firewall and qos

**Files:**
- Modify: `modules/dashboard/backend/internal/state/state.go`
- Modify: `modules/dashboard/backend/internal/state/state_test.go`

**Why this design:** Same `Set*`/`Snapshot*` pattern as every other section (`SetTraffic`, `SetTunnels`, etc.). Defensive deep copy on both write and read so callers can mutate freely.

- [ ] **Step 1: Add fields + methods to `state.go`**

In `modules/dashboard/backend/internal/state/state.go`, add to the `State` struct (next to the other section fields):

```go
	firewall        model.Firewall
	firewallUpdated time.Time

	qos        model.QoS
	qosUpdated time.Time
```

Then append these methods at the end of the file (after the existing `copyAdguard` helper):

```go
// --- Firewall ---

// SetFirewall replaces the cached firewall snapshot with a defensive copy.
func (s *State) SetFirewall(v model.Firewall) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.firewall = copyFirewall(v)
	s.firewallUpdated = time.Now()
}

// SnapshotFirewall returns a defensive copy of the firewall snapshot
// and the section's update time.
func (s *State) SnapshotFirewall() (model.Firewall, time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return copyFirewall(s.firewall), s.firewallUpdated
}

// --- QoS ---

// SetQoS replaces the cached QoS snapshot with a defensive copy.
func (s *State) SetQoS(v model.QoS) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.qos = copyQoS(v)
	s.qosUpdated = time.Now()
}

// SnapshotQoS returns a defensive copy of the QoS snapshot and the
// section's update time.
func (s *State) SnapshotQoS() (model.QoS, time.Time) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return copyQoS(s.qos), s.qosUpdated
}

func copyFirewall(src model.Firewall) model.Firewall {
	dst := model.Firewall{
		BlockedForwardCount1h: src.BlockedForwardCount1h,
	}
	if src.PortForwards != nil {
		dst.PortForwards = make([]model.PortForward, len(src.PortForwards))
		copy(dst.PortForwards, src.PortForwards)
	}
	dst.PBR = copyPBR(src.PBR)
	if src.AllowedMACs != nil {
		dst.AllowedMACs = make([]string, len(src.AllowedMACs))
		copy(dst.AllowedMACs, src.AllowedMACs)
	}
	if src.Chains != nil {
		dst.Chains = make([]model.FirewallChain, len(src.Chains))
		for i, c := range src.Chains {
			dst.Chains[i] = c
			if c.Counters != nil {
				dst.Chains[i].Counters = make([]model.RuleCounter, len(c.Counters))
				copy(dst.Chains[i].Counters, c.Counters)
			}
		}
	}
	if src.UPnPLeases != nil {
		dst.UPnPLeases = make([]model.UPnPLease, len(src.UPnPLeases))
		copy(dst.UPnPLeases, src.UPnPLeases)
	}
	return dst
}

func copyPBR(src model.PBR) model.PBR {
	dst := model.PBR{}
	if src.SourceRules != nil {
		dst.SourceRules = make([]model.PBRSourceRule, len(src.SourceRules))
		for i, r := range src.SourceRules {
			dst.SourceRules[i] = r
			if r.Sources != nil {
				dst.SourceRules[i].Sources = append([]string(nil), r.Sources...)
			}
		}
	}
	if src.DomainRules != nil {
		dst.DomainRules = make([]model.PBRDomainRule, len(src.DomainRules))
		for i, r := range src.DomainRules {
			dst.DomainRules[i] = r
			if r.Domains != nil {
				dst.DomainRules[i].Domains = append([]string(nil), r.Domains...)
			}
		}
	}
	if src.PooledRules != nil {
		dst.PooledRules = make([]model.PBRPooledRule, len(src.PooledRules))
		for i, r := range src.PooledRules {
			dst.PooledRules[i] = r
			if r.Sources != nil {
				dst.PooledRules[i].Sources = append([]string(nil), r.Sources...)
			}
		}
	}
	return dst
}

func copyQoS(src model.QoS) model.QoS {
	dst := model.QoS{}
	if src.Egress != nil {
		eg := *src.Egress
		if src.Egress.Tins != nil {
			eg.Tins = make([]model.CAKETin, len(src.Egress.Tins))
			copy(eg.Tins, src.Egress.Tins)
		}
		dst.Egress = &eg
	}
	if src.Ingress != nil {
		in := *src.Ingress
		if src.Ingress.Tins != nil {
			in.Tins = make([]model.CAKETin, len(src.Ingress.Tins))
			copy(in.Tins, src.Ingress.Tins)
		}
		dst.Ingress = &in
	}
	return dst
}
```

- [ ] **Step 2: Add tests in `state_test.go`**

Append:

```go
func TestFirewallRoundTrip(t *testing.T) {
	s := New()
	in := model.Firewall{
		PortForwards: []model.PortForward{{Protocol: "tcp", ExternalPort: 35978, Destination: "192.168.20.6:32400"}},
		PBR: model.PBR{
			SourceRules: []model.PBRSourceRule{{Sources: []string{"192.168.1.225"}, Tunnel: "wg_sw"}},
			DomainRules: []model.PBRDomainRule{{Tunnel: "wg_sw", Domains: []string{"example.com"}}},
			PooledRules: []model.PBRPooledRule{{Sources: []string{"192.168.1.10"}, Pool: "all"}},
		},
		AllowedMACs:           []string{"aa:bb:cc:dd:ee:ff"},
		BlockedForwardCount1h: 42,
		Chains: []model.FirewallChain{{
			Family: "inet", Table: "filter", Name: "input", Hook: "input", Policy: "drop",
			Counters: []model.RuleCounter{{Handle: 16, Packets: 51139, Bytes: 19119175}},
		}},
		UPnPLeases: []model.UPnPLease{{Protocol: "tcp", ExternalPort: 35978, InternalAddr: "192.168.20.6", InternalPort: 32400, Description: "plex/0"}},
	}
	s.SetFirewall(in)
	out, ts := s.SnapshotFirewall()
	if ts.IsZero() {
		t.Error("SnapshotFirewall ts should be non-zero")
	}
	if len(out.PortForwards) != 1 || out.PortForwards[0].ExternalPort != 35978 {
		t.Errorf("PortForwards = %+v", out.PortForwards)
	}
	if len(out.PBR.SourceRules) != 1 || out.PBR.SourceRules[0].Tunnel != "wg_sw" {
		t.Errorf("PBR.SourceRules = %+v", out.PBR.SourceRules)
	}
	if out.BlockedForwardCount1h != 42 {
		t.Errorf("BlockedForwardCount1h = %d, want 42", out.BlockedForwardCount1h)
	}
	if len(out.Chains) != 1 || len(out.Chains[0].Counters) != 1 || out.Chains[0].Counters[0].Bytes != 19119175 {
		t.Errorf("Chains = %+v", out.Chains)
	}
	if len(out.UPnPLeases) != 1 || out.UPnPLeases[0].InternalAddr != "192.168.20.6" {
		t.Errorf("UPnPLeases = %+v", out.UPnPLeases)
	}
	// Mutate output → original cache should be untouched.
	out.Chains[0].Counters[0].Bytes = 999
	out.PBR.SourceRules[0].Sources[0] = "9.9.9.9"
	out2, _ := s.SnapshotFirewall()
	if out2.Chains[0].Counters[0].Bytes != 19119175 {
		t.Errorf("chain counter mutation leaked: %d", out2.Chains[0].Counters[0].Bytes)
	}
	if out2.PBR.SourceRules[0].Sources[0] != "192.168.1.225" {
		t.Errorf("pbr source mutation leaked: %q", out2.PBR.SourceRules[0].Sources[0])
	}
}

func TestQoSRoundTrip(t *testing.T) {
	s := New()
	eg := model.QdiscStats{Kind: "cake", BandwidthBps: 100_000_000, SentBytes: 100, Tins: []model.CAKETin{{Name: "Bulk"}}}
	in := model.QdiscStats{Kind: "htb+fq_codel", SentBytes: 200, ECNMark: 5}
	s.SetQoS(model.QoS{Egress: &eg, Ingress: &in})
	out, ts := s.SnapshotQoS()
	if ts.IsZero() {
		t.Error("SnapshotQoS ts should be non-zero")
	}
	if out.Egress == nil || out.Egress.Kind != "cake" || len(out.Egress.Tins) != 1 {
		t.Errorf("Egress = %+v", out.Egress)
	}
	if out.Ingress == nil || out.Ingress.Kind != "htb+fq_codel" || out.Ingress.ECNMark != 5 {
		t.Errorf("Ingress = %+v", out.Ingress)
	}
	// Mutate output → original cache should be untouched.
	out.Egress.Tins[0].Name = "X"
	out2, _ := s.SnapshotQoS()
	if out2.Egress.Tins[0].Name != "Bulk" {
		t.Error("egress tin mutation leaked")
	}
}
```

- [ ] **Step 3: Run tests**

```bash
cd modules/dashboard/backend && go test ./internal/state/... -run 'Firewall|QoS' -v
```

Expected: both new tests pass; existing tests still pass.

- [ ] **Step 4: Commit**

```bash
git add modules/dashboard/backend/internal/state/state.go modules/dashboard/backend/internal/state/state_test.go
git commit -m "dashboard: add firewall and qos sections to state cache"
```

---

## Task 5: Firewall collector

**Files:**
- Create: `modules/dashboard/backend/internal/collector/firewall.go`
- Create: `modules/dashboard/backend/internal/collector/firewall_test.go`

**Why this design:** Single collector at medium tier (5s) does one `nft --json list ruleset` invocation, derives Chains+Counters+UPnP leases from the same parse, and merges topology-sourced static fields (`port_forwards`, `pbr`, `allowed_macs`) into one `Firewall` snapshot. A per-collector ring buffer tracks the total forward-chain drop counter every tick so `BlockedForwardCount1h` can be published as the delta against the oldest sample ≥60 minutes back. The collector reads topology *every* tick (cheap — topology is in-memory) so a nixos-rebuild that updates the config propagates without a dashboard restart.

- [ ] **Step 1: Write failing tests**

Create `modules/dashboard/backend/internal/collector/firewall_test.go`:

```go
package collector

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/model"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/state"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/topology"
)

func newTestTopology() *topology.Topology {
	return &topology.Topology{
		AllowedMACs: []string{"aa:bb:cc:dd:ee:ff"},
		PortForwards: []topology.PortForward{
			{Protocol: "tcp", ExternalPort: 35978, Destination: "192.168.20.6:32400"},
		},
		PBRSourceRules: []topology.PBRSourceRule{
			{Sources: []string{"192.168.1.225"}, Tunnel: "wg_sw"},
		},
		PBRDomainRules: []topology.PBRDomainRule{
			{Tunnel: "wg_sw", Domains: []string{"example.com"}},
		},
		PooledRules: []topology.PooledRule{
			{Sources: []string{"192.168.1.10"}, Pool: "all"},
		},
	}
}

func TestFirewallCollectorPopulatesState(t *testing.T) {
	st := state.New()
	stub := func(_ context.Context, _ ...string) ([]byte, error) {
		return []byte(`{"nftables":[
			{"chain":{"family":"inet","table":"filter","name":"input","handle":1,"type":"filter","hook":"input","prio":0,"policy":"drop"}},
			{"chain":{"family":"inet","table":"filter","name":"forward","handle":2,"type":"filter","hook":"forward","prio":0,"policy":"drop"}},
			{"rule":{"family":"inet","table":"filter","chain":"input","handle":16,"expr":[{"counter":{"packets":42,"bytes":1024}},{"drop":null}]}},
			{"rule":{"family":"inet","table":"filter","chain":"forward","handle":20,"expr":[{"counter":{"packets":100,"bytes":4096}},{"drop":null}]}}
		]}`), nil
	}
	c := NewFirewall(FirewallOpts{State: st, Topology: newTestTopology(), Run: stub})
	if err := c.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	got, ts := st.SnapshotFirewall()
	if ts.IsZero() {
		t.Error("ts zero")
	}
	if len(got.PortForwards) != 1 || got.PortForwards[0].ExternalPort != 35978 {
		t.Errorf("PortForwards = %+v", got.PortForwards)
	}
	if len(got.PBR.SourceRules) != 1 || got.PBR.SourceRules[0].Tunnel != "wg_sw" {
		t.Errorf("PBR.SourceRules = %+v", got.PBR.SourceRules)
	}
	if len(got.PBR.PooledRules) != 1 || got.PBR.PooledRules[0].Pool != "all" {
		t.Errorf("PBR.PooledRules = %+v", got.PBR.PooledRules)
	}
	if len(got.AllowedMACs) != 1 || got.AllowedMACs[0] != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("AllowedMACs = %+v", got.AllowedMACs)
	}
	if len(got.Chains) != 2 {
		t.Errorf("Chains = %+v", got.Chains)
	}
	var forward model.FirewallChain
	for _, ch := range got.Chains {
		if ch.Name == "forward" {
			forward = ch
		}
	}
	if len(forward.Counters) != 1 || forward.Counters[0].Bytes != 4096 {
		t.Errorf("forward chain counters = %+v", forward.Counters)
	}
	if c.Tier() != Medium {
		t.Errorf("Tier = %v, want Medium", c.Tier())
	}
	if c.Name() != "firewall" {
		t.Errorf("Name = %q, want firewall", c.Name())
	}
}

func TestFirewallCollectorBlockedForwardCount1h(t *testing.T) {
	// Inject a fake clock and nft stub whose forward-drop counter
	// climbs over several ticks; verify the 1h delta equals
	// current - oldest-sample-at-or-before-1h-ago.
	st := state.New()
	var counter int64
	stub := func(_ context.Context, _ ...string) ([]byte, error) {
		return []byte(fmt.Sprintf(`{"nftables":[
			{"chain":{"family":"inet","table":"filter","name":"forward","handle":1,"type":"filter","hook":"forward","prio":0,"policy":"drop"}},
			{"rule":{"family":"inet","table":"filter","chain":"forward","handle":2,"expr":[{"counter":{"packets":%d,"bytes":0}},{"drop":null}]}}
		]}`, counter)), nil
	}
	now := time.Unix(1_700_000_000, 0)
	clock := func() time.Time { return now }
	c := NewFirewall(FirewallOpts{State: st, Topology: &topology.Topology{}, Run: stub, Clock: clock})

	// t=0min:   total forward-drops = 10
	counter = 10
	if err := c.Run(context.Background()); err != nil {
		t.Fatalf("run1: %v", err)
	}
	// t=65min:  total forward-drops = 50 → 1h delta should be 50 - 10 = 40
	now = now.Add(65 * time.Minute)
	counter = 50
	if err := c.Run(context.Background()); err != nil {
		t.Fatalf("run2: %v", err)
	}
	got, _ := st.SnapshotFirewall()
	if got.BlockedForwardCount1h != 40 {
		t.Errorf("BlockedForwardCount1h = %d, want 40", got.BlockedForwardCount1h)
	}
}
```

- [ ] **Step 2: Run tests, expect failure**

```bash
cd modules/dashboard/backend && go test ./internal/collector/... -run TestFirewallCollector
```

Expected: FAIL with "undefined: NewFirewall" (or "undefined: FirewallOpts").

- [ ] **Step 3: Implement collector**

Create `modules/dashboard/backend/internal/collector/firewall.go`:

```go
package collector

import (
	"context"
	"sync"
	"time"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/model"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/nft"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/state"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/topology"
)

// FirewallOpts configures the Firewall collector.
type FirewallOpts struct {
	State    *state.State
	Topology *topology.Topology // static config source for port_forwards / pbr / allowed_macs
	Run      nft.Runner         // nil → DefaultRunner
	Clock    func() time.Time   // nil → time.Now
}

// Firewall is a medium-tier collector that runs `nft --json list ruleset`,
// projects the parse into model.Firewall (chains-with-nested-counters +
// UPnP leases), merges the static topology config (port forwards / PBR /
// allowlist), and rolls the 1h blocked-forward counter forward via an
// in-memory ring.
type Firewall struct {
	opts FirewallOpts

	mu      sync.Mutex
	samples []forwardDropSample // trimmed to 1h window on each Run
}

type forwardDropSample struct {
	ts    time.Time
	count int64
}

// NewFirewall creates a Firewall collector.
func NewFirewall(opts FirewallOpts) *Firewall {
	if opts.Run == nil {
		opts.Run = nft.DefaultRunner
	}
	if opts.Clock == nil {
		opts.Clock = time.Now
	}
	return &Firewall{opts: opts}
}

func (*Firewall) Name() string { return "firewall" }
func (*Firewall) Tier() Tier   { return Medium }

func (c *Firewall) Run(ctx context.Context) error {
	r, err := nft.Collect(ctx, c.opts.Run)
	if err != nil {
		return err
	}

	// Index counters by (family,table,chain,handle) so we can nest
	// them under their owning chain.
	type key struct {
		family, table, chain string
		handle               int
	}
	countersByRule := map[key][]model.RuleCounter{}
	forwardDropTotal := int64(0)
	for _, ct := range r.Counters {
		k := key{ct.Family, ct.Table, ct.ChainName, ct.Handle}
		countersByRule[k] = append(countersByRule[k], model.RuleCounter{
			Handle: ct.Handle, Comment: ct.Comment,
			Packets: ct.Packets, Bytes: ct.Bytes,
		})
		if ct.Family == "inet" && ct.Table == "filter" && ct.ChainName == "forward" {
			forwardDropTotal += ct.Packets
		}
	}

	// Append-or-prune the forward-drop ring under lock. Oldest sample
	// at-or-before (now - 1h) is our baseline; 1h delta is current total
	// minus that baseline's recorded count.
	now := c.opts.Clock()
	oneHourAgo := now.Add(-time.Hour)
	c.mu.Lock()
	c.samples = append(c.samples, forwardDropSample{ts: now, count: forwardDropTotal})
	// Keep at most one sample older than 1h (our baseline).
	var trimmed []forwardDropSample
	var baseline *forwardDropSample
	for i := range c.samples {
		s := c.samples[i]
		if s.ts.Before(oneHourAgo) {
			b := s
			baseline = &b
			continue
		}
		trimmed = append(trimmed, s)
	}
	if baseline != nil {
		trimmed = append([]forwardDropSample{*baseline}, trimmed...)
	}
	c.samples = trimmed
	var delta int64
	if len(c.samples) > 1 {
		delta = c.samples[len(c.samples)-1].count - c.samples[0].count
		if delta < 0 {
			delta = 0 // counter reset (nftables reload) — start over.
			c.samples = []forwardDropSample{c.samples[len(c.samples)-1]}
		}
	}
	c.mu.Unlock()

	out := model.Firewall{
		BlockedForwardCount1h: delta,
		Chains:                make([]model.FirewallChain, 0, len(r.Chains)),
		UPnPLeases:            make([]model.UPnPLease, 0, len(r.UPnPMappings)),
	}

	// Merge topology-sourced static fields.
	if topo := c.opts.Topology; topo != nil {
		out.AllowedMACs = append([]string(nil), topo.AllowedMACs...)
		for _, pf := range topo.PortForwards {
			out.PortForwards = append(out.PortForwards, model.PortForward{
				Protocol:     pf.Protocol,
				ExternalPort: pf.ExternalPort,
				Destination:  pf.Destination,
			})
		}
		for _, r := range topo.PBRSourceRules {
			out.PBR.SourceRules = append(out.PBR.SourceRules, model.PBRSourceRule{
				Sources: append([]string(nil), r.Sources...),
				Tunnel:  r.Tunnel,
			})
		}
		for _, r := range topo.PBRDomainRules {
			out.PBR.DomainRules = append(out.PBR.DomainRules, model.PBRDomainRule{
				Tunnel:  r.Tunnel,
				Domains: append([]string(nil), r.Domains...),
			})
		}
		for _, r := range topo.PooledRules {
			out.PBR.PooledRules = append(out.PBR.PooledRules, model.PBRPooledRule{
				Sources: append([]string(nil), r.Sources...),
				Pool:    r.Pool,
			})
		}
	}

	// Chains + nested counters.
	for _, ch := range r.Chains {
		mc := model.FirewallChain{
			Family: ch.Family, Table: ch.Table, Name: ch.Name, Type: ch.Type,
			Hook: ch.Hook, Priority: ch.Priority, Policy: ch.Policy,
			Handle: ch.Handle,
		}
		// Attach every per-rule counter belonging to this chain. We don't
		// know the chain's rule handles directly, so we filter countersByRule
		// by matching chain identity.
		for k, cs := range countersByRule {
			if k.family == ch.Family && k.table == ch.Table && k.chain == ch.Name {
				mc.Counters = append(mc.Counters, cs...)
			}
		}
		out.Chains = append(out.Chains, mc)
	}

	// UPnP leases derived from the miniupnpd table.
	for _, m := range r.UPnPMappings {
		out.UPnPLeases = append(out.UPnPLeases, model.UPnPLease{
			Protocol: m.Protocol, ExternalPort: m.ExternalPort,
			InternalAddr: m.InternalAddr, InternalPort: m.InternalPort,
			Description: m.Description,
		})
	}

	c.opts.State.SetFirewall(out)
	return nil
}
```
```

- [ ] **Step 4: Run tests, expect green**

```bash
cd modules/dashboard/backend && go test ./internal/collector/... -run TestFirewallCollector -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add modules/dashboard/backend/internal/collector/firewall.go modules/dashboard/backend/internal/collector/firewall_test.go
git commit -m "dashboard: add firewall collector (medium-tier nft snapshot)"
```

---

## Task 6: QoS collector

**Files:**
- Create: `modules/dashboard/backend/internal/collector/qos.go`
- Create: `modules/dashboard/backend/internal/collector/qos_test.go`

- [ ] **Step 1: Write failing test**

Create `modules/dashboard/backend/internal/collector/qos_test.go`:

```go
package collector

import (
	"context"
	"testing"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/state"
)

func TestQoSCollectorPopulatesState(t *testing.T) {
	st := state.New()
	stub := func(_ context.Context, args ...string) ([]byte, error) {
		// args = ["-s", "qdisc", "show", "dev", iface]
		var iface string
		for i, a := range args {
			if a == "dev" && i+1 < len(args) {
				iface = args[i+1]
			}
		}
		switch iface {
		case "eth1":
			return []byte("qdisc cake 8003: root refcnt 2 bandwidth 100Mbit\n Sent 1000 bytes 10 pkt (dropped 1, overlimits 2 requeues 0) \n backlog 0b 0p requeues 0\n"), nil
		case "ifb4eth1":
			return []byte("qdisc htb 1: root refcnt 2\n Sent 2000 bytes 20 pkt (dropped 3, overlimits 0 requeues 0) \nqdisc fq_codel 8004: parent 1:1\n Sent 2000 bytes 20 pkt (dropped 3, overlimits 0 requeues 0) \n  maxpacket 1500 drop_overlimit 0 new_flow_count 5 ecn_mark 1\n"), nil
		}
		return nil, nil
	}

	c := NewQoS(QoSOpts{State: st, Run: stub, EgressInterface: "eth1", IngressInterface: "ifb4eth1"})
	if err := c.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	got, _ := st.SnapshotQoS()
	if got.Egress == nil || got.Egress.SentBytes != 1000 {
		t.Errorf("Egress = %+v", got.Egress)
	}
	if got.Ingress == nil || got.Ingress.SentBytes != 2000 || got.Ingress.ECNMark != 1 {
		t.Errorf("Ingress = %+v", got.Ingress)
	}
	if c.Tier() != Medium {
		t.Errorf("Tier = %v, want Medium", c.Tier())
	}
}
```

- [ ] **Step 2: Run tests, expect failure**

```bash
cd modules/dashboard/backend && go test ./internal/collector/... -run TestQoSCollector
```

Expected: FAIL with "undefined: NewQoS".

- [ ] **Step 3: Implement collector**

Create `modules/dashboard/backend/internal/collector/qos.go`:

```go
package collector

import (
	"context"
	"log/slog"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/model"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/tc"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/state"
)

// QoSOpts configures the QoS collector.
type QoSOpts struct {
	State            *state.State
	Run              tc.Runner // nil → DefaultRunner
	EgressInterface  string    // typically "eth1"
	IngressInterface string    // typically "ifb4eth1"
}

// QoS is a medium-tier collector that snapshots the egress CAKE qdisc
// on the WAN physical interface and the HTB+fq_codel qdisc on its IFB.
// One of either side failing is non-fatal — the other side still gets
// published.
type QoS struct {
	opts QoSOpts
}

// NewQoS creates a QoS collector.
func NewQoS(opts QoSOpts) *QoS {
	if opts.Run == nil {
		opts.Run = tc.DefaultRunner
	}
	return &QoS{opts: opts}
}

func (*QoS) Name() string { return "qos" }
func (*QoS) Tier() Tier   { return Medium }

func (c *QoS) Run(ctx context.Context) error {
	out := model.QoS{}
	if c.opts.EgressInterface != "" {
		eg, err := tc.CollectCAKE(ctx, c.opts.Run, c.opts.EgressInterface)
		if err != nil {
			slog.Warn("qos: egress collect failed", "iface", c.opts.EgressInterface, "err", err)
		} else {
			eq := toModel(eg)
			out.Egress = &eq
		}
	}
	if c.opts.IngressInterface != "" {
		ig, err := tc.CollectHTB(ctx, c.opts.Run, c.opts.IngressInterface)
		if err != nil {
			slog.Warn("qos: ingress collect failed", "iface", c.opts.IngressInterface, "err", err)
		} else {
			iq := toModel(ig)
			out.Ingress = &iq
		}
	}
	c.opts.State.SetQoS(out)
	return nil
}

func toModel(s tc.QdiscStats) model.QdiscStats {
	dst := model.QdiscStats{
		Kind: s.Kind, BandwidthBps: s.BandwidthBps,
		SentBytes: s.SentBytes, SentPackets: s.SentPackets,
		Dropped: s.Dropped, Overlimits: s.Overlimits, Requeues: s.Requeues,
		BacklogBytes: s.BacklogBytes, BacklogPkts: s.BacklogPkts,
		NewFlowCount:  s.NewFlowCount,
		OldFlowsLen:   s.OldFlowsLen,
		NewFlowsLen:   s.NewFlowsLen,
		ECNMark:       s.ECNMark,
		DropOverlimit: s.DropOverlimit,
	}
	for _, t := range s.Tins {
		dst.Tins = append(dst.Tins, model.CAKETin{
			Name: t.Name, ThreshKbit: t.ThreshKbit,
			TargetUs: t.TargetUs, IntervalUs: t.IntervalUs,
			PeakDelayUs: t.PeakDelayUs, AvgDelayUs: t.AvgDelayUs,
			BacklogBytes: t.BacklogBytes,
			Packets: t.Packets, Bytes: t.Bytes,
			Drops: t.Drops, Marks: t.Marks,
		})
	}
	return dst
}
```

- [ ] **Step 4: Run tests, expect green**

```bash
cd modules/dashboard/backend && go test ./internal/collector/... -run TestQoSCollector -v
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add modules/dashboard/backend/internal/collector/qos.go modules/dashboard/backend/internal/collector/qos_test.go
git commit -m "dashboard: add qos collector — CAKE egress + HTB ingress snapshot"
```

---

## Task 7: Handlers (firewall + upnp + qos)

**Files:**
- Create: `modules/dashboard/backend/internal/server/handlers_firewall.go`
- Create: `modules/dashboard/backend/internal/server/handlers_qos.go`
- Modify: `modules/dashboard/backend/internal/server/handlers_test.go`

**Why this design:** Handlers carve three distinct projections from the single `Firewall` snapshot. Each endpoint uses its own stale window so user-visible "stale" badges don't flicker — `/api/firewall/rules` is tolerant (its data barely changes), `/api/firewall/counters` is tight (counters tick fast). Windows map to 2× the frontend poll interval from spec §8.4 so a collector miss exactly at the next poll still reads fresh, but a sustained collector outage immediately flips stale. `/api/qos` tracks the medium-tier cadence because tc stats tick and the frontend polls at 5s.

- [ ] **Step 1: Create `handlers_firewall.go`**

```go
package server

import (
	"net/http"
	"time"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/envelope"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/state"
)

// Stale-window constants derived from spec §8.4 frontend poll cadence × 2.
// Using per-endpoint windows (rather than one tier-wide value) keeps the
// stale badge honest for data that barely changes (rules) vs data that
// ticks fast (counters).
const (
	firewallRulesStaleAfter    = 60 * time.Second // spec: 30 s poll × 2
	firewallCountersStaleAfter = 10 * time.Second // spec:  5 s poll × 2
	upnpStaleAfter             = 30 * time.Second // spec: 15 s poll × 2
	qosStaleAfter              = 10 * time.Second // spec:  5 s poll × 2
)

// handleFirewallRules serves the static-ish Firewall projection:
// port forwards, PBR rules, allowed MACs, and the rolled-up 1h
// forward-drop count. Spec §7.4: "{port_forwards, pbr, allowed_macs,
// blocked_forward_count_1h}".
func handleFirewallRules(st *state.State) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		fw, updated := st.SnapshotFirewall()
		stale := state.IsStale(updated, firewallRulesStaleAfter/2)
		body := map[string]any{
			"port_forwards":            fw.PortForwards,
			"pbr":                      fw.PBR,
			"allowed_macs":             fw.AllowedMACs,
			"blocked_forward_count_1h": fw.BlockedForwardCount1h,
		}
		envelope.WriteJSON(w, http.StatusOK, body, updated, stale)
	}
}

// handleFirewallCounters serves the dynamic counters view —
// {chains: [ {family, table, name, hook, policy, counters: [{handle, comment, packets, bytes}]} ]}.
// Spec §7.4: "{chains: [...]}".
func handleFirewallCounters(st *state.State) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		fw, updated := st.SnapshotFirewall()
		stale := state.IsStale(updated, firewallCountersStaleAfter/2)
		body := struct {
			Chains any `json:"chains"`
		}{Chains: fw.Chains}
		envelope.WriteJSON(w, http.StatusOK, body, updated, stale)
	}
}

// handleUPnP serves active UPnP mappings extracted from the
// inet/miniupnpd table. Spec §7.4: "{leases: [...]}".
func handleUPnP(st *state.State) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		fw, updated := st.SnapshotFirewall()
		stale := state.IsStale(updated, upnpStaleAfter/2)
		body := struct {
			Leases any `json:"leases"`
		}{Leases: fw.UPnPLeases}
		envelope.WriteJSON(w, http.StatusOK, body, updated, stale)
	}
}
```

Note: `IsStale(updated, window/2)` gives the correct semantic because `IsStale` fires when `time.Since(updated) > 2*interval` (see `internal/state/state.go`). Passing `window/2` means "fire when older than window" — matching our intended stale windows above.

- [ ] **Step 2: Create `handlers_qos.go`**

```go
package server

import (
	"net/http"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/envelope"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/state"
)

func handleQoS(st *state.State) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		data, updated := st.SnapshotQoS()
		stale := state.IsStale(updated, qosStaleAfter/2)
		envelope.WriteJSON(w, http.StatusOK, data, updated, stale)
	}
}
```

- [ ] **Step 3: Add handler tests**

In `modules/dashboard/backend/internal/server/handlers_test.go` add at the end:

```go
func TestFirewallRulesHandler(t *testing.T) {
	st := state.New()
	st.SetFirewall(model.Firewall{
		PortForwards:          []model.PortForward{{Protocol: "tcp", ExternalPort: 35978, Destination: "192.168.20.6:32400"}},
		PBR:                   model.PBR{SourceRules: []model.PBRSourceRule{{Sources: []string{"192.168.1.225"}, Tunnel: "wg_sw"}}},
		AllowedMACs:           []string{"aa:bb:cc:dd:ee:ff"},
		BlockedForwardCount1h: 7,
	})
	h := New(&config.Config{}, st, &topology.Topology{})
	req := httptest.NewRequest(http.MethodGet, "/api/firewall/rules", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var env struct {
		Data struct {
			PortForwards          []map[string]any `json:"port_forwards"`
			PBR                   map[string]any   `json:"pbr"`
			AllowedMACs           []string         `json:"allowed_macs"`
			BlockedForwardCount1h float64          `json:"blocked_forward_count_1h"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(env.Data.PortForwards) != 1 || env.Data.PortForwards[0]["external_port"].(float64) != 35978 {
		t.Errorf("port_forwards = %+v", env.Data.PortForwards)
	}
	if env.Data.PBR == nil {
		t.Fatal("pbr missing")
	}
	if len(env.Data.AllowedMACs) != 1 || env.Data.AllowedMACs[0] != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("allowed_macs = %+v", env.Data.AllowedMACs)
	}
	if env.Data.BlockedForwardCount1h != 7 {
		t.Errorf("blocked_forward_count_1h = %v, want 7", env.Data.BlockedForwardCount1h)
	}
}

func TestFirewallCountersHandler(t *testing.T) {
	st := state.New()
	st.SetFirewall(model.Firewall{
		Chains: []model.FirewallChain{{
			Family: "inet", Table: "filter", Name: "input", Hook: "input", Policy: "drop",
			Counters: []model.RuleCounter{{Handle: 16, Packets: 100, Bytes: 4096, Comment: "drop"}},
		}},
	})
	h := New(&config.Config{}, st, &topology.Topology{})
	req := httptest.NewRequest(http.MethodGet, "/api/firewall/counters", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var env struct {
		Data struct {
			Chains []struct {
				Name     string `json:"name"`
				Counters []struct {
					Handle  float64 `json:"handle"`
					Bytes   float64 `json:"bytes"`
					Comment string  `json:"comment"`
				} `json:"counters"`
			} `json:"chains"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(env.Data.Chains) != 1 || env.Data.Chains[0].Name != "input" {
		t.Errorf("chains = %+v", env.Data.Chains)
	}
	if len(env.Data.Chains[0].Counters) != 1 || env.Data.Chains[0].Counters[0].Bytes != 4096 {
		t.Errorf("counters = %+v", env.Data.Chains[0].Counters)
	}
}

func TestUPnPHandler(t *testing.T) {
	st := state.New()
	st.SetFirewall(model.Firewall{
		UPnPLeases: []model.UPnPLease{{Protocol: "tcp", ExternalPort: 35978, InternalAddr: "192.168.20.6", InternalPort: 32400, Description: "plex"}},
	})
	h := New(&config.Config{}, st, &topology.Topology{})
	req := httptest.NewRequest(http.MethodGet, "/api/upnp", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var env struct {
		Data struct {
			Leases []map[string]any `json:"leases"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(env.Data.Leases) != 1 || env.Data.Leases[0]["external_port"].(float64) != 35978 {
		t.Errorf("leases = %+v", env.Data.Leases)
	}
}

func TestQoSHandler(t *testing.T) {
	st := state.New()
	eg := model.QdiscStats{Kind: "cake", SentBytes: 1234, BandwidthBps: 100_000_000}
	st.SetQoS(model.QoS{Egress: &eg})
	h := New(&config.Config{}, st, &topology.Topology{})
	req := httptest.NewRequest(http.MethodGet, "/api/qos", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var env struct {
		Data struct {
			Egress *struct {
				Kind      string `json:"kind"`
				SentBytes int64  `json:"sent_bytes"`
			} `json:"wan_egress"`
			Ingress any `json:"wan_ingress"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Data.Egress == nil || env.Data.Egress.Kind != "cake" || env.Data.Egress.SentBytes != 1234 {
		t.Errorf("egress = %+v", env.Data.Egress)
	}
	if env.Data.Ingress != nil {
		t.Errorf("ingress should be nil when not collected, got %+v", env.Data.Ingress)
	}
}
```

(These won't pass yet — Task 8 wires the routes.)

- [ ] **Step 4: Commit handler files (without route wiring yet — tests will pass after Task 8)**

```bash
git add modules/dashboard/backend/internal/server/handlers_firewall.go modules/dashboard/backend/internal/server/handlers_qos.go modules/dashboard/backend/internal/server/handlers_test.go
git commit -m "dashboard: add firewall/upnp/qos HTTP handlers + tests"
```

---

## Task 8: Wire collectors + routes

**Files:**
- Modify: `modules/dashboard/backend/internal/server/server.go`
- Modify: `modules/dashboard/backend/cmd/dashboard/main.go`
- Modify: `modules/dashboard/backend/internal/topology/topology.go` (add IngressIFB derivation)

**Why this design:** The ingress IFB device's name is conventionally `ifb4<wan>` (e.g. `ifb4eth1`). The router config could change this in the future, but today it's hard-coded by the QoS module. We derive it from `topology.WANInterface` rather than wiring a separate config field.

- [ ] **Step 1: Register the new routes in `server.go`**

In `modules/dashboard/backend/internal/server/server.go`, inside `func New(...)`, add these lines after the existing `mux.HandleFunc("GET /api/adguard/querylog", ...)`:

```go
	mux.HandleFunc("GET /api/firewall/rules", handleFirewallRules(st))
	mux.HandleFunc("GET /api/firewall/counters", handleFirewallCounters(st))
	mux.HandleFunc("GET /api/upnp", handleUPnP(st))
	mux.HandleFunc("GET /api/qos", handleQoS(st))
```

- [ ] **Step 2: Wire the new collectors in `cmd/dashboard/main.go`**

In `modules/dashboard/backend/cmd/dashboard/main.go`, inside the `collectors := []collector.Collector{...}` slice (after the existing `NewSystemMedium(...)`), append:

```go
		collector.NewFirewall(collector.FirewallOpts{State: st, Topology: topo}),
		collector.NewQoS(collector.QoSOpts{
			State:            st,
			EgressInterface:  topo.WANInterface,
			IngressInterface: ifbForWAN(topo.WANInterface),
		}),
```

Above `func main()`, add the helper:

```go
// ifbForWAN returns the IFB device name conventionally created by the
// QoS module for ingress shaping on the given WAN interface. Returns
// "" when the WAN interface is unset (dev mode), which the QoS
// collector treats as "skip ingress".
func ifbForWAN(wan string) string {
	if wan == "" {
		return ""
	}
	return "ifb4" + wan
}
```

- [ ] **Step 3: Run all tests**

```bash
cd modules/dashboard/backend && go test ./...
```

Expected: ALL tests pass, including the four new handler tests from Task 7.

- [ ] **Step 4: Run go vet**

```bash
cd modules/dashboard/backend && go vet ./...
```

Expected: no findings.

- [ ] **Step 5: Commit**

```bash
git add modules/dashboard/backend/internal/server/server.go modules/dashboard/backend/cmd/dashboard/main.go
git commit -m "dashboard: register firewall/upnp/qos routes and collectors"
```

---

## Task 9: Frontend API types + query keys

**Files:**
- Modify: `modules/dashboard/frontend/src/lib/api.ts`
- Modify: `modules/dashboard/frontend/src/lib/query-keys.ts`

- [ ] **Step 1: Add types to `api.ts`**

In `modules/dashboard/frontend/src/lib/api.ts`, add these types (place them after the existing `AdguardStats`-related types, before the `fetchEnvelope` function):

```ts
// --- Firewall ---
export type PortForward = {
  protocol: string;
  external_port: number;
  destination: string; // "ip:port"
};
export type PBRSourceRule = { sources: string[]; tunnel: string };
export type PBRDomainRule = { tunnel: string; domains: string[] };
export type PBRPooledRule = { sources: string[]; pool: string };
export type PBR = {
  source_rules: PBRSourceRule[];
  domain_rules: PBRDomainRule[];
  pooled_rules: PBRPooledRule[];
};
export type FirewallRules = {
  port_forwards: PortForward[];
  pbr: PBR;
  allowed_macs: string[];
  blocked_forward_count_1h: number;
};
export type RuleCounter = {
  handle: number;
  comment?: string;
  packets: number;
  bytes: number;
};
export type FirewallChain = {
  family: string;
  table: string;
  name: string;
  type: string;
  hook: string;
  priority: number;
  policy: string;
  handle: number;
  counters: RuleCounter[];
};
export type UPnPLease = {
  protocol: string;
  external_port: number;
  internal_addr: string;
  internal_port: number;
  description?: string;
};

// --- QoS ---
export type CAKETin = {
  name: string;
  thresh_kbit: number;
  target_us: number;
  interval_us: number;
  peak_delay_us: number;
  avg_delay_us: number;
  backlog_bytes: number;
  packets: number;
  bytes: number;
  drops: number;
  marks: number;
};
export type QdiscStats = {
  kind: string;
  bandwidth_bps: number;
  sent_bytes: number;
  sent_packets: number;
  dropped: number;
  overlimits: number;
  requeues: number;
  backlog_bytes: number;
  backlog_pkts: number;
  tins?: CAKETin[];
  new_flow_count?: number;
  old_flows_len?: number;
  new_flows_len?: number;
  ecn_mark?: number;
  drop_overlimit?: number;
};
export type QoS = {
  wan_egress?: QdiscStats;
  wan_ingress?: QdiscStats;
};
```

Then add to the `api` object literal:

```ts
  firewallRules: () => fetchEnvelope<FirewallRules>("/api/firewall/rules"),
  firewallCounters: () =>
    fetchEnvelope<{ chains: FirewallChain[] }>("/api/firewall/counters"),
  upnp: () => fetchEnvelope<{ leases: UPnPLease[] }>("/api/upnp"),
  qos: () => fetchEnvelope<QoS>("/api/qos"),
```

- [ ] **Step 2: Add keys to `query-keys.ts`**

Append to the `queryKeys` object:

```ts
  firewallRules: () => ["firewall", "rules"] as const,
  firewallCounters: () => ["firewall", "counters"] as const,
  upnp: () => ["upnp"] as const,
  qos: () => ["qos"] as const,
```

- [ ] **Step 3: Type-check**

```bash
cd modules/dashboard/frontend && npx tsc --noEmit
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add modules/dashboard/frontend/src/lib/api.ts modules/dashboard/frontend/src/lib/query-keys.ts
git commit -m "dashboard: add firewall/upnp/qos API types and query keys"
```

---

## Task 10: VPN Tunnels list page

**Files:**
- Create: `modules/dashboard/frontend/src/pages/VpnTunnels.tsx`

**Why this design:** Single-page list of all WireGuard tunnels. Each row links to the detail page. Per-tunnel cross-reference to active client count comes from `/api/clients` (sum of clients whose `tunnel_conns[fwmark] > 0`). Layout copies `VpnPools.tsx` — error-then-loading guard, combined StaleIndicator across pools+clients.

- [ ] **Step 1: Create the page**

```tsx
import { Link } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { Tunnel, Client } from "@/lib/api";
import { queryKeys } from "@/lib/query-keys";
import { MonoText } from "@/components/MonoText";
import { StatusBadge } from "@/components/StatusBadge";
import { StaleIndicator } from "@/components/StaleIndicator";
import { DataTable, type Column } from "@/components/DataTable";
import { formatBytes, formatDuration } from "@/lib/formatters";

type Row = {
  name: string;
  fwmark: string;
  endpoint: string;
  healthy: boolean;
  handshakeAgo: number;
  rxBytes: number;
  txBytes: number;
  clientCount: number;
};

const columns: Column<Row>[] = [
  {
    key: "name",
    label: "Tunnel",
    render: (r) => (
      <Link
        to={`/vpn/tunnels/${encodeURIComponent(r.name)}`}
        className="text-primary hover:underline"
      >
        <MonoText>{r.name}</MonoText>
      </Link>
    ),
    sortValue: (r) => r.name,
  },
  {
    key: "health",
    label: "Health",
    render: (r) => (
      <StatusBadge kind={r.healthy ? "healthy" : "failed"}>
        {r.healthy ? "healthy" : "down"}
      </StatusBadge>
    ),
    sortValue: (r) => (r.healthy ? 0 : 1),
  },
  {
    key: "fwmark",
    label: "Fwmark",
    render: (r) => <MonoText>{r.fwmark}</MonoText>,
    sortValue: (r) => r.fwmark,
  },
  {
    key: "endpoint",
    label: "Endpoint",
    render: (r) => <MonoText className="text-xs">{r.endpoint || "—"}</MonoText>,
    sortValue: (r) => r.endpoint,
  },
  {
    key: "handshake",
    label: "Last handshake",
    render: (r) =>
      r.handshakeAgo > 0 ? (
        <MonoText>{formatDuration(r.handshakeAgo)} ago</MonoText>
      ) : (
        <span className="text-on-surface-variant">never</span>
      ),
    sortValue: (r) => r.handshakeAgo,
  },
  {
    key: "rx",
    label: "RX",
    render: (r) => <MonoText>{formatBytes(r.rxBytes)}</MonoText>,
    sortValue: (r) => r.rxBytes,
    className: "text-right",
  },
  {
    key: "tx",
    label: "TX",
    render: (r) => <MonoText>{formatBytes(r.txBytes)}</MonoText>,
    sortValue: (r) => r.txBytes,
    className: "text-right",
  },
  {
    key: "clients",
    label: "Routed clients",
    render: (r) => <MonoText>{r.clientCount}</MonoText>,
    sortValue: (r) => r.clientCount,
    className: "text-right",
  },
];

function buildRows(tunnels: Tunnel[], clients: Client[]): Row[] {
  return tunnels.map((t) => {
    let clientCount = 0;
    for (const c of clients) {
      const tc = c.tunnel_conns ?? {};
      if ((tc[t.fwmark] ?? 0) > 0) clientCount++;
    }
    return {
      name: t.name,
      fwmark: t.fwmark,
      endpoint: t.endpoint,
      healthy: t.healthy,
      handshakeAgo: t.latest_handshake_seconds_ago,
      rxBytes: t.rx_bytes,
      txBytes: t.tx_bytes,
      clientCount,
    };
  });
}

export function VpnTunnels() {
  const tunnelsQ = useQuery({
    queryKey: queryKeys.tunnels(),
    queryFn: api.tunnels,
    refetchInterval: 5_000,
  });
  const clientsQ = useQuery({
    queryKey: queryKeys.clients(),
    queryFn: api.clients,
    refetchInterval: 5_000,
  });

  const noData = !tunnelsQ.data || !clientsQ.data;
  if ((tunnelsQ.isError || clientsQ.isError) && noData) {
    return (
      <div className="text-sm text-rose font-mono">
        Failed to load tunnel data — retry shortly.
      </div>
    );
  }
  if (tunnelsQ.isPending || clientsQ.isPending || !tunnelsQ.data || !clientsQ.data) {
    return <div className="text-sm text-on-surface-variant">Loading...</div>;
  }

  const refetchFailed = tunnelsQ.isError || clientsQ.isError;
  const tunnels = tunnelsQ.data.data.tunnels;
  const clients = clientsQ.data.data.clients;
  const rows = buildRows(tunnels, clients);

  const combinedStale =
    (tunnelsQ.data.stale ?? false) || (clientsQ.data.stale ?? false);
  const tunnelsUpdated = tunnelsQ.data.updated_at ?? null;
  const clientsUpdated = clientsQ.data.updated_at ?? null;
  const combinedUpdatedAt =
    tunnelsUpdated && clientsUpdated
      ? tunnelsUpdated < clientsUpdated
        ? tunnelsUpdated
        : clientsUpdated
      : (tunnelsUpdated ?? clientsUpdated);

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold">VPN Tunnels</h1>
        <div className="flex items-center gap-2">
          {refetchFailed && (
            <span className="text-[10px] uppercase tracking-wider font-bold bg-rose/10 text-rose px-2 py-0.5 rounded-sm">
              Refetch failed
            </span>
          )}
          <StaleIndicator stale={combinedStale} updatedAt={combinedUpdatedAt} />
        </div>
      </div>
      {rows.length === 0 ? (
        <p className="text-sm text-on-surface-variant font-mono">
          No tunnels configured.
        </p>
      ) : (
        <DataTable columns={columns} rows={rows} rowKey={(r) => r.name} />
      )}
    </div>
  );
}
```

- [ ] **Step 2: Type-check**

```bash
cd modules/dashboard/frontend && npx tsc --noEmit
```

Expected: no errors. (The page isn't routed yet — Task 16 wires it.)

- [ ] **Step 3: Commit**

```bash
git add modules/dashboard/frontend/src/pages/VpnTunnels.tsx
git commit -m "dashboard: add VPN Tunnels list page"
```

---

## Task 11: VPN Tunnel detail page

**Files:**
- Create: `modules/dashboard/frontend/src/pages/VpnTunnelDetail.tsx`

**Why this design:** Detail page filters `/api/tunnels` by name client-side (no `/api/tunnels/{name}` endpoint exists, and the per-tunnel record is ~200 bytes — server roundtrip is wasted). For per-client routing breakdown we cross-reference `/api/clients` like the pool detail page does. For RX/TX rate sparkline we read `samples_60s` from `/api/traffic` for the tunnel's interface (e.g., `wg_sw`).

- [ ] **Step 1: Create the page**

```tsx
import { Link, useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { Tunnel, Client, Interface } from "@/lib/api";
import { queryKeys } from "@/lib/query-keys";
import { MonoText } from "@/components/MonoText";
import { StatusBadge } from "@/components/StatusBadge";
import { StaleIndicator } from "@/components/StaleIndicator";
import { ClientBadge } from "@/components/ClientBadge";
import { DataTable, type Column } from "@/components/DataTable";
import { Sparkline } from "@/components/Sparkline";
import { formatBytes, formatBps, formatDuration } from "@/lib/formatters";

function StatTile({ label, value }: { label: string; value: string }) {
  return (
    <div className="bg-surface-container rounded-sm p-4">
      <p className="label-xs mb-1">{label}</p>
      <MonoText className="text-lg font-semibold">{value}</MonoText>
    </div>
  );
}

type ClientRow = {
  hostname: string;
  ip: string;
  conn_count: number;
};

const clientColumns: Column<ClientRow>[] = [
  {
    key: "hostname",
    label: "Hostname",
    render: (r) =>
      r.hostname || <span className="text-on-surface-variant">—</span>,
    sortValue: (r) => r.hostname.toLowerCase(),
  },
  {
    key: "ip",
    label: "IP",
    render: (r) => <ClientBadge ip={r.ip} />,
    sortValue: (r) => r.ip,
  },
  {
    key: "conns",
    label: "Connections",
    render: (r) => <MonoText>{r.conn_count.toLocaleString()}</MonoText>,
    sortValue: (r) => r.conn_count,
    className: "text-right",
  },
];

export function VpnTunnelDetail() {
  const { name } = useParams<{ name: string }>();
  const tunnelsQ = useQuery({
    queryKey: queryKeys.tunnels(),
    queryFn: api.tunnels,
    refetchInterval: 5_000,
  });
  const clientsQ = useQuery({
    queryKey: queryKeys.clients(),
    queryFn: api.clients,
    refetchInterval: 5_000,
  });
  const trafficQ = useQuery({
    queryKey: queryKeys.traffic(),
    queryFn: api.traffic,
    refetchInterval: 2_000,
  });

  const noData =
    !tunnelsQ.data || !clientsQ.data || !trafficQ.data;
  if (
    (tunnelsQ.isError || clientsQ.isError || trafficQ.isError) &&
    noData
  ) {
    return (
      <div className="text-sm text-rose font-mono">
        Failed to load tunnel data — retry shortly.
      </div>
    );
  }
  if (tunnelsQ.isPending || clientsQ.isPending || trafficQ.isPending) {
    return <div className="text-sm text-on-surface-variant">Loading...</div>;
  }
  if (!tunnelsQ.data || !clientsQ.data || !trafficQ.data) {
    return <div className="text-sm text-on-surface-variant">Loading...</div>;
  }

  const tunnel: Tunnel | undefined = tunnelsQ.data.data.tunnels.find(
    (t) => t.name === name,
  );
  if (!tunnel) {
    return (
      <div className="space-y-4">
        <Link
          to="/vpn/tunnels"
          className="text-sm text-primary hover:underline"
        >
          &larr; Back to tunnels
        </Link>
        <p className="text-sm text-on-surface-variant">Tunnel not found.</p>
      </div>
    );
  }

  const refetchFailed =
    tunnelsQ.isError || clientsQ.isError || trafficQ.isError;
  const allClients: Client[] = clientsQ.data.data.clients;
  const routedClients: ClientRow[] = allClients
    .map((c) => {
      const conns = (c.tunnel_conns ?? {})[tunnel.fwmark] ?? 0;
      return { hostname: c.hostname, ip: c.ip, conn_count: conns };
    })
    .filter((r) => r.conn_count > 0);

  const totalConns = routedClients.reduce((s, r) => s + r.conn_count, 0);

  const iface: Interface | undefined = trafficQ.data.data.interfaces.find(
    (i) => i.name === tunnel.interface,
  );
  const rxSeries = iface?.samples_60s.map((s) => s.rx_bps) ?? [];
  const txSeries = iface?.samples_60s.map((s) => s.tx_bps) ?? [];

  const combinedStale =
    (tunnelsQ.data.stale ?? false) ||
    (clientsQ.data.stale ?? false) ||
    (trafficQ.data.stale ?? false);
  const updatedAts = [
    tunnelsQ.data.updated_at,
    clientsQ.data.updated_at,
    trafficQ.data.updated_at,
  ].filter((u): u is string => !!u);
  const combinedUpdatedAt =
    updatedAts.length === 0
      ? null
      : updatedAts.reduce((a, b) => (a < b ? a : b));

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Link
            to="/vpn/tunnels"
            className="text-sm text-primary hover:underline"
          >
            &larr; Tunnels
          </Link>
          <h1 className="text-lg font-semibold">{tunnel.name}</h1>
          <StatusBadge kind={tunnel.healthy ? "healthy" : "failed"}>
            {tunnel.healthy ? "healthy" : "down"}
          </StatusBadge>
        </div>
        <div className="flex items-center gap-2">
          {refetchFailed && (
            <span className="text-[10px] uppercase tracking-wider font-bold bg-rose/10 text-rose px-2 py-0.5 rounded-sm">
              Refetch failed
            </span>
          )}
          <StaleIndicator stale={combinedStale} updatedAt={combinedUpdatedAt} />
        </div>
      </div>

      <div className="grid grid-cols-3 gap-4">
        <StatTile
          label="Total Connections"
          value={totalConns.toLocaleString()}
        />
        <StatTile
          label="RX / TX"
          value={`${formatBytes(tunnel.rx_bytes)} / ${formatBytes(tunnel.tx_bytes)}`}
        />
        <StatTile
          label="Last Handshake"
          value={
            tunnel.latest_handshake_seconds_ago > 0
              ? `${formatDuration(tunnel.latest_handshake_seconds_ago)} ago`
              : "never"
          }
        />
      </div>

      <div className="grid grid-cols-2 gap-4">
        <div className="bg-surface-container rounded-sm p-4">
          <p className="label-xs mb-2">Endpoint</p>
          <MonoText className="text-sm">{tunnel.endpoint || "—"}</MonoText>
          <p className="label-xs mt-3 mb-1">Public key</p>
          <MonoText className="text-xs break-all text-on-surface-variant">
            {tunnel.public_key}
          </MonoText>
          <p className="label-xs mt-3 mb-1">Fwmark · Routing table</p>
          <MonoText className="text-sm">
            {tunnel.fwmark} · {tunnel.routing_table}
          </MonoText>
        </div>
        <div className="bg-surface-container rounded-sm p-4">
          <p className="label-xs mb-2">RX bps (60s)</p>
          <Sparkline data={rxSeries} className="h-12" />
          <p className="text-xs text-on-surface-variant mt-1">
            now {formatBps(iface?.rx_bps ?? 0)}
          </p>
          <p className="label-xs mt-4 mb-2">TX bps (60s)</p>
          <Sparkline data={txSeries} className="h-12" />
          <p className="text-xs text-on-surface-variant mt-1">
            now {formatBps(iface?.tx_bps ?? 0)}
          </p>
        </div>
      </div>

      <div className="space-y-2">
        <h2 className="label-xs">Routed Clients</h2>
        {routedClients.length === 0 ? (
          <p className="text-sm text-on-surface-variant font-mono">
            No clients are routing through this tunnel right now.
          </p>
        ) : (
          <DataTable
            columns={clientColumns}
            rows={routedClients}
            rowKey={(r) => r.ip}
          />
        )}
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Type-check**

```bash
cd modules/dashboard/frontend && npx tsc --noEmit
```

Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add modules/dashboard/frontend/src/pages/VpnTunnelDetail.tsx
git commit -m "dashboard: add VPN Tunnel detail page"
```

---

## Task 12: Traffic page

**Files:**
- Create: `modules/dashboard/frontend/src/pages/Traffic.tsx`

**Why this design:** One card per interface (LAN, WAN, each tunnel), each card showing current RX/TX rate, total bytes, and a 60-sample rate sparkline. Order: WAN first, then LAN, then tunnels by name. Empty state appears only if topology has no interfaces (impossible in practice).

- [ ] **Step 1: Create the page**

```tsx
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { Interface } from "@/lib/api";
import { queryKeys } from "@/lib/query-keys";
import { MonoText } from "@/components/MonoText";
import { StatusBadge } from "@/components/StatusBadge";
import { StaleIndicator } from "@/components/StaleIndicator";
import { Sparkline } from "@/components/Sparkline";
import { formatBytes, formatBps } from "@/lib/formatters";

const ROLE_ORDER: Record<string, number> = {
  wan: 0,
  lan: 1,
  tunnel: 2,
  "": 3,
};

function sortInterfaces(a: Interface, b: Interface): number {
  const ra = ROLE_ORDER[a.role] ?? 3;
  const rb = ROLE_ORDER[b.role] ?? 3;
  if (ra !== rb) return ra - rb;
  return a.name.localeCompare(b.name);
}

function InterfaceCard({ iface }: { iface: Interface }) {
  const rxSeries = iface.samples_60s.map((s) => s.rx_bps);
  const txSeries = iface.samples_60s.map((s) => s.tx_bps);
  return (
    <div className="bg-surface-container rounded-sm p-4 space-y-3">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <h2 className="text-sm font-semibold">
            <MonoText>{iface.name}</MonoText>
          </h2>
          {iface.role && (
            <span className="text-[10px] uppercase tracking-wider font-bold text-on-surface-variant">
              {iface.role}
            </span>
          )}
        </div>
        <StatusBadge
          kind={iface.operstate === "up" ? "healthy" : iface.operstate === "down" ? "failed" : "muted"}
        >
          {iface.operstate}
        </StatusBadge>
      </div>

      <div className="grid grid-cols-2 gap-3">
        <div>
          <p className="label-xs mb-1">RX bps (60s)</p>
          <Sparkline data={rxSeries} className="h-10" />
          <div className="flex items-baseline justify-between mt-1">
            <MonoText className="text-xs text-on-surface-variant">
              now {formatBps(iface.rx_bps)}
            </MonoText>
            <MonoText className="text-xs text-on-surface-variant">
              total {formatBytes(iface.rx_bytes_total)}
            </MonoText>
          </div>
        </div>
        <div>
          <p className="label-xs mb-1">TX bps (60s)</p>
          <Sparkline data={txSeries} className="h-10" />
          <div className="flex items-baseline justify-between mt-1">
            <MonoText className="text-xs text-on-surface-variant">
              now {formatBps(iface.tx_bps)}
            </MonoText>
            <MonoText className="text-xs text-on-surface-variant">
              total {formatBytes(iface.tx_bytes_total)}
            </MonoText>
          </div>
        </div>
      </div>
    </div>
  );
}

export function Traffic() {
  const trafficQ = useQuery({
    queryKey: queryKeys.traffic(),
    queryFn: api.traffic,
    refetchInterval: 2_000,
  });

  if (trafficQ.isError && !trafficQ.data) {
    return (
      <div className="text-sm text-rose font-mono">
        Failed to load traffic data — retry shortly.
      </div>
    );
  }
  if (trafficQ.isPending || !trafficQ.data) {
    return <div className="text-sm text-on-surface-variant">Loading...</div>;
  }

  const interfaces = [...trafficQ.data.data.interfaces].sort(sortInterfaces);
  const refetchFailed = trafficQ.isError;

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold">Traffic</h1>
        <div className="flex items-center gap-2">
          {refetchFailed && (
            <span className="text-[10px] uppercase tracking-wider font-bold bg-rose/10 text-rose px-2 py-0.5 rounded-sm">
              Refetch failed
            </span>
          )}
          <StaleIndicator
            stale={trafficQ.data.stale ?? false}
            updatedAt={trafficQ.data.updated_at ?? null}
          />
        </div>
      </div>

      {interfaces.length === 0 ? (
        <p className="text-sm text-on-surface-variant font-mono">
          No interfaces reported.
        </p>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          {interfaces.map((iface) => (
            <InterfaceCard key={iface.name} iface={iface} />
          ))}
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Type-check + commit**

```bash
cd modules/dashboard/frontend && npx tsc --noEmit
git add modules/dashboard/frontend/src/pages/Traffic.tsx
git commit -m "dashboard: add Traffic page with per-interface sparklines"
```

---

## Task 13: System page

**Files:**
- Create: `modules/dashboard/frontend/src/pages/System.tsx`

**Why this design:** Single page polling `/api/system` at 2s. Layout: top row of stat tiles (CPU usage, memory %, temperature, throttle flag), middle row showing throttle reason text + uptime + boot time, bottom section with a services table.

- [ ] **Step 1: Create the page**

```tsx
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { Service } from "@/lib/api";
import { queryKeys } from "@/lib/query-keys";
import { MonoText } from "@/components/MonoText";
import { StatusBadge } from "@/components/StatusBadge";
import { StaleIndicator } from "@/components/StaleIndicator";
import { DataTable, type Column } from "@/components/DataTable";
import { formatBytes, formatPercent, formatDuration, formatAbsoluteTime } from "@/lib/formatters";

function StatTile({
  label,
  value,
  tone,
}: {
  label: string;
  value: string;
  tone?: "healthy" | "degraded" | "failed" | undefined;
}) {
  const colorClass =
    tone === "failed"
      ? "text-rose"
      : tone === "degraded"
        ? "text-amber"
        : tone === "healthy"
          ? "text-emerald"
          : "";
  return (
    <div className="bg-surface-container rounded-sm p-4">
      <p className="label-xs mb-1">{label}</p>
      <MonoText className={`text-lg font-semibold ${colorClass}`}>
        {value}
      </MonoText>
    </div>
  );
}

const serviceColumns: Column<Service>[] = [
  {
    key: "name",
    label: "Service",
    render: (r) => <MonoText className="text-xs">{r.name}</MonoText>,
    sortValue: (r) => r.name,
  },
  {
    key: "active",
    label: "Status",
    render: (r) => (
      <StatusBadge kind={r.active ? "healthy" : "failed"}>
        {r.raw_state}
      </StatusBadge>
    ),
    sortValue: (r) => (r.active ? 0 : 1),
  },
];

export function System() {
  const systemQ = useQuery({
    queryKey: queryKeys.system(),
    queryFn: api.system,
    refetchInterval: 2_000,
  });

  if (systemQ.isError && !systemQ.data) {
    return (
      <div className="text-sm text-rose font-mono">
        Failed to load system data — retry shortly.
      </div>
    );
  }
  if (systemQ.isPending || !systemQ.data) {
    return <div className="text-sm text-on-surface-variant">Loading...</div>;
  }

  const sys = systemQ.data.data;
  const refetchFailed = systemQ.isError;
  const cpuActive = sys.cpu.percent_user + sys.cpu.percent_system;
  const memTone =
    sys.memory.percent_used >= 90
      ? "failed"
      : sys.memory.percent_used >= 75
        ? "degraded"
        : "healthy";
  const tempTone =
    sys.temperature_c >= 80
      ? "failed"
      : sys.temperature_c >= 70
        ? "degraded"
        : "healthy";
  const throttleTone = sys.throttled_flag ? "failed" : "healthy";
  const services = sys.services ?? [];
  const downServices = services.filter((s) => !s.active);

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold">System</h1>
        <div className="flex items-center gap-2">
          {refetchFailed && (
            <span className="text-[10px] uppercase tracking-wider font-bold bg-rose/10 text-rose px-2 py-0.5 rounded-sm">
              Refetch failed
            </span>
          )}
          <StaleIndicator
            stale={systemQ.data.stale ?? false}
            updatedAt={systemQ.data.updated_at ?? null}
          />
        </div>
      </div>

      <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
        <StatTile label="CPU active" value={formatPercent(cpuActive)} />
        <StatTile
          label="Memory used"
          value={`${formatPercent(sys.memory.percent_used)} (${formatBytes(sys.memory.used_bytes)} / ${formatBytes(sys.memory.total_bytes)})`}
          tone={memTone}
        />
        <StatTile
          label="Temperature"
          value={`${sys.temperature_c.toFixed(1)} °C`}
          tone={tempTone}
        />
        <StatTile
          label="Throttle"
          value={sys.throttled_flag ? "ACTIVE" : "clear"}
          tone={throttleTone}
        />
      </div>

      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
        <div className="bg-surface-container rounded-sm p-4">
          <p className="label-xs mb-1">CPU breakdown</p>
          <div className="text-xs space-y-1 mt-2 font-mono">
            <div className="flex justify-between">
              <span>user</span>
              <MonoText>{formatPercent(sys.cpu.percent_user)}</MonoText>
            </div>
            <div className="flex justify-between">
              <span>system</span>
              <MonoText>{formatPercent(sys.cpu.percent_system)}</MonoText>
            </div>
            <div className="flex justify-between">
              <span>iowait</span>
              <MonoText>{formatPercent(sys.cpu.percent_iowait)}</MonoText>
            </div>
            <div className="flex justify-between">
              <span>idle</span>
              <MonoText>{formatPercent(sys.cpu.percent_idle)}</MonoText>
            </div>
          </div>
        </div>
        <div className="bg-surface-container rounded-sm p-4">
          <p className="label-xs mb-1">Throttle reason</p>
          <MonoText className="text-xs break-all text-on-surface-variant">
            {sys.throttled || "—"}
          </MonoText>
        </div>
        <div className="bg-surface-container rounded-sm p-4">
          <p className="label-xs mb-1">Uptime</p>
          <MonoText className="text-sm">
            {formatDuration(sys.uptime_seconds)}
          </MonoText>
          <p className="label-xs mt-3 mb-1">Boot time</p>
          <MonoText className="text-xs text-on-surface-variant">
            {formatAbsoluteTime(sys.boot_time)}
          </MonoText>
        </div>
      </div>

      <div className="space-y-2">
        <div className="flex items-center gap-3">
          <h2 className="label-xs">Services</h2>
          {downServices.length > 0 && (
            <span className="text-[10px] uppercase tracking-wider font-bold bg-rose/10 text-rose px-2 py-0.5 rounded-sm">
              {downServices.length} down
            </span>
          )}
        </div>
        {services.length === 0 ? (
          <p className="text-sm text-on-surface-variant font-mono">
            No services tracked.
          </p>
        ) : (
          <DataTable
            columns={serviceColumns}
            rows={services}
            rowKey={(r) => r.name}
          />
        )}
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Type-check + commit**

```bash
cd modules/dashboard/frontend && npx tsc --noEmit
git add modules/dashboard/frontend/src/pages/System.tsx
git commit -m "dashboard: add System page (CPU, memory, temp, throttle, services)"
```

---

## Task 14: Firewall + UPnP page

**Files:**
- Create: `modules/dashboard/frontend/src/pages/Firewall.tsx`

**Why this design:** The page surfaces the full Firewall contract from spec §7.4:
1. **Port forwards** table from `/api/firewall/rules.port_forwards` (static topology).
2. **PBR** section with three sub-tables from `/api/firewall/rules.pbr.{source_rules, domain_rules, pooled_rules}`.
3. **Allowlist** list from `/api/firewall/rules.allowed_macs` + the `blocked_forward_count_1h` callout as a stat tile ("Blocked forwards (1h)").
4. **Counters** — one section per chain from `/api/firewall/counters.chains[]`, each with its nested per-rule counters sorted by bytes desc.
5. **UPnP leases** table from `/api/upnp.leases`.

Combined stale indicator across all three queries. Empty-state copy for each empty list (no forwards, no PBR, no MACs, no counters, no leases).

- [ ] **Step 1: Create the page**

```tsx
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type {
  PortForward,
  PBRSourceRule,
  PBRDomainRule,
  PBRPooledRule,
  FirewallChain,
  RuleCounter,
  UPnPLease,
} from "@/lib/api";
import { queryKeys } from "@/lib/query-keys";
import { MonoText } from "@/components/MonoText";
import { StatusBadge } from "@/components/StatusBadge";
import { StaleIndicator } from "@/components/StaleIndicator";
import { DataTable, type Column } from "@/components/DataTable";
import { formatBytes } from "@/lib/formatters";

function StatTile({ label, value, tone }: { label: string; value: string; tone?: "failed" | "degraded" | "healthy" | undefined }) {
  const color =
    tone === "failed"
      ? "text-rose"
      : tone === "degraded"
        ? "text-amber"
        : tone === "healthy"
          ? "text-emerald"
          : "";
  return (
    <div className="bg-surface-container rounded-sm p-4">
      <p className="label-xs mb-1">{label}</p>
      <MonoText className={`text-lg font-semibold ${color}`}>{value}</MonoText>
    </div>
  );
}

const portForwardColumns: Column<PortForward>[] = [
  {
    key: "ext",
    label: "External",
    render: (r) => <MonoText>{r.protocol}/{r.external_port}</MonoText>,
    sortValue: (r) => r.external_port,
  },
  {
    key: "dest",
    label: "Destination",
    render: (r) => <MonoText>{r.destination}</MonoText>,
    sortValue: (r) => r.destination,
  },
];

const sourceRuleColumns: Column<PBRSourceRule>[] = [
  {
    key: "sources",
    label: "Sources",
    render: (r) => <MonoText className="text-xs">{r.sources.join(", ")}</MonoText>,
    sortValue: (r) => r.sources.join(","),
  },
  {
    key: "tunnel",
    label: "Tunnel",
    render: (r) => <MonoText>{r.tunnel}</MonoText>,
    sortValue: (r) => r.tunnel,
  },
];

const domainRuleColumns: Column<PBRDomainRule>[] = [
  {
    key: "tunnel",
    label: "Tunnel",
    render: (r) => <MonoText>{r.tunnel}</MonoText>,
    sortValue: (r) => r.tunnel,
  },
  {
    key: "domains",
    label: "Domains",
    render: (r) => (
      <MonoText className="text-xs">{r.domains.join(", ")}</MonoText>
    ),
    sortValue: (r) => r.domains.join(","),
  },
];

const pooledRuleColumns: Column<PBRPooledRule>[] = [
  {
    key: "pool",
    label: "Pool",
    render: (r) => <MonoText>{r.pool}</MonoText>,
    sortValue: (r) => r.pool,
  },
  {
    key: "sources",
    label: "Sources",
    render: (r) => <MonoText className="text-xs">{r.sources.join(", ")}</MonoText>,
    sortValue: (r) => r.sources.join(","),
  },
];

const upnpColumns: Column<UPnPLease>[] = [
  {
    key: "external",
    label: "External",
    render: (r) => <MonoText>{r.protocol}/{r.external_port}</MonoText>,
    sortValue: (r) => r.external_port,
  },
  {
    key: "internal",
    label: "Internal target",
    render: (r) => <MonoText>{r.internal_addr}:{r.internal_port}</MonoText>,
    sortValue: (r) => r.internal_addr,
  },
  {
    key: "description",
    label: "Description",
    render: (r) =>
      r.description || <span className="text-on-surface-variant">—</span>,
    sortValue: (r) => r.description ?? "",
  },
];

const counterColumns: Column<RuleCounter>[] = [
  {
    key: "handle",
    label: "Rule",
    render: (r) => <MonoText className="text-xs">#{r.handle}</MonoText>,
    sortValue: (r) => r.handle,
  },
  {
    key: "comment",
    label: "Comment",
    render: (r) =>
      r.comment ? (
        <MonoText className="text-xs">{r.comment}</MonoText>
      ) : (
        <span className="text-on-surface-variant">—</span>
      ),
    sortValue: (r) => r.comment ?? "",
  },
  {
    key: "packets",
    label: "Packets",
    render: (r) => <MonoText>{r.packets.toLocaleString()}</MonoText>,
    sortValue: (r) => r.packets,
    className: "text-right",
  },
  {
    key: "bytes",
    label: "Bytes",
    render: (r) => <MonoText>{formatBytes(r.bytes)}</MonoText>,
    sortValue: (r) => r.bytes,
    className: "text-right",
  },
];

function ChainCountersCard({ chain }: { chain: FirewallChain }) {
  const rows = [...chain.counters].sort((a, b) => b.bytes - a.bytes);
  return (
    <div className="bg-surface-container rounded-sm p-4 space-y-2">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold">
          <MonoText>{chain.family}/{chain.table}/{chain.name}</MonoText>
        </h3>
        <div className="flex items-center gap-2">
          {chain.hook && (
            <span className="text-[10px] uppercase tracking-wider font-bold text-on-surface-variant">
              {chain.hook}
            </span>
          )}
          {chain.policy && (
            <StatusBadge kind={chain.policy === "drop" ? "failed" : "info"}>
              {chain.policy}
            </StatusBadge>
          )}
        </div>
      </div>
      {rows.length === 0 ? (
        <p className="text-xs text-on-surface-variant font-mono">
          No counter rules in this chain.
        </p>
      ) : (
        <DataTable
          columns={counterColumns}
          rows={rows}
          rowKey={(r) => String(r.handle)}
        />
      )}
    </div>
  );
}

export function Firewall() {
  const rulesQ = useQuery({
    queryKey: queryKeys.firewallRules(),
    queryFn: api.firewallRules,
    refetchInterval: 30_000,
  });
  const countersQ = useQuery({
    queryKey: queryKeys.firewallCounters(),
    queryFn: api.firewallCounters,
    refetchInterval: 5_000,
  });
  const upnpQ = useQuery({
    queryKey: queryKeys.upnp(),
    queryFn: api.upnp,
    refetchInterval: 15_000,
  });

  const noData = !rulesQ.data || !countersQ.data || !upnpQ.data;
  if ((rulesQ.isError || countersQ.isError || upnpQ.isError) && noData) {
    return (
      <div className="text-sm text-rose font-mono">
        Failed to load firewall data — retry shortly.
      </div>
    );
  }
  if (rulesQ.isPending || countersQ.isPending || upnpQ.isPending) {
    return <div className="text-sm text-on-surface-variant">Loading...</div>;
  }
  if (!rulesQ.data || !countersQ.data || !upnpQ.data) {
    return <div className="text-sm text-on-surface-variant">Loading...</div>;
  }

  const refetchFailed =
    rulesQ.isError || countersQ.isError || upnpQ.isError;
  const rules = rulesQ.data.data;
  const chains = countersQ.data.data.chains;
  const leases = upnpQ.data.data.leases;

  const combinedStale =
    (rulesQ.data.stale ?? false) ||
    (countersQ.data.stale ?? false) ||
    (upnpQ.data.stale ?? false);
  const updatedAts = [
    rulesQ.data.updated_at,
    countersQ.data.updated_at,
    upnpQ.data.updated_at,
  ].filter((u): u is string => !!u);
  const combinedUpdatedAt =
    updatedAts.length === 0
      ? null
      : updatedAts.reduce((a, b) => (a < b ? a : b));

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold">Firewall &amp; UPnP</h1>
        <div className="flex items-center gap-2">
          {refetchFailed && (
            <span className="text-[10px] uppercase tracking-wider font-bold bg-rose/10 text-rose px-2 py-0.5 rounded-sm">
              Refetch failed
            </span>
          )}
          <StaleIndicator stale={combinedStale} updatedAt={combinedUpdatedAt} />
        </div>
      </div>

      <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
        <StatTile label="Port forwards" value={String(rules.port_forwards.length)} />
        <StatTile label="PBR rules" value={String(rules.pbr.source_rules.length + rules.pbr.domain_rules.length + rules.pbr.pooled_rules.length)} />
        <StatTile label="Allowed MACs" value={String(rules.allowed_macs.length)} />
        <StatTile
          label="Blocked forwards (1h)"
          value={rules.blocked_forward_count_1h.toLocaleString()}
          tone={rules.blocked_forward_count_1h > 0 ? "degraded" : undefined}
        />
      </div>

      <div className="space-y-2">
        <h2 className="label-xs">Port forwards</h2>
        {rules.port_forwards.length === 0 ? (
          <p className="text-sm text-on-surface-variant font-mono">
            No port forwards configured.
          </p>
        ) : (
          <DataTable
            columns={portForwardColumns}
            rows={rules.port_forwards}
            rowKey={(r) => `${r.protocol}/${r.external_port}`}
          />
        )}
      </div>

      <div className="space-y-2">
        <h2 className="label-xs">PBR — source rules</h2>
        {rules.pbr.source_rules.length === 0 ? (
          <p className="text-sm text-on-surface-variant font-mono">
            No source-based PBR rules.
          </p>
        ) : (
          <DataTable
            columns={sourceRuleColumns}
            rows={rules.pbr.source_rules}
            rowKey={(r) => `${r.tunnel}|${r.sources.join(",")}`}
          />
        )}
      </div>

      <div className="space-y-2">
        <h2 className="label-xs">PBR — domain rules</h2>
        {rules.pbr.domain_rules.length === 0 ? (
          <p className="text-sm text-on-surface-variant font-mono">
            No domain-based PBR rules.
          </p>
        ) : (
          <DataTable
            columns={domainRuleColumns}
            rows={rules.pbr.domain_rules}
            rowKey={(r) => `${r.tunnel}|${r.domains.join(",")}`}
          />
        )}
      </div>

      <div className="space-y-2">
        <h2 className="label-xs">PBR — pooled rules</h2>
        {rules.pbr.pooled_rules.length === 0 ? (
          <p className="text-sm text-on-surface-variant font-mono">
            No pooled PBR rules.
          </p>
        ) : (
          <DataTable
            columns={pooledRuleColumns}
            rows={rules.pbr.pooled_rules}
            rowKey={(r) => `${r.pool}|${r.sources.join(",")}`}
          />
        )}
      </div>

      <div className="space-y-2">
        <h2 className="label-xs">Allowlisted MACs ({rules.allowed_macs.length})</h2>
        {rules.allowed_macs.length === 0 ? (
          <p className="text-sm text-on-surface-variant font-mono">
            Allowlist is empty or disabled.
          </p>
        ) : (
          <div className="bg-surface-container rounded-sm p-4 flex flex-wrap gap-2">
            {rules.allowed_macs.map((m) => (
              <MonoText key={m} className="text-xs bg-surface-high px-2 py-1 rounded-sm">
                {m}
              </MonoText>
            ))}
          </div>
        )}
      </div>

      <div className="space-y-3">
        <h2 className="label-xs">Counters ({chains.length} chains)</h2>
        {chains.length === 0 ? (
          <p className="text-sm text-on-surface-variant font-mono">
            No chains reported.
          </p>
        ) : (
          chains
            .slice()
            .sort((a, b) => (a.family + a.table + a.name).localeCompare(b.family + b.table + b.name))
            .map((c) => (
              <ChainCountersCard key={`${c.family}/${c.table}/${c.name}`} chain={c} />
            ))
        )}
      </div>

      <div className="space-y-2">
        <h2 className="label-xs">UPnP leases ({leases.length})</h2>
        {leases.length === 0 ? (
          <p className="text-sm text-on-surface-variant font-mono">
            No active UPnP leases.
          </p>
        ) : (
          <DataTable
            columns={upnpColumns}
            rows={leases}
            rowKey={(r) => `${r.protocol}/${r.external_port}`}
          />
        )}
      </div>
    </div>
  );
}
```

Note: PBR rule rowKeys are derived as `tunnel|sources.join(",")` (or pool|sources for pooled) — the (tunnel, sources) tuple is unique per rule by design (the nft module rejects overlapping pooled+source rules), so the join is a stable key without needing the row index.

- [ ] **Step 2: Type-check + commit**

```bash
cd modules/dashboard/frontend && npx tsc --noEmit
git add modules/dashboard/frontend/src/pages/Firewall.tsx
git commit -m "dashboard: add Firewall + UPnP page (port forwards, PBR, allowlist, counters, leases)"
```

---

## Task 15: QoS page

**Files:**
- Create: `modules/dashboard/frontend/src/pages/Qos.tsx`

**Why this design:** Two columns: WAN egress (CAKE) on the left, WAN ingress (HTB+fq_codel) on the right. CAKE column has a per-tin breakdown table; HTB column has a fq_codel block (drops, ECN marks, flow counts).

- [ ] **Step 1: Create the page**

```tsx
import { useQuery } from "@tanstack/react-query";
import { api } from "@/lib/api";
import type { QdiscStats, CAKETin } from "@/lib/api";
import { queryKeys } from "@/lib/query-keys";
import { MonoText } from "@/components/MonoText";
import { StaleIndicator } from "@/components/StaleIndicator";
import { DataTable, type Column } from "@/components/DataTable";
import { formatBytes, formatBps } from "@/lib/formatters";

function StatRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex justify-between text-xs font-mono">
      <span className="text-on-surface-variant">{label}</span>
      <MonoText>{value}</MonoText>
    </div>
  );
}

const tinColumns: Column<CAKETin>[] = [
  {
    key: "name",
    label: "Tin",
    render: (r) => <MonoText className="text-xs">{r.name}</MonoText>,
    sortValue: (r) => r.name,
  },
  {
    key: "thresh",
    label: "Thresh",
    render: (r) =>
      r.thresh_kbit > 0 ? (
        <MonoText className="text-xs">{r.thresh_kbit.toLocaleString()} kbit</MonoText>
      ) : (
        <span className="text-on-surface-variant">—</span>
      ),
    sortValue: (r) => r.thresh_kbit,
    className: "text-right",
  },
  {
    key: "bytes",
    label: "Bytes",
    render: (r) => <MonoText className="text-xs">{formatBytes(r.bytes)}</MonoText>,
    sortValue: (r) => r.bytes,
    className: "text-right",
  },
  {
    key: "drops",
    label: "Drops",
    render: (r) => (
      <MonoText className={`text-xs ${r.drops > 0 ? "text-amber" : ""}`}>
        {r.drops.toLocaleString()}
      </MonoText>
    ),
    sortValue: (r) => r.drops,
    className: "text-right",
  },
  {
    key: "marks",
    label: "ECN marks",
    render: (r) => (
      <MonoText className="text-xs">{r.marks.toLocaleString()}</MonoText>
    ),
    sortValue: (r) => r.marks,
    className: "text-right",
  },
];

function EgressCard({ stats }: { stats: QdiscStats }) {
  return (
    <div className="bg-surface-container rounded-sm p-4 space-y-3">
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-semibold">WAN egress (CAKE)</h2>
        {stats.bandwidth_bps > 0 && (
          <span className="text-[10px] uppercase tracking-wider font-bold text-on-surface-variant">
            shaped {formatBps(stats.bandwidth_bps)}
          </span>
        )}
      </div>
      <div className="space-y-1">
        <StatRow label="Sent" value={`${formatBytes(stats.sent_bytes)} / ${stats.sent_packets.toLocaleString()} pkt`} />
        <StatRow label="Dropped" value={stats.dropped.toLocaleString()} />
        <StatRow label="Overlimits" value={stats.overlimits.toLocaleString()} />
        <StatRow label="Requeues" value={stats.requeues.toLocaleString()} />
        <StatRow label="Backlog" value={`${formatBytes(stats.backlog_bytes)} / ${stats.backlog_pkts} pkt`} />
      </div>
      {stats.tins && stats.tins.length > 0 && (
        <div className="space-y-2">
          <p className="label-xs">Per-tin breakdown</p>
          <DataTable columns={tinColumns} rows={stats.tins} rowKey={(r) => r.name} />
        </div>
      )}
    </div>
  );
}

function IngressCard({ stats }: { stats: QdiscStats }) {
  return (
    <div className="bg-surface-container rounded-sm p-4 space-y-3">
      <div className="flex items-center justify-between">
        <h2 className="text-sm font-semibold">WAN ingress (HTB + fq_codel)</h2>
      </div>
      <div className="space-y-1">
        <StatRow label="Sent" value={`${formatBytes(stats.sent_bytes)} / ${stats.sent_packets.toLocaleString()} pkt`} />
        <StatRow label="Dropped" value={stats.dropped.toLocaleString()} />
        <StatRow label="Overlimits" value={stats.overlimits.toLocaleString()} />
        <StatRow label="Backlog" value={`${formatBytes(stats.backlog_bytes)} / ${stats.backlog_pkts} pkt`} />
        <StatRow label="ECN marks" value={(stats.ecn_mark ?? 0).toLocaleString()} />
        <StatRow label="Drop overlimit" value={(stats.drop_overlimit ?? 0).toLocaleString()} />
        <StatRow label="New flows seen" value={(stats.new_flow_count ?? 0).toLocaleString()} />
        <StatRow label="Active flows" value={`${(stats.new_flows_len ?? 0).toLocaleString()} new / ${(stats.old_flows_len ?? 0).toLocaleString()} old`} />
      </div>
    </div>
  );
}

export function Qos() {
  const qosQ = useQuery({
    queryKey: queryKeys.qos(),
    queryFn: api.qos,
    refetchInterval: 5_000,
  });

  if (qosQ.isError && !qosQ.data) {
    return (
      <div className="text-sm text-rose font-mono">
        Failed to load QoS data — retry shortly.
      </div>
    );
  }
  if (qosQ.isPending || !qosQ.data) {
    return <div className="text-sm text-on-surface-variant">Loading...</div>;
  }

  const refetchFailed = qosQ.isError;
  const eg = qosQ.data.data.wan_egress;
  const in_ = qosQ.data.data.wan_ingress;

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h1 className="text-lg font-semibold">QoS</h1>
        <div className="flex items-center gap-2">
          {refetchFailed && (
            <span className="text-[10px] uppercase tracking-wider font-bold bg-rose/10 text-rose px-2 py-0.5 rounded-sm">
              Refetch failed
            </span>
          )}
          <StaleIndicator
            stale={qosQ.data.stale ?? false}
            updatedAt={qosQ.data.updated_at ?? null}
          />
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        {eg ? (
          <EgressCard stats={eg} />
        ) : (
          <div className="bg-surface-container rounded-sm p-4 text-sm text-on-surface-variant font-mono">
            WAN egress qdisc unavailable.
          </div>
        )}
        {in_ ? (
          <IngressCard stats={in_} />
        ) : (
          <div className="bg-surface-container rounded-sm p-4 text-sm text-on-surface-variant font-mono">
            WAN ingress qdisc unavailable.
          </div>
        )}
      </div>
    </div>
  );
}
```

- [ ] **Step 2: Type-check + commit**

```bash
cd modules/dashboard/frontend && npx tsc --noEmit
git add modules/dashboard/frontend/src/pages/Qos.tsx
git commit -m "dashboard: add QoS page (CAKE egress + HTB+fq_codel ingress)"
```

---

## Task 16: Wire routes + remove Placeholder

**Files:**
- Modify: `modules/dashboard/frontend/src/App.tsx`
- Delete: `modules/dashboard/frontend/src/pages/Placeholder.tsx`

- [ ] **Step 1: Replace `App.tsx`**

```tsx
import { Route, Routes } from "react-router-dom";
import { Layout } from "./components/Layout";
import { Overview } from "./pages/Overview";
import { VpnPools } from "./pages/VpnPools";
import { VpnPoolDetail } from "./pages/VpnPoolDetail";
import { VpnTunnels } from "./pages/VpnTunnels";
import { VpnTunnelDetail } from "./pages/VpnTunnelDetail";
import { Clients } from "./pages/Clients";
import { ClientDetail } from "./pages/ClientDetail";
import { Adguard } from "./pages/Adguard";
import { Traffic } from "./pages/Traffic";
import { System } from "./pages/System";
import { Firewall } from "./pages/Firewall";
import { Qos } from "./pages/Qos";

function NotFound() {
  return (
    <div className="bg-surface-container p-8 mt-8 text-center">
      <h1 className="text-lg font-semibold mb-4">Not Found</h1>
      <p className="font-mono text-sm text-on-surface-variant">
        No such page.
      </p>
    </div>
  );
}

export function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route path="/" element={<Overview />} />
        <Route path="/vpn/pools" element={<VpnPools />} />
        <Route path="/vpn/pools/:name" element={<VpnPoolDetail />} />
        <Route path="/vpn/tunnels" element={<VpnTunnels />} />
        <Route path="/vpn/tunnels/:name" element={<VpnTunnelDetail />} />
        <Route path="/clients" element={<Clients />} />
        <Route path="/clients/:ip" element={<ClientDetail />} />
        <Route path="/adguard" element={<Adguard />} />
        <Route path="/traffic" element={<Traffic />} />
        <Route path="/firewall" element={<Firewall />} />
        <Route path="/qos" element={<Qos />} />
        <Route path="/system" element={<System />} />
        <Route path="*" element={<NotFound />} />
      </Route>
    </Routes>
  );
}
```

- [ ] **Step 2: Delete the now-unused Placeholder file**

```bash
rm modules/dashboard/frontend/src/pages/Placeholder.tsx
```

- [ ] **Step 3: Type-check + smoke-run**

```bash
cd modules/dashboard/frontend && npx tsc --noEmit
```

Then start the dev server (in a separate shell) and click through every Phase 3 route:

```bash
cd modules/dashboard/frontend && npm run dev
```

In the browser, open each: `/vpn/tunnels`, `/vpn/tunnels/wg_sw`, `/traffic`, `/firewall`, `/qos`, `/system`. Verify each renders without console errors. (The dev server proxies `/api/*` to the local backend; if the backend isn't running, every page should show its loading state then "Failed to load" — that's expected and proves the error guard works.)

- [ ] **Step 4: Commit**

```bash
git add modules/dashboard/frontend/src/App.tsx
git rm modules/dashboard/frontend/src/pages/Placeholder.tsx
git commit -m "dashboard: route Phase 3 pages and drop Placeholder"
```

---

## Task 17: Empty/error/refetch-failed audit

**Files:**
- Modify (only as needed): any Phase 1+2 page missing the polish patterns

**Why this design:** Rather than re-edit pages that already follow the pattern, audit each one and only touch the gaps. The patterns all six new pages adopt are:
1. **Cold-failure error guard** before loading guard.
2. **Refetch-failed indicator** in the header next to `<StaleIndicator>` (small rose chip).
3. **Empty-state copy** ("No X found / configured / yet.") rendered instead of an empty `<DataTable>`.

- [ ] **Step 1: Audit existing pages**

For each of `Overview.tsx`, `VpnPools.tsx`, `VpnPoolDetail.tsx`, `Clients.tsx`, `ClientDetail.tsx`, `Adguard.tsx`:

```bash
grep -nE 'isError|empty|No .* found|noData' modules/dashboard/frontend/src/pages/<file>
```

For each page, confirm:
- Error guard checks `isError && noData` (not just `isError`).
- Header has the `Refetch failed` chip when `refetchFailed` is true.
- Each `<DataTable>` is wrapped in `length === 0 ? <empty copy> : <DataTable />`.

Document any gap in a one-line note (no need to commit — just collect them).

- [ ] **Step 2: Apply fixes**

For each gap found, edit the file using the same pattern shown in Task 14 (Firewall page) — paste the `Refetch failed` chip into the header, wrap the table in the empty-state ternary, switch the error guard to the `noData`-aware variant.

- [ ] **Step 3: Type-check + commit**

```bash
cd modules/dashboard/frontend && npx tsc --noEmit
git add modules/dashboard/frontend/src/pages/
git commit -m "dashboard: audit pre-Phase-3 pages for empty/refetch-failed polish"
```

If no changes were needed, skip the commit (no commit on green audit).

---

## Task 18: Deploy to router via flake-split workflow

Per `/Users/giko/Documents/router/CLAUDE.md`'s flake-split workflow.

- [ ] **Step 1: Push public flake**

```bash
cd /Users/giko/Documents/nixos-rpi4-router && git push origin main
```

- [ ] **Step 2: Wait for CI release**

```bash
gh run watch --exit-status $(gh run list --workflow build-dashboard.yml --limit 1 --json databaseId --jq '.[0].databaseId')
gh release view --json tagName,assets --jq '.tagName, .assets[0].name'
```

Expected: tag `dashboard-20260413-<sha>`; asset `dashboard` of size ~7-8 MB.

- [ ] **Step 3: Pull CI's release commit (it bumps `version.json`)**

```bash
cd /Users/giko/Documents/nixos-rpi4-router && git pull --ff-only origin main
cat modules/dashboard/version.json
```

Expected: `version` matches the new release tag.

- [ ] **Step 4: rsync local config to Pi**

```bash
cd /Users/giko/Documents/router
rsync -av --no-o --no-g --delete --exclude='.claude/' --exclude='.superpowers/' ./ root@192.168.1.1:/etc/nixos/
```

- [ ] **Step 5: Update flake input + rebuild on Pi**

```bash
ssh root@192.168.1.1 "cd /etc/nixos && nix --extra-experimental-features 'nix-command flakes' flake update rpi4-router && nixos-rebuild switch --flake /etc/nixos#router"
```

Expected: `Done. The new configuration is /nix/store/...-nixos-system-router-...` at the end.

- [ ] **Step 6: Smoke-test the deployed dashboard**

```bash
ssh root@192.168.1.1 "systemctl is-active router-dashboard"
curl -s http://192.168.1.1:9090/api/health
curl -s http://192.168.1.1:9090/api/firewall/rules | head -c 400
curl -s http://192.168.1.1:9090/api/firewall/counters | head -c 400
curl -s http://192.168.1.1:9090/api/upnp | head -c 400
curl -s http://192.168.1.1:9090/api/qos | head -c 400
```

Expected:
- `active`
- `{"ok":true,"version":"<latest>","started_at":"..."}`
- `{"data":{"chains":[...]},"updated_at":"...","stale":false}`
- `{"data":{"counters":[...]},"updated_at":"...","stale":false}`
- `{"data":{"mappings":[]},"updated_at":"...","stale":false}` (empty unless an active UPnP session exists)
- `{"data":{"wan_egress":{"kind":"cake",...},"wan_ingress":{"kind":"htb+fq_codel",...}},"updated_at":"...","stale":false}`

Then open `http://192.168.1.1:9090` in a browser from an allowlisted host and click through every page (Overview → Tunnels → Tunnel detail → Pools → Clients → AdGuard → Traffic → Firewall → QoS → System). Watch the dev console for errors. Verify counters tick, sparklines move, and stale indicators stay green.

- [ ] **Step 7: Pull updated `flake.lock` back + commit**

```bash
cd /Users/giko/Documents/router
rsync -av root@192.168.1.1:/etc/nixos/flake.lock ./flake.lock
git diff flake.lock
git add flake.lock
git commit -m "pin Plan 3 dashboard release"
git push origin main
```

---

## Self-review checklist (writer-only — do not delegate)

**1. Spec coverage:**
- §8.2 routes for `/vpn/tunnels`, `/vpn/tunnels/:name`, `/traffic`, `/firewall`, `/qos`, `/system` → Tasks 10, 11, 12, 14, 15, 13 respectively. ✓
- §7.4 endpoint SHAPES:
  - `/api/firewall/rules` → `{port_forwards, pbr, allowed_macs, blocked_forward_count_1h}` — Task 7 handler. ✓
  - `/api/firewall/counters` → `{chains: [...]}` (chains with nested per-rule counters) — Task 7 handler. ✓
  - `/api/upnp` → `{leases: [...]}` — Task 7 handler. ✓
  - `/api/qos` → `{wan_egress, wan_ingress}` — Task 7 handler. ✓
- §7.3 sources:
  - `firewall.port_forwards` → `router.portForwards` via topology (Task 3 nix expansion). ✓
  - `firewall.pbr.*` → `router.pbr.{sourceRules, domainSets, pooledRules}` via topology (Task 3). ✓
  - `firewall.counters` → `nft --json list ruleset` (Task 1 source, Task 5 collector). ✓
  - `qos.wan_egress/wan_ingress` → `tc -s qdisc show dev` (Task 2 source, Task 6 collector). ✓
  - `upnp.leases` → "or equivalent" = derived from nft miniupnpd table (Task 1 extractor, Task 5 collector). ✓
- §8.3 page-↔-endpoint mapping → Task 14 Firewall page consumes all three firewall endpoints with correct keys. ✓
- §8.4 poll intervals honored per page; handler stale windows (Task 7) match §8.4 × 2. ✓
- §8.5 shared components used: `MonoText`, `StatusBadge`, `StaleIndicator`, `DataTable`, `Sparkline`, `ClientBadge`. ✓
- §11 testing strategy: parsers + state + collectors get Go tests; pages are smoke-tested manually. ✓
- §12 v1 scope: every v1 page is implemented; tunnel detail (deferred to v1.1 in spec) is included as Task 11 because the original Plan 2 roadmap listed it under Phase 3 and the data is already available. ✓

**2. Placeholder scan:**
- No "TBD", "TODO", "implement later" strings in tasks. ✓
- No "Similar to Task N" references — full code repeated where needed. ✓
- Every step that changes code shows the code. ✓
- Every command has expected output. ✓
- **No live firewall ruleset committed to repo** — Task 1 uses a synthetic, content-free fixture. ✓

**3. Type consistency:**
- `model.Firewall` fields (`PortForwards`, `PBR`, `AllowedMACs`, `BlockedForwardCount1h`, `Chains`, `UPnPLeases`) match handler projections in Task 7 and TS `FirewallRules` / `FirewallChain` / `UPnPLease` in Task 9. ✓
- `model.FirewallChain.Counters` (nested) matches `FirewallChain.counters` in TS and the `chains: [...]` shape of `/api/firewall/counters`. ✓
- `model.UPnPLease` (not `UPnPMapping`) matches `UPnPLease` in TS and the `leases: [...]` shape of `/api/upnp`. ✓
- `topology.PortForward`, `topology.PBRSourceRule`, `topology.PBRDomainRule` parse the JSON emitted by `default.nix` Step 1 (`port_forwards` + `pbr_source_rules` + `pbr_domain_rules`). ✓
- `QoS` / `QdiscStats` / `CAKETin` shapes match across Go model → handler → TS type. ✓
- `nft.Ruleset.UPnPMappings` shape (Protocol/ExternalPort/InternalAddr/InternalPort/Description) → adapted in Task 5 collector to `model.UPnPLease`. ✓
- `tc.QdiscStats` field names (Kind, NewFlowCount, OldFlowsLen, NewFlowsLen, ECNMark, DropOverlimit) match `model.QdiscStats` and TS `QdiscStats`. ✓
- Frontend `Tunnel` consumed in Tasks 10, 11 has `fwmark`, `endpoint`, `latest_handshake_seconds_ago`, `rx_bytes`, `tx_bytes`, `healthy`, `public_key`, `routing_table`, `interface` — all already in `lib/api.ts`. ✓
- `Interface.role` ("wan"/"lan"/"tunnel") consumed in Task 12 — already in `lib/api.ts` (added in Round 1 fixes). ✓
- `Service` has `name`, `active`, `raw_state` per `lib/api.ts` — used in Task 13 service table. ✓

**4. Security / operational:**
- No live firewall topology (port forwards, MAC allowlist, private IPs) leaks into this public repo via test fixtures — Task 1 Step 1 explicitly warns and uses synthetic content. ✓
- Per-endpoint stale windows (Task 7) match spec §8.4 poll cadence × 2 so stale badges don't false-fire on a single missed collector tick for rarely-changing data. ✓

**5. Workflow / per-task review:**
Each backend task ends with `go test ./...` green; each frontend task ends with `npx tsc --noEmit` clean. The `/codex:review --wait --base <SHA>` invocation between tasks is documented in the plan header and is the agreed cadence — it isn't repeated in every step to keep tasks readable.

---

**Plan complete.**
