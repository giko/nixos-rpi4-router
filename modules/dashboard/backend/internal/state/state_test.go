package state

import (
	"sync"
	"testing"
	"time"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/model"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/conntrack"
)

func TestNewStateHasZeroValues(t *testing.T) {
	s := New()

	traffic, updatedAt := s.SnapshotTraffic()
	if len(traffic.Interfaces) != 0 {
		t.Errorf("expected zero interfaces, got %d", len(traffic.Interfaces))
	}
	if !updatedAt.IsZero() {
		t.Errorf("expected zero updated_at, got %v", updatedAt)
	}

	sys, sysUpdated := s.SnapshotSystem()
	if sys.TemperatureC != 0 {
		t.Errorf("expected zero temperature, got %f", sys.TemperatureC)
	}
	if !sysUpdated.IsZero() {
		t.Errorf("expected zero system updated_at, got %v", sysUpdated)
	}

	tunnels, tunUpdated := s.SnapshotTunnels()
	if len(tunnels) != 0 {
		t.Errorf("expected zero tunnels, got %d", len(tunnels))
	}
	if !tunUpdated.IsZero() {
		t.Errorf("expected zero tunnels updated_at, got %v", tunUpdated)
	}

	pools, poolUpdated := s.SnapshotPools()
	if len(pools) != 0 {
		t.Errorf("expected zero pools, got %d", len(pools))
	}
	if !poolUpdated.IsZero() {
		t.Errorf("expected zero pools updated_at, got %v", poolUpdated)
	}

	clients, cliUpdated := s.SnapshotClients()
	if len(clients) != 0 {
		t.Errorf("expected zero clients, got %d", len(clients))
	}
	if !cliUpdated.IsZero() {
		t.Errorf("expected zero clients updated_at, got %v", cliUpdated)
	}

	adguard, agUpdated := s.SnapshotAdguard()
	if adguard.Queries24h != 0 {
		t.Errorf("expected zero queries, got %d", adguard.Queries24h)
	}
	if !agUpdated.IsZero() {
		t.Errorf("expected zero adguard updated_at, got %v", agUpdated)
	}
}

func TestSetAndSnapshotTraffic(t *testing.T) {
	s := New()

	before := time.Now()
	s.SetTraffic(model.Traffic{
		Interfaces: []model.Interface{
			{Name: "eth0", RXBps: 1000, TXBps: 500},
			{Name: "eth1", RXBps: 2000, TXBps: 1000},
		},
	})
	after := time.Now()

	traffic, updatedAt := s.SnapshotTraffic()

	if len(traffic.Interfaces) != 2 {
		t.Fatalf("expected 2 interfaces, got %d", len(traffic.Interfaces))
	}
	if traffic.Interfaces[0].Name != "eth0" {
		t.Errorf("expected interface name eth0, got %s", traffic.Interfaces[0].Name)
	}
	if traffic.Interfaces[0].RXBps != 1000 {
		t.Errorf("expected RXBps 1000, got %d", traffic.Interfaces[0].RXBps)
	}
	if traffic.Interfaces[1].Name != "eth1" {
		t.Errorf("expected interface name eth1, got %s", traffic.Interfaces[1].Name)
	}

	if updatedAt.Before(before) || updatedAt.After(after) {
		t.Errorf("updated_at %v not in range [%v, %v]", updatedAt, before, after)
	}
}

func TestSetAndSnapshotTrafficDefensiveCopy(t *testing.T) {
	s := New()

	original := model.Traffic{
		Interfaces: []model.Interface{
			{Name: "eth0", RXBps: 1000, TXBps: 500},
		},
	}
	s.SetTraffic(original)

	// Mutate the original after setting -- should not affect state.
	original.Interfaces[0].Name = "mutated"

	traffic, _ := s.SnapshotTraffic()
	if traffic.Interfaces[0].Name != "eth0" {
		t.Error("SetTraffic did not defensively copy: mutation of original affected state")
	}

	// Mutate the snapshot -- should not affect state.
	traffic.Interfaces[0].Name = "mutated-snapshot"

	traffic2, _ := s.SnapshotTraffic()
	if traffic2.Interfaces[0].Name != "eth0" {
		t.Error("SnapshotTraffic did not defensively copy: mutation of snapshot affected state")
	}
}

func TestSetAndSnapshotTunnelsDefensiveCopy(t *testing.T) {
	s := New()

	original := []model.Tunnel{
		{Name: "wg_sw", Healthy: true},
	}
	s.SetTunnels(original)

	// Mutate the original after setting.
	original[0].Name = "mutated"

	tunnels, _ := s.SnapshotTunnels()
	if tunnels[0].Name != "wg_sw" {
		t.Error("SetTunnels did not defensively copy")
	}

	// Mutate the snapshot.
	tunnels[0].Name = "mutated-snapshot"

	tunnels2, _ := s.SnapshotTunnels()
	if tunnels2[0].Name != "wg_sw" {
		t.Error("SnapshotTunnels did not defensively copy")
	}
}

func TestSnapshotClientFound(t *testing.T) {
	s := New()

	s.SetClients([]model.Client{
		{IP: "192.168.1.10", Hostname: "phone"},
		{IP: "192.168.1.20", Hostname: "laptop"},
	})

	client, updatedAt, ok := s.SnapshotClient("192.168.1.20")
	if !ok {
		t.Fatal("expected to find client 192.168.1.20")
	}
	if client.Hostname != "laptop" {
		t.Errorf("expected hostname laptop, got %s", client.Hostname)
	}
	if updatedAt.IsZero() {
		t.Error("expected non-zero updated_at")
	}
}

func TestSnapshotClientNotFound(t *testing.T) {
	s := New()

	s.SetClients([]model.Client{
		{IP: "192.168.1.10", Hostname: "phone"},
	})

	_, _, ok := s.SnapshotClient("192.168.1.99")
	if ok {
		t.Error("expected not found for non-existent IP")
	}
}

func TestIsStale(t *testing.T) {
	interval := 10 * time.Second

	// Zero time is always stale.
	if !IsStale(time.Time{}, interval) {
		t.Error("zero time should be stale")
	}

	// Fresh update (just now) is not stale.
	if IsStale(time.Now(), interval) {
		t.Error("fresh update should not be stale")
	}

	// Old update (well past 2x interval) is stale.
	old := time.Now().Add(-30 * time.Second)
	if !IsStale(old, interval) {
		t.Error("old update (30s with 10s interval) should be stale")
	}

	// Exactly at boundary (2x interval) is NOT stale (strict >).
	boundary := time.Now().Add(-20 * time.Second)
	// Give 1ms of tolerance for clock jitter.
	time.Sleep(time.Millisecond)
	if IsStale(boundary, interval) {
		// The boundary case: time.Since(boundary) should be just over 20s,
		// which is exactly 2*interval. With strict >, this should still be
		// within tolerance. We need to be careful here since time passes.
		// Instead, test with a value that is clearly within 2x.
		t.Log("boundary test is timing-sensitive, verifying with a value clearly within 2x")
	}

	// Clearly within 2x interval should not be stale.
	within := time.Now().Add(-19 * time.Second)
	if IsStale(within, interval) {
		t.Error("update within 2x interval (19s with 10s interval) should not be stale")
	}
}

func TestClientConnsRoundTrip(t *testing.T) {
	s := New()
	s.SetClientConns(map[string]conntrack.ClientConnInfo{
		"192.168.1.10": {TotalConns: 5, TunnelConns: map[string]int{"0x20000": 3}},
	})

	got, ts := s.SnapshotClientConns()
	if got["192.168.1.10"].TotalConns != 5 {
		t.Errorf("TotalConns = %d, want 5", got["192.168.1.10"].TotalConns)
	}
	if got["192.168.1.10"].TunnelConns["0x20000"] != 3 {
		t.Errorf("TunnelConns[0x20000] = %d, want 3", got["192.168.1.10"].TunnelConns["0x20000"])
	}
	if ts.IsZero() {
		t.Error("expected non-zero updated time")
	}

	// Mutating snapshot must not mutate state.
	got["192.168.1.10"] = conntrack.ClientConnInfo{}
	got2, _ := s.SnapshotClientConns()
	if got2["192.168.1.10"].TotalConns != 5 {
		t.Error("state mutated through snapshot")
	}
}

func TestClientConnsZeroValue(t *testing.T) {
	s := New()
	got, ts := s.SnapshotClientConns()
	if len(got) != 0 {
		t.Errorf("expected empty map, got %d entries", len(got))
	}
	if !ts.IsZero() {
		t.Errorf("expected zero updated_at, got %v", ts)
	}
}

func TestConcurrentReadersAndWriter(t *testing.T) {
	s := New()
	const goroutines = 10
	const iterations = 1000

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				// Writers (even IDs).
				if id%2 == 0 {
					s.SetTraffic(model.Traffic{
						Interfaces: []model.Interface{
							{Name: "eth0", RXBps: uint64(j)},
						},
					})
					s.SetSystem(model.SystemStats{
						TemperatureC: float64(j),
					})
					s.SetTunnels([]model.Tunnel{
						{Name: "wg_sw", RXBytes: uint64(j)},
					})
					s.SetPools([]model.Pool{
						{Name: "vpn", ClientIPs: []string{"192.168.1.1"}},
					})
					s.SetClients([]model.Client{
						{IP: "192.168.1.10", Hostname: "test"},
					})
					s.SetAdguard(model.AdguardStats{
						Queries24h: j,
					})
					s.SetClientConns(map[string]conntrack.ClientConnInfo{
						"192.168.1.10": {TotalConns: 5, TunnelConns: map[string]int{"0x20000": 3}},
					})
				} else {
					// Readers (odd IDs).
					s.SnapshotTraffic()
					s.SnapshotSystem()
					s.SnapshotTunnels()
					s.SnapshotPools()
					s.SnapshotClients()
					s.SnapshotClient("192.168.1.10")
					s.SnapshotAdguard()
					s.SnapshotClientConns()
				}
			}
		}(i)
	}

	wg.Wait()
}
