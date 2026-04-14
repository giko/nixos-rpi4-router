package collector

import (
	"context"
	"io"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/adguard"
)

type stubAdguard struct {
	rows []string // already-encoded JSON rows
}

func (s stubAdguard) FetchQueryLogPage(_ context.Context, _ time.Time, _ int) (adguard.QueryLogResponse, error) {
	body := "[" + strings.Join(s.rows, ",") + "]"
	return adguard.QueryLogResponse{Data: []byte(body)}, nil
}

func TestClientDetailRuntimeLifecycleFanout(t *testing.T) {
	rt := NewClientDetailRuntime(ClientDetailOpts{LeaseGrace: time.Minute})
	ip := netip.MustParseAddr("192.168.1.42")

	now := time.Now()
	rt.OnLeaseScan([]netip.Addr{ip}, now)
	if got := rt.Lifecycle.Status(ip); got != LeaseStatusDynamic {
		t.Fatalf("Status after birth = %q", got)
	}
	// All four per-client collectors should accept Drop without panic
	// — proves they were Tracked.
	rt.Traffic.Drop(ip)
	rt.FlowCount.Drop(ip)
	rt.DnsRate.Drop(ip)
	rt.TopDestinations.Drop(ip)
}

func TestClientDetailRuntimeReapClearsState(t *testing.T) {
	rt := NewClientDetailRuntime(ClientDetailOpts{LeaseGrace: time.Millisecond})
	ip := netip.MustParseAddr("192.168.1.42")

	t0 := time.Now()
	rt.OnLeaseScan([]netip.Addr{ip}, t0)
	rt.OnLeaseScan(nil, t0.Add(2*time.Millisecond)) // death
	reaped := rt.Reap(t0.Add(time.Second))
	if len(reaped) != 1 || reaped[0] != ip {
		t.Fatalf("Reap = %v", reaped)
	}
	if rt.Lifecycle.Status(ip) != LeaseStatusUnknown {
		t.Errorf("Status after reap = %q", rt.Lifecycle.Status(ip))
	}
}

func TestClientDetailRuntimeIngestTickFansToCollectors(t *testing.T) {
	row := `{"time":"2026-04-14T08:00:00.000000000Z","client":"192.168.1.42","question":{"name":"example.com","type":"A"},"answer":[{"value":"93.184.216.34","ttl":300}],"reason":"NotFiltered"}`
	rt := NewClientDetailRuntime(ClientDetailOpts{
		AdguardIngest: stubAdguard{rows: []string{row}},
	})
	rt.OnLeaseScan([]netip.Addr{netip.MustParseAddr("192.168.1.42")}, time.Now())

	if err := rt.IngestTick(context.Background(), time.Now()); err != nil {
		t.Fatal(err)
	}
	if d, ok := rt.Domains.Lookup(
		netip.MustParseAddr("192.168.1.42"),
		netip.MustParseAddr("93.184.216.34"),
		time.Now(),
	); !ok || d != "example.com" {
		t.Errorf("enricher missed: domain=%q ok=%v", d, ok)
	}
}

func TestClientDetailRuntimeTickWithoutReaderIsNoop(t *testing.T) {
	rt := NewClientDetailRuntime(ClientDetailOpts{})
	if err := rt.Tick(time.Now()); err != nil {
		t.Fatal(err)
	}
	if got := rt.Snapshot(); got == nil {
		t.Errorf("Snapshot should be empty slice (not nil) after no-op Tick")
	}
}

// Verifies the runtime works end-to-end with a fixture-provided conntrack reader.
func TestClientDetailRuntimeTickPopulatesSnapshot(t *testing.T) {
	fixture := `ipv4     2 tcp      6 432000 ESTABLISHED src=192.168.1.42 dst=8.8.8.8 sport=51000 dport=443 packets=10 bytes=500 src=8.8.8.8 dst=198.51.100.1 sport=443 dport=51000 packets=12 bytes=30000 [ASSURED] mark=0 use=1
`
	rt := NewClientDetailRuntime(ClientDetailOpts{
		LANPrefixes: []netip.Prefix{netip.MustParsePrefix("192.168.1.0/24")},
		ConntrackReader: func() (io.ReadCloser, error) {
			return io.NopCloser(strings.NewReader(fixture)), nil
		},
	})
	ip := netip.MustParseAddr("192.168.1.42")
	rt.OnLeaseScan([]netip.Addr{ip}, time.Now())

	if err := rt.Tick(time.Now()); err != nil {
		t.Fatal(err)
	}
	flows := rt.Snapshot()
	if len(flows) != 1 {
		t.Fatalf("Snapshot len = %d", len(flows))
	}
	if flows[0].ClientIP != ip {
		t.Errorf("ClientIP = %v", flows[0].ClientIP)
	}
}
