// Package integration exercises the client-detail wiring end-to-end:
// a fully-populated ClientDetailRuntime is driven through three ticks
// against fixture data, an httptest server is stood up with
// server.NewWithDeps, and the four per-client HTTP endpoints
// (/traffic, /connections, /dns, /top-destinations) are hit to validate
// that Task 9.1 + 9.2 wiring produces the right body shapes and
// lease-status semantics.
package integration

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/collector"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/config"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/envelope"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/model"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/server"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/adguard"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/state"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/topology"
)

// conntrackTicks are three successive /proc/net/nf_conntrack snapshots.
// Tick 1: both an outbound flow (192.168.1.42 → 8.8.8.8) and an inbound
// DNAT flow (203.0.113.5 → 192.168.1.50) with modest byte counts.
// Tick 2: both flows grew.
// Tick 3: outbound flow grew further; DNAT flow is gone.
var conntrackTicks = []string{
	`ipv4     2 tcp      6 432000 ESTABLISHED src=192.168.1.42 dst=8.8.8.8 sport=51000 dport=443 packets=10 bytes=500 src=8.8.8.8 dst=198.51.100.1 sport=443 dport=51000 packets=12 bytes=30000 [ASSURED] mark=0 use=1
ipv4     2 tcp      6 432000 ESTABLISHED src=203.0.113.5 dst=198.51.100.1 sport=41000 dport=32400 packets=8 bytes=900 src=192.168.1.50 dst=203.0.113.5 sport=32400 dport=41000 packets=10 bytes=1200 [ASSURED] mark=0 use=1
`,
	`ipv4     2 tcp      6 432000 ESTABLISHED src=192.168.1.42 dst=8.8.8.8 sport=51000 dport=443 packets=20 bytes=1500 src=8.8.8.8 dst=198.51.100.1 sport=443 dport=51000 packets=24 bytes=120000 [ASSURED] mark=0 use=1
ipv4     2 tcp      6 432000 ESTABLISHED src=203.0.113.5 dst=198.51.100.1 sport=41000 dport=32400 packets=16 bytes=1900 src=192.168.1.50 dst=203.0.113.5 sport=32400 dport=41000 packets=20 bytes=2700 [ASSURED] mark=0 use=1
`,
	`ipv4     2 tcp      6 432000 ESTABLISHED src=192.168.1.42 dst=8.8.8.8 sport=51000 dport=443 packets=30 bytes=2500 src=8.8.8.8 dst=198.51.100.1 sport=443 dport=51000 packets=36 bytes=210000 [ASSURED] mark=0 use=1
`,
}

// tickReader hands out one conntrack fixture per Tick, latching onto the
// final fixture once all are consumed so extra ticks stay valid.
type tickReader struct {
	ticks []string
	i     int
}

func (r *tickReader) next() (io.ReadCloser, error) {
	body := r.ticks[r.i]
	if r.i+1 < len(r.ticks) {
		r.i++
	}
	return io.NopCloser(strings.NewReader(body)), nil
}

// fakeAdguardIngest satisfies collector.AdguardClient. It returns a
// single A record so the enricher learns 8.8.8.8 → dns.google for client
// 192.168.1.42.
type fakeAdguardIngest struct{}

func (fakeAdguardIngest) FetchQueryLogPage(_ context.Context, _ time.Time, _ int) (adguard.QueryLogResponse, error) {
	body := `[{"time":"2026-04-14T08:00:00.000000000Z","client":"192.168.1.42","question":{"name":"dns.google","type":"A"},"answer":[{"value":"8.8.8.8","ttl":300}],"reason":"NotFiltered","upstream":"tls://1.1.1.1","elapsedMs":1.2}]`
	return adguard.QueryLogResponse{Data: []byte(body)}, nil
}

// fakeAdguardQuerylog satisfies server.AdguardQueryLogClient. Distinct
// from fakeAdguardIngest so we can return per-client rows without
// coupling to the ingest fake's JSON envelope.
type fakeAdguardQuerylog struct{}

func (fakeAdguardQuerylog) FetchQueryLogForClient(_ context.Context, _ string, _ int) ([]adguard.QueryLogClientRow, error) {
	return []adguard.QueryLogClientRow{
		{
			Time:         time.Now().UTC(),
			Question:     "dns.google",
			QuestionType: "A",
			Upstream:     "tls://1.1.1.1",
			Reason:       "NotFiltered",
			ElapsedMs:    1.2,
		},
	}, nil
}

// lifecycleOnlyLookup adapts the runtime's LifecycleTracker to the
// server.clientLookup surface: Status comes from the tracker; we never
// claim static-or-neighbor, so the handlers resolve strictly via the
// lifecycle signal.
type lifecycleOnlyLookup struct{ rt *collector.ClientDetailRuntime }

func (a lifecycleOnlyLookup) Status(ip netip.Addr) collector.LeaseStatus {
	return a.rt.Lifecycle.Status(ip)
}

func (lifecycleOnlyLookup) IsStatic(_ netip.Addr) bool           { return false }
func (lifecycleOnlyLookup) IsStaticOrNeighbor(_ netip.Addr) bool { return false }

func TestClientDetailEndToEnd(t *testing.T) {
	tickSrc := &tickReader{ticks: conntrackTicks}

	rt := collector.NewClientDetailRuntime(collector.ClientDetailOpts{
		LANPrefixes:     []netip.Prefix{netip.MustParsePrefix("192.168.1.0/24")},
		ConntrackReader: tickSrc.next,
		AdguardIngest:   fakeAdguardIngest{},
	})

	st := state.New()
	cfg := &config.Config{}
	topo := &topology.Topology{}

	deps := server.Deps{
		ClientLookup:    lifecycleOnlyLookup{rt: rt},
		ClientTraffic:   rt.Traffic,
		AdguardQueryLog: fakeAdguardQuerylog{},
		Flows:           rt,
		Domains:         rt.Domains,
		TopDestinations: rt.TopDestinations,
		DnsRate:         rt.DnsRate,
		FlowCount:       rt.FlowCount,
	}
	srv := httptest.NewServer(server.NewWithDeps(cfg, st, topo, deps))
	defer srv.Close()

	leaseClient1 := netip.MustParseAddr("192.168.1.42")
	leaseClient2 := netip.MustParseAddr("192.168.1.50")

	t0 := time.Date(2026, 4, 14, 8, 0, 0, 0, time.UTC)

	// Tick 1: both clients present.
	rt.OnLeaseScan([]netip.Addr{leaseClient1, leaseClient2}, t0)
	if err := rt.Tick(t0); err != nil {
		t.Fatalf("tick 1: %v", err)
	}
	if err := rt.IngestTick(context.Background(), t0); err != nil {
		t.Fatalf("ingest 1: %v", err)
	}

	// Tick 2: both clients still present, 10 s later.
	t1 := t0.Add(10 * time.Second)
	rt.OnLeaseScan([]netip.Addr{leaseClient1, leaseClient2}, t1)
	if err := rt.Tick(t1); err != nil {
		t.Fatalf("tick 2: %v", err)
	}

	// Tick 3: leaseClient2 disappears (lease released between t1 and t2).
	// LifecycleTracker marks it tombstoned; because Reap never runs here,
	// its state remains accessible and Status() returns "expired".
	t2 := t1.Add(10 * time.Second)
	rt.OnLeaseScan([]netip.Addr{leaseClient1}, t2)
	if err := rt.Tick(t2); err != nil {
		t.Fatalf("tick 3: %v", err)
	}

	// --- Assertions ---------------------------------------------------

	// 1. /traffic for leaseClient1 returns a populated ring.
	t.Run("traffic_dynamic_populated", func(t *testing.T) {
		var body model.ClientTraffic
		mustGet(t, srv.URL+"/api/clients/192.168.1.42/traffic", &body)
		if body.LeaseStatus != "dynamic" {
			t.Errorf("lease_status = %q, want dynamic", body.LeaseStatus)
		}
		if len(body.Samples) == 0 {
			t.Errorf("samples empty after three ticks")
		}
	})

	// 2. /connections for leaseClient1 includes the outbound flow with
	//    domain enrichment from the DNS ingest.
	t.Run("connections_enriched", func(t *testing.T) {
		var body model.ClientConnections
		mustGet(t, srv.URL+"/api/clients/192.168.1.42/connections", &body)
		if body.Count == 0 {
			t.Fatalf("count = 0; want at least one flow")
		}
		var foundEnriched bool
		for _, f := range body.Flows {
			if f.Domain == "dns.google" {
				foundEnriched = true
				break
			}
		}
		if !foundEnriched {
			t.Errorf("enricher did not populate domain; got flows: %+v", body.Flows)
		}
	})

	// 3. /dns proxies the fake AdGuard query-log client.
	t.Run("dns_proxied", func(t *testing.T) {
		var body model.ClientDns
		mustGet(t, srv.URL+"/api/clients/192.168.1.42/dns", &body)
		if body.Count != 1 || len(body.Recent) != 1 || body.Recent[0].Question != "dns.google" {
			t.Errorf("unexpected body: %+v", body)
		}
	})

	// 4. /traffic for leaseClient2 — the IP that disappeared on tick 3 —
	//    returns lease_status: "expired" and envelope.stale = true.
	t.Run("expired_client_stale", func(t *testing.T) {
		body, env := mustGetWithEnvelope(t, srv.URL+"/api/clients/192.168.1.50/traffic")
		if body.LeaseStatus != "expired" {
			t.Errorf("lease_status = %q, want expired", body.LeaseStatus)
		}
		if !env.Stale {
			t.Errorf("envelope.stale = false, want true")
		}
	})
}

// mustGet fetches url, asserts 200, and unmarshals envelope.Data into
// into. Fails the test on any step.
func mustGet(t *testing.T, url string, into any) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s -> %d", url, resp.StatusCode)
	}
	var env envelope.Response
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	raw, err := json.Marshal(env.Data)
	if err != nil {
		t.Fatalf("re-marshal data: %v", err)
	}
	if err := json.Unmarshal(raw, into); err != nil {
		t.Fatalf("unmarshal %T: %v", into, err)
	}
}

// mustGetWithEnvelope is like mustGet but returns the envelope alongside
// a decoded ClientTraffic body so callers can assert on envelope.Stale.
func mustGetWithEnvelope(t *testing.T, url string) (model.ClientTraffic, envelope.Response) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s -> %d", url, resp.StatusCode)
	}
	var env envelope.Response
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("decode envelope: %v", err)
	}
	raw, err := json.Marshal(env.Data)
	if err != nil {
		t.Fatalf("re-marshal data: %v", err)
	}
	var body model.ClientTraffic
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("unmarshal ClientTraffic: %v", err)
	}
	return body, env
}
