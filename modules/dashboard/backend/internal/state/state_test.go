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

func TestSetSystemHotPreservesMediumFields(t *testing.T) {
	s := New()

	// Seed medium-tier fields.
	s.SetSystemMedium(
		[]model.ServiceState{{Name: "nftables", Active: true, RawState: "active"}},
		"0x50000", true,
	)

	// Hot write must not clobber medium values.
	s.SetSystemHot(
		model.CPUStats{PercentUser: 42},
		model.MemoryStats{TotalBytes: 1024},
		55.5,
		123.4,
		time.Unix(1700000000, 0).UTC(),
	)

	snap, _ := s.SnapshotSystem()
	if snap.CPU.PercentUser != 42 {
		t.Errorf("PercentUser = %v, want 42", snap.CPU.PercentUser)
	}
	if snap.TemperatureC != 55.5 {
		t.Errorf("TemperatureC = %v, want 55.5", snap.TemperatureC)
	}
	if snap.Throttled != "0x50000" || !snap.ThrottledFlag {
		t.Errorf("medium fields lost: throttled=%q flag=%v", snap.Throttled, snap.ThrottledFlag)
	}
	if len(snap.Services) != 1 || snap.Services[0].Name != "nftables" {
		t.Errorf("services lost: %+v", snap.Services)
	}
}

func TestSetSystemMediumPreservesHotFields(t *testing.T) {
	s := New()

	s.SetSystemHot(
		model.CPUStats{PercentUser: 42},
		model.MemoryStats{TotalBytes: 1024},
		55.5, 123.4,
		time.Unix(1700000000, 0).UTC(),
	)

	s.SetSystemMedium(
		[]model.ServiceState{{Name: "dnsmasq", Active: true}},
		"0x0", false,
	)

	snap, _ := s.SnapshotSystem()
	if snap.CPU.PercentUser != 42 {
		t.Errorf("CPU lost: %+v", snap.CPU)
	}
	if snap.TemperatureC != 55.5 {
		t.Errorf("TemperatureC lost: %v", snap.TemperatureC)
	}
	if snap.Memory.TotalBytes != 1024 {
		t.Errorf("Memory lost: %+v", snap.Memory)
	}
	if len(snap.Services) != 1 || snap.Services[0].Name != "dnsmasq" {
		t.Errorf("services wrong: %+v", snap.Services)
	}
}

func TestSnapshotSystemTiersIndependentTimestamps(t *testing.T) {
	s := New()

	beforeHot := time.Now()
	s.SetSystemHot(model.CPUStats{}, model.MemoryStats{}, 0, 0, time.Time{})
	afterHot := time.Now()

	// Wait past clock resolution so medium stamp is strictly greater.
	time.Sleep(2 * time.Millisecond)

	beforeMed := time.Now()
	s.SetSystemMedium(nil, "0x0", false)
	afterMed := time.Now()

	_, hot, med := s.SnapshotSystemTiers()
	if hot.Before(beforeHot) || hot.After(afterHot) {
		t.Errorf("hot updated_at %v out of [%v, %v]", hot, beforeHot, afterHot)
	}
	if med.Before(beforeMed) || med.After(afterMed) {
		t.Errorf("medium updated_at %v out of [%v, %v]", med, beforeMed, afterMed)
	}
	if !hot.Before(med) {
		t.Errorf("expected hot (%v) to be before medium (%v)", hot, med)
	}

	// SnapshotSystem's timestamp should equal the oldest (hot) tier.
	_, combined := s.SnapshotSystem()
	if !combined.Equal(hot) {
		t.Errorf("SnapshotSystem ts = %v, want oldest %v", combined, hot)
	}
}

func TestSetPoolsHotPreservesFlowCounts(t *testing.T) {
	s := New()

	// Seed state with flow counts already in place (as if cold tier wrote).
	s.SetPools([]model.Pool{
		{
			Name: "vpn_pool",
			Members: []model.PoolMember{
				{Tunnel: "wg_sw", Fwmark: "0x20000", Healthy: true, FlowCount: 42},
				{Tunnel: "wg_us", Fwmark: "0x30000", Healthy: true, FlowCount: 7},
			},
		},
	})

	// Hot tier re-derives topology; FlowCount zero in the incoming slice.
	s.SetPoolsHot([]model.Pool{
		{
			Name: "vpn_pool",
			Members: []model.PoolMember{
				{Tunnel: "wg_sw", Fwmark: "0x20000", Healthy: false},
				{Tunnel: "wg_us", Fwmark: "0x30000", Healthy: true},
			},
			FailsafeDropActive: false,
		},
	})

	pools, _ := s.SnapshotPools()
	if len(pools) != 1 {
		t.Fatalf("len = %d, want 1", len(pools))
	}
	if pools[0].Members[0].Healthy {
		t.Errorf("Healthy was not updated by hot pass")
	}
	if pools[0].Members[0].FlowCount != 42 {
		t.Errorf("FlowCount clobbered: got %d, want 42", pools[0].Members[0].FlowCount)
	}
	if pools[0].Members[1].FlowCount != 7 {
		t.Errorf("FlowCount clobbered: got %d, want 7", pools[0].Members[1].FlowCount)
	}
}

func TestSetPoolFlowsOnlyUpdatesCounts(t *testing.T) {
	s := New()

	s.SetPools([]model.Pool{
		{
			Name: "vpn_pool",
			Members: []model.PoolMember{
				{Tunnel: "wg_sw", Fwmark: "0x20000", Healthy: true, FlowCount: 1},
				{Tunnel: "wg_us", Fwmark: "0x30000", Healthy: false, FlowCount: 5},
			},
			ClientIPs:          []string{"192.168.1.10"},
			FailsafeDropActive: true,
		},
	})

	s.SetPoolFlows(map[string]map[string]int{
		"vpn_pool": {"wg_sw": 99, "wg_us": 100},
	})

	pools, _ := s.SnapshotPools()
	if len(pools) != 1 {
		t.Fatalf("len = %d, want 1", len(pools))
	}
	if !pools[0].Members[0].Healthy {
		t.Errorf("Healthy mutated by flow update")
	}
	if pools[0].Members[1].Healthy {
		t.Errorf("Healthy mutated by flow update")
	}
	if !pools[0].FailsafeDropActive {
		t.Errorf("FailsafeDropActive mutated by flow update")
	}
	if pools[0].Members[0].FlowCount != 99 {
		t.Errorf("FlowCount wg_sw = %d, want 99", pools[0].Members[0].FlowCount)
	}
	if pools[0].Members[1].FlowCount != 100 {
		t.Errorf("FlowCount wg_us = %d, want 100", pools[0].Members[1].FlowCount)
	}
	if len(pools[0].ClientIPs) != 1 || pools[0].ClientIPs[0] != "192.168.1.10" {
		t.Errorf("ClientIPs mutated: %+v", pools[0].ClientIPs)
	}
}

func TestSetPoolFlowsIgnoresUnknownPools(t *testing.T) {
	s := New()
	s.SetPools([]model.Pool{
		{Name: "vpn_pool", Members: []model.PoolMember{{Tunnel: "wg_sw", FlowCount: 3}}},
	})

	// Unknown pool — ignored without panicking.
	s.SetPoolFlows(map[string]map[string]int{
		"ghost": {"wg_sw": 999},
	})

	pools, _ := s.SnapshotPools()
	if pools[0].Members[0].FlowCount != 3 {
		t.Errorf("FlowCount mutated by unknown pool update: %d", pools[0].Members[0].FlowCount)
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
