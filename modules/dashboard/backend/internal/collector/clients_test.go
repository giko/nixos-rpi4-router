package collector

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/conntrack"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/state"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/topology"
)

func TestClientsCollectorMergesSources(t *testing.T) {
	// Write a dnsmasq leases fixture with two dynamic leases.
	dir := t.TempDir()
	leasesPath := filepath.Join(dir, "dnsmasq.leases")
	leasesData := "1712800000 aa:bb:cc:dd:ee:01 192.168.1.50 phone 01:aa:bb:cc:dd:ee:01\n" +
		"1712800000 aa:bb:cc:dd:ee:02 192.168.1.51 laptop *\n"
	if err := os.WriteFile(leasesPath, []byte(leasesData), 0644); err != nil {
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
		StaticLeases: []topology.StaticLease{
			{MAC: "AA:BB:CC:00:00:01", IP: "192.168.1.10", Name: "desktop"},
		},
		AllowlistEnabled: true,
		AllowedMACs:      []string{"AA:BB:CC:DD:EE:01"},
	}

	st := state.New()

	// Seed client connection info: desktop has connections through wg_sw.
	st.SetClientConns(map[string]conntrack.ClientConnInfo{
		"192.168.1.10": {TotalConns: 5, TunnelConns: map[string]int{"0x20000": 3}},
	})

	// Fake neighbour table: 3 IPs, one overlaps with static (should be skipped).
	fakeNeigh := func(_ context.Context) (map[string]string, error) {
		return map[string]string{
			"192.168.1.10": "aa:bb:cc:00:00:01", // overlap with static
			"192.168.1.50": "aa:bb:cc:dd:ee:01", // overlap with dynamic
			"192.168.1.99": "ff:ff:ff:00:00:01", // new neighbor
		}, nil
	}

	c := NewClients(ClientsOpts{
		Topology:   topo,
		LeasesPath: leasesPath,
		State:      st,
		Neigh:      fakeNeigh,
	})

	if c.Name() != "clients" {
		t.Errorf("Name() = %q, want %q", c.Name(), "clients")
	}
	if c.Tier() != Medium {
		t.Errorf("Tier() = %v, want Medium", c.Tier())
	}

	if err := c.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	clients, updated := st.SnapshotClients()
	if updated.IsZero() {
		t.Fatal("expected non-zero updated time")
	}

	// Should have 4 clients: 1 static + 2 dynamic + 1 neighbor.
	if len(clients) != 4 {
		t.Fatalf("expected 4 clients, got %d", len(clients))
	}

	// Clients should be sorted by IP.
	for i := 1; i < len(clients); i++ {
		if compareIPs(clients[i-1].IP, clients[i].IP) > 0 {
			t.Errorf("clients not sorted: %s > %s", clients[i-1].IP, clients[i].IP)
		}
	}

	// Find and verify each client by IP.
	byIP := make(map[string]int, len(clients))
	for i, cl := range clients {
		byIP[cl.IP] = i
	}

	// Static lease: desktop.
	idx, ok := byIP["192.168.1.10"]
	if !ok {
		t.Fatal("missing static lease 192.168.1.10")
	}
	cl := clients[idx]
	if cl.LeaseType != "static" {
		t.Errorf("static lease: LeaseType = %q, want static", cl.LeaseType)
	}
	if cl.Hostname != "desktop" {
		t.Errorf("static lease: Hostname = %q, want desktop", cl.Hostname)
	}
	if cl.MAC != "aa:bb:cc:00:00:01" {
		t.Errorf("static lease: MAC = %q, want aa:bb:cc:00:00:01", cl.MAC)
	}
	// Static lease MAC is implicitly in allowed set.
	if cl.AllowlistStatus != "allowed" {
		t.Errorf("static lease: AllowlistStatus = %q, want allowed", cl.AllowlistStatus)
	}
	if cl.Route != "pool:vpn_pool" {
		t.Errorf("static lease: Route = %q, want pool:vpn_pool", cl.Route)
	}
	if cl.ConnCount != 5 {
		t.Errorf("static lease: ConnCount = %d, want 5", cl.ConnCount)
	}

	// Dynamic lease: phone (explicitly in AllowedMACs).
	idx, ok = byIP["192.168.1.50"]
	if !ok {
		t.Fatal("missing dynamic lease 192.168.1.50")
	}
	cl = clients[idx]
	if cl.LeaseType != "dynamic" {
		t.Errorf("phone: LeaseType = %q, want dynamic", cl.LeaseType)
	}
	if cl.Hostname != "phone" {
		t.Errorf("phone: Hostname = %q, want phone", cl.Hostname)
	}
	if cl.AllowlistStatus != "allowed" {
		t.Errorf("phone: AllowlistStatus = %q, want allowed", cl.AllowlistStatus)
	}
	if cl.Route != "wan" {
		t.Errorf("phone: Route = %q, want wan", cl.Route)
	}

	// Dynamic lease: laptop (NOT in AllowedMACs).
	idx, ok = byIP["192.168.1.51"]
	if !ok {
		t.Fatal("missing dynamic lease 192.168.1.51")
	}
	cl = clients[idx]
	if cl.LeaseType != "dynamic" {
		t.Errorf("laptop: LeaseType = %q, want dynamic", cl.LeaseType)
	}
	if cl.AllowlistStatus != "blocked" {
		t.Errorf("laptop: AllowlistStatus = %q, want blocked", cl.AllowlistStatus)
	}

	// Neighbor-only device.
	idx, ok = byIP["192.168.1.99"]
	if !ok {
		t.Fatal("missing neighbor 192.168.1.99")
	}
	cl = clients[idx]
	if cl.LeaseType != "neighbor" {
		t.Errorf("neighbor: LeaseType = %q, want neighbor", cl.LeaseType)
	}
	if cl.AllowlistStatus != "blocked" {
		t.Errorf("neighbor: AllowlistStatus = %q, want blocked", cl.AllowlistStatus)
	}
	if cl.Route != "wan" {
		t.Errorf("neighbor: Route = %q, want wan", cl.Route)
	}
}

func TestClientsCollectorAllowlistDisabled(t *testing.T) {
	dir := t.TempDir()
	leasesPath := filepath.Join(dir, "dnsmasq.leases")
	leasesData := "1712800000 aa:bb:cc:dd:ee:01 192.168.1.50 phone *\n"
	if err := os.WriteFile(leasesPath, []byte(leasesData), 0644); err != nil {
		t.Fatal(err)
	}

	topo := &topology.Topology{
		StaticLeases: []topology.StaticLease{
			{MAC: "AA:BB:CC:00:00:01", IP: "192.168.1.10", Name: "desktop"},
		},
		AllowlistEnabled: false,
	}

	st := state.New()

	fakeNeigh := func(_ context.Context) (map[string]string, error) {
		return map[string]string{
			"192.168.1.99": "ff:ff:ff:00:00:01",
		}, nil
	}

	c := NewClients(ClientsOpts{
		Topology:   topo,
		LeasesPath: leasesPath,
		State:      st,
		Neigh:      fakeNeigh,
	})

	if err := c.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	clients, _ := st.SnapshotClients()
	if len(clients) != 3 {
		t.Fatalf("expected 3 clients, got %d", len(clients))
	}

	for _, cl := range clients {
		if cl.AllowlistStatus != "n/a" {
			t.Errorf("client %s: AllowlistStatus = %q, want n/a", cl.IP, cl.AllowlistStatus)
		}
	}
}

func TestClientsCollectorNoNeighFunc(t *testing.T) {
	dir := t.TempDir()
	leasesPath := filepath.Join(dir, "dnsmasq.leases")
	if err := os.WriteFile(leasesPath, nil, 0644); err != nil {
		t.Fatal(err)
	}

	topo := &topology.Topology{
		StaticLeases: []topology.StaticLease{
			{MAC: "AA:BB:CC:00:00:01", IP: "192.168.1.10", Name: "desktop"},
		},
		AllowlistEnabled: false,
	}

	st := state.New()

	// Neigh is nil -- should not panic.
	c := NewClients(ClientsOpts{
		Topology:   topo,
		LeasesPath: leasesPath,
		State:      st,
		Neigh:      nil,
	})

	if err := c.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	clients, _ := st.SnapshotClients()
	if len(clients) != 1 {
		t.Fatalf("expected 1 client, got %d", len(clients))
	}
	if clients[0].IP != "192.168.1.10" {
		t.Errorf("IP = %q, want 192.168.1.10", clients[0].IP)
	}
}
