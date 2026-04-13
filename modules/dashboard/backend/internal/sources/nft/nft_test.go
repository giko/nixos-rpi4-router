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
