package collector

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/wireguard"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/state"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/topology"
)

func TestTunnelsCollector(t *testing.T) {
	// Fake ShowFunc returning different dumps per interface.
	fakeShow := func(_ context.Context, iface string) (wireguard.Dump, error) {
		switch iface {
		case "wg_sw":
			return wireguard.Dump{
				Fwmark: "0x20000",
				Peers: []wireguard.Peer{{
					PublicKey:           "PUB_SW",
					Endpoint:            "1.2.3.4:51820",
					LatestHandshakeUnix: 1712845000,
					RXBytes:             1000,
					TXBytes:             2000,
				}},
			}, nil
		case "wg_us":
			return wireguard.Dump{
				Fwmark: "0x30000",
				Peers: []wireguard.Peer{{
					PublicKey:           "PUB_US",
					Endpoint:            "5.6.7.8:51821",
					LatestHandshakeUnix: 0, // never handshaked
					RXBytes:             3000,
					TXBytes:             4000,
				}},
			}, nil
		default:
			return wireguard.Dump{}, nil
		}
	}

	// Write pool-health fixture.
	phDir := t.TempDir()
	phPath := filepath.Join(phDir, "state.json")
	phData := `{
  "updated_at": "2026-04-11T12:00:00Z",
  "tunnels": {
    "wg_sw": { "healthy": true, "consecutive_failures": 0 },
    "wg_us": { "healthy": false, "consecutive_failures": 2 }
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
	}

	st := state.New()

	c := NewTunnels(TunnelsOpts{
		Topology:       topo,
		PoolHealthPath: phPath,
		State:          st,
		Show:           fakeShow,
	})

	if c.Name() != "tunnels" {
		t.Errorf("Name() = %q, want %q", c.Name(), "tunnels")
	}
	if c.Tier() != Hot {
		t.Errorf("Tier() = %v, want Hot", c.Tier())
	}

	if err := c.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	tunnels, updated := st.SnapshotTunnels()
	if updated.IsZero() {
		t.Fatal("expected non-zero updated time")
	}
	if len(tunnels) != 2 {
		t.Fatalf("expected 2 tunnels, got %d", len(tunnels))
	}

	// Verify wg_sw.
	sw := tunnels[0]
	if sw.Name != "wg_sw" {
		t.Errorf("tunnels[0].Name = %q, want %q", sw.Name, "wg_sw")
	}
	if sw.Fwmark != "0x20000" {
		t.Errorf("tunnels[0].Fwmark = %q, want %q", sw.Fwmark, "0x20000")
	}
	if sw.RoutingTable != 200 {
		t.Errorf("tunnels[0].RoutingTable = %d, want 200", sw.RoutingTable)
	}
	if sw.PublicKey != "PUB_SW" {
		t.Errorf("tunnels[0].PublicKey = %q, want %q", sw.PublicKey, "PUB_SW")
	}
	if sw.Endpoint != "1.2.3.4:51820" {
		t.Errorf("tunnels[0].Endpoint = %q, want %q", sw.Endpoint, "1.2.3.4:51820")
	}
	if sw.RXBytes != 1000 {
		t.Errorf("tunnels[0].RXBytes = %d, want 1000", sw.RXBytes)
	}
	if sw.TXBytes != 2000 {
		t.Errorf("tunnels[0].TXBytes = %d, want 2000", sw.TXBytes)
	}
	if !sw.Healthy {
		t.Error("tunnels[0] should be healthy")
	}
	if sw.ConsecutiveFailures != 0 {
		t.Errorf("tunnels[0].ConsecutiveFailures = %d, want 0", sw.ConsecutiveFailures)
	}
	if sw.LatestHandshakeSecondsAgo <= 0 {
		t.Errorf("tunnels[0].LatestHandshakeSecondsAgo = %d, want > 0", sw.LatestHandshakeSecondsAgo)
	}

	// Verify wg_us.
	us := tunnels[1]
	if us.Name != "wg_us" {
		t.Errorf("tunnels[1].Name = %q, want %q", us.Name, "wg_us")
	}
	if us.Fwmark != "0x30000" {
		t.Errorf("tunnels[1].Fwmark = %q, want %q", us.Fwmark, "0x30000")
	}
	if us.PublicKey != "PUB_US" {
		t.Errorf("tunnels[1].PublicKey = %q, want %q", us.PublicKey, "PUB_US")
	}
	if us.Healthy {
		t.Error("tunnels[1] should not be healthy")
	}
	if us.ConsecutiveFailures != 2 {
		t.Errorf("tunnels[1].ConsecutiveFailures = %d, want 2", us.ConsecutiveFailures)
	}
	if us.LatestHandshakeSecondsAgo != -1 {
		t.Errorf("tunnels[1].LatestHandshakeSecondsAgo = %d, want -1 (never handshaked)", us.LatestHandshakeSecondsAgo)
	}
}

func TestTunnelsCollectorMissingPoolHealth(t *testing.T) {
	fakeShow := func(_ context.Context, _ string) (wireguard.Dump, error) {
		return wireguard.Dump{
			Peers: []wireguard.Peer{{
				PublicKey:           "PUB_A",
				Endpoint:            "9.8.7.6:51820",
				LatestHandshakeUnix: 1712845000,
				RXBytes:             500,
				TXBytes:             600,
			}},
		}, nil
	}

	topo := &topology.Topology{
		Tunnels: []topology.Tunnel{
			{Name: "wg_sw", Interface: "wg_sw", Fwmark: "0x20000", RoutingTable: 200},
		},
	}

	st := state.New()

	c := NewTunnels(TunnelsOpts{
		Topology:       topo,
		PoolHealthPath: "/nonexistent/state.json",
		State:          st,
		Show:           fakeShow,
	})

	if err := c.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	tunnels, _ := st.SnapshotTunnels()
	if len(tunnels) != 1 {
		t.Fatalf("expected 1 tunnel, got %d", len(tunnels))
	}

	// With missing pool-health, tunnel should default to healthy.
	if !tunnels[0].Healthy {
		t.Error("tunnel should default to healthy when pool-health is missing")
	}
	if tunnels[0].ConsecutiveFailures != 0 {
		t.Errorf("ConsecutiveFailures = %d, want 0", tunnels[0].ConsecutiveFailures)
	}
}
