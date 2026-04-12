package collector

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/model"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/state"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/topology"
)

func TestPoolsCollectorDerivesMembership(t *testing.T) {
	// Write pool-health fixture: wg_sw healthy, wg_us unhealthy.
	phDir := t.TempDir()
	phPath := filepath.Join(phDir, "state.json")
	phData := `{
  "updated_at": "2026-04-11T12:00:00Z",
  "tunnels": {
    "wg_sw": { "healthy": true, "consecutive_failures": 0 },
    "wg_us": { "healthy": false, "consecutive_failures": 3 }
  }
}`
	if err := os.WriteFile(phPath, []byte(phData), 0644); err != nil {
		t.Fatal(err)
	}

	topo := &topology.Topology{
		Tunnels: []topology.Tunnel{
			{Name: "wg_sw", Interface: "wg_sw", Fwmark: "0x20000", RoutingTable: 200},
			{Name: "wg_us", Interface: "wg_us", Fwmark: "0x30000", RoutingTable: 300},
		},
		Pools: []topology.Pool{
			{Name: "vpn_pool", Members: []string{"wg_sw", "wg_us"}},
		},
		PooledRules: []topology.PooledRule{
			{Sources: []string{"192.168.1.10", "192.168.1.11"}, Pool: "vpn_pool"},
			{Sources: []string{"192.168.1.12"}, Pool: "vpn_pool"},
		},
	}

	st := state.New()

	c := NewPools(PoolsOpts{
		Topology:       topo,
		PoolHealthPath: phPath,
		State:          st,
	})

	if c.Name() != "pools" {
		t.Errorf("Name() = %q, want %q", c.Name(), "pools")
	}
	if c.Tier() != Hot {
		t.Errorf("Tier() = %v, want Hot", c.Tier())
	}

	if err := c.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	pools, updated := st.SnapshotPools()
	if updated.IsZero() {
		t.Fatal("expected non-zero updated time")
	}
	if len(pools) != 1 {
		t.Fatalf("expected 1 pool, got %d", len(pools))
	}

	p := pools[0]
	if p.Name != "vpn_pool" {
		t.Errorf("pool name = %q, want %q", p.Name, "vpn_pool")
	}

	// Verify members.
	if len(p.Members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(p.Members))
	}

	sw := p.Members[0]
	if sw.Tunnel != "wg_sw" {
		t.Errorf("members[0].Tunnel = %q, want %q", sw.Tunnel, "wg_sw")
	}
	if sw.Fwmark != "0x20000" {
		t.Errorf("members[0].Fwmark = %q, want %q", sw.Fwmark, "0x20000")
	}
	if !sw.Healthy {
		t.Error("members[0] should be healthy")
	}
	if sw.FlowCount != 0 {
		t.Errorf("members[0].FlowCount = %d, want 0", sw.FlowCount)
	}

	us := p.Members[1]
	if us.Tunnel != "wg_us" {
		t.Errorf("members[1].Tunnel = %q, want %q", us.Tunnel, "wg_us")
	}
	if us.Fwmark != "0x30000" {
		t.Errorf("members[1].Fwmark = %q, want %q", us.Fwmark, "0x30000")
	}
	if us.Healthy {
		t.Error("members[1] should not be healthy")
	}

	// 3 client IPs from 2 pooled rules.
	if len(p.ClientIPs) != 3 {
		t.Fatalf("expected 3 client IPs, got %d", len(p.ClientIPs))
	}
	wantIPs := map[string]bool{
		"192.168.1.10": true,
		"192.168.1.11": true,
		"192.168.1.12": true,
	}
	for _, ip := range p.ClientIPs {
		if !wantIPs[ip] {
			t.Errorf("unexpected client IP %q", ip)
		}
	}

	// Failsafe NOT active (wg_sw is healthy).
	if p.FailsafeDropActive {
		t.Error("failsafe_drop_active should be false when at least one member is healthy")
	}
}

func TestPoolsCollectorFailsafeDrop(t *testing.T) {
	// All members unhealthy.
	phDir := t.TempDir()
	phPath := filepath.Join(phDir, "state.json")
	phData := `{
  "updated_at": "2026-04-11T12:00:00Z",
  "tunnels": {
    "wg_sw": { "healthy": false, "consecutive_failures": 5 },
    "wg_us": { "healthy": false, "consecutive_failures": 3 }
  }
}`
	if err := os.WriteFile(phPath, []byte(phData), 0644); err != nil {
		t.Fatal(err)
	}

	topo := &topology.Topology{
		Tunnels: []topology.Tunnel{
			{Name: "wg_sw", Interface: "wg_sw", Fwmark: "0x20000", RoutingTable: 200},
			{Name: "wg_us", Interface: "wg_us", Fwmark: "0x30000", RoutingTable: 300},
		},
		Pools: []topology.Pool{
			{Name: "vpn_pool", Members: []string{"wg_sw", "wg_us"}},
		},
		PooledRules: []topology.PooledRule{
			{Sources: []string{"192.168.1.10"}, Pool: "vpn_pool"},
		},
	}

	st := state.New()

	c := NewPools(PoolsOpts{
		Topology:       topo,
		PoolHealthPath: phPath,
		State:          st,
	})

	if err := c.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	pools, _ := st.SnapshotPools()
	if len(pools) != 1 {
		t.Fatalf("expected 1 pool, got %d", len(pools))
	}

	if !pools[0].FailsafeDropActive {
		t.Error("failsafe_drop_active should be true when all members are unhealthy")
	}

	// Verify both members are unhealthy.
	for i, m := range pools[0].Members {
		if m.Healthy {
			t.Errorf("members[%d] (%s) should not be healthy", i, m.Tunnel)
		}
	}
}

func TestPoolsCollectorPreservesFlowCount(t *testing.T) {
	phDir := t.TempDir()
	phPath := filepath.Join(phDir, "state.json")
	phData := `{
  "updated_at": "2026-04-11T12:00:00Z",
  "tunnels": {
    "wg_sw": { "healthy": true, "consecutive_failures": 0 }
  }
}`
	if err := os.WriteFile(phPath, []byte(phData), 0644); err != nil {
		t.Fatal(err)
	}

	topo := &topology.Topology{
		Tunnels: []topology.Tunnel{
			{Name: "wg_sw", Interface: "wg_sw", Fwmark: "0x20000", RoutingTable: 200},
		},
		Pools: []topology.Pool{
			{Name: "vpn_pool", Members: []string{"wg_sw"}},
		},
	}

	st := state.New()

	// Pre-populate state with a flow count (as if cold tier wrote it).
	st.SetPools([]model.Pool{
		{
			Name: "vpn_pool",
			Members: []model.PoolMember{
				{Tunnel: "wg_sw", Fwmark: "0x20000", Healthy: true, FlowCount: 42},
			},
		},
	})

	c := NewPools(PoolsOpts{
		Topology:       topo,
		PoolHealthPath: phPath,
		State:          st,
	})

	if err := c.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	pools, _ := st.SnapshotPools()
	if len(pools) != 1 {
		t.Fatalf("expected 1 pool, got %d", len(pools))
	}
	if len(pools[0].Members) != 1 {
		t.Fatalf("expected 1 member, got %d", len(pools[0].Members))
	}
	if pools[0].Members[0].FlowCount != 42 {
		t.Errorf("FlowCount = %d, want 42 (should be preserved from previous state)", pools[0].Members[0].FlowCount)
	}
}
