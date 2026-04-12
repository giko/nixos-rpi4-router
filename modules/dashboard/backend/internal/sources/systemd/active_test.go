package systemd

import (
	"testing"
)

func TestParseIsActiveOutput(t *testing.T) {
	raw := "active\nfailed\ninactive\nactivating\n"
	names := []string{"nftables", "adguardhome", "wireguard-wg_sw", "dnsmasq"}
	got := parseIsActive(names, raw)

	if len(got) != 4 {
		t.Fatalf("len = %d, want 4", len(got))
	}

	want := []struct {
		name     string
		active   bool
		rawState string
	}{
		{"nftables", true, "active"},
		{"adguardhome", false, "failed"},
		{"wireguard-wg_sw", false, "inactive"},
		{"dnsmasq", false, "activating"},
	}

	for i, w := range want {
		if got[i].Name != w.name {
			t.Errorf("[%d] Name = %q, want %q", i, got[i].Name, w.name)
		}
		if got[i].Active != w.active {
			t.Errorf("[%d] Active = %v, want %v", i, got[i].Active, w.active)
		}
		if got[i].RawState != w.rawState {
			t.Errorf("[%d] RawState = %q, want %q", i, got[i].RawState, w.rawState)
		}
	}
}

func TestParseIsActiveFewerLines(t *testing.T) {
	// If stdout has fewer lines than units, remaining units get empty rawState.
	raw := "active\n"
	names := []string{"nftables", "adguardhome"}
	got := parseIsActive(names, raw)

	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	if got[0].RawState != "active" {
		t.Errorf("[0] RawState = %q, want %q", got[0].RawState, "active")
	}
	if got[1].RawState != "" {
		t.Errorf("[1] RawState = %q, want %q", got[1].RawState, "")
	}
	if got[1].Active {
		t.Error("[1] Active should be false for empty state")
	}
}
