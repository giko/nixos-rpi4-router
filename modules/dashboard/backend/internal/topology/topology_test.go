package topology

import (
	"os"
	"path/filepath"
	"testing"
)

const fixture = `{
  "tunnels": [
    {"name":"wg_sw","interface":"wg_sw","fwmark":"0x20000","routing_table":131072},
    {"name":"wg_us","interface":"wg_us","fwmark":"0x30000","routing_table":196608}
  ],
  "pools": [
    {"name":"all_vpns","members":["wg_sw","wg_us"]}
  ],
  "pooled_rules": [
    {"sources":["192.168.1.125"],"pool":"all_vpns"}
  ],
  "static_leases": [
    {"mac":"aa:bb:cc:dd:ee:ff","ip":"192.168.1.10","name":"XBOX"}
  ],
  "allowlist_enabled": true,
  "allowed_macs": ["aa:bb:cc:dd:ee:ff"],
  "lan_interface": "eth0"
}`

func TestLoadValid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "topology.json")
	if err := os.WriteFile(path, []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}
	topo, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(topo.Tunnels) != 2 {
		t.Errorf("tunnels = %d, want 2", len(topo.Tunnels))
	}
	if topo.Tunnels[0].Fwmark != "0x20000" {
		t.Errorf("fwmark = %q", topo.Tunnels[0].Fwmark)
	}
	if len(topo.Pools) != 1 || topo.Pools[0].Name != "all_vpns" {
		t.Errorf("unexpected pools: %+v", topo.Pools)
	}
	if len(topo.PooledRules) != 1 || topo.PooledRules[0].Pool != "all_vpns" {
		t.Errorf("unexpected pooled rules: %+v", topo.PooledRules)
	}
	if len(topo.StaticLeases) != 1 || topo.StaticLeases[0].Name != "XBOX" {
		t.Errorf("unexpected leases: %+v", topo.StaticLeases)
	}
	if topo.LANInterface != "eth0" {
		t.Errorf("lan = %q", topo.LANInterface)
	}
	if !topo.AllowlistEnabled {
		t.Errorf("allowlist_enabled = %v, want true", topo.AllowlistEnabled)
	}
}

func TestLoadAllowlistDisabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "topology.json")
	body := `{"allowlist_enabled": false, "allowed_macs": []}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	topo, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if topo.AllowlistEnabled {
		t.Errorf("AllowlistEnabled = true, want false")
	}
}

func TestLoadEmptyPathReturnsEmpty(t *testing.T) {
	topo, err := Load("")
	if err != nil {
		t.Fatalf("Load(\"\") error: %v", err)
	}
	if len(topo.Tunnels) != 0 || len(topo.Pools) != 0 {
		t.Errorf("expected empty topology, got %+v", topo)
	}
}

func TestLoadMissingFileReturnsEmpty(t *testing.T) {
	topo, err := Load("/nonexistent/dashboard-config.json")
	if err != nil {
		t.Fatalf("Load missing: %v", err)
	}
	if len(topo.Tunnels) != 0 {
		t.Error("expected empty topology from missing file")
	}
}

func TestLoadInvalidJSONIsHardError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error from invalid JSON")
	}
}

func TestFwmarkLookup(t *testing.T) {
	topo := &Topology{Tunnels: []Tunnel{
		{Name: "wg_sw", Fwmark: "0x20000"},
		{Name: "wg_us", Fwmark: "0x30000"},
	}}
	if got := topo.TunnelByName("wg_sw"); got == nil || got.Fwmark != "0x20000" {
		t.Errorf("TunnelByName(wg_sw) = %+v", got)
	}
	if got := topo.TunnelByName("wg_nope"); got != nil {
		t.Errorf("TunnelByName(wg_nope) = %+v, want nil", got)
	}
}
