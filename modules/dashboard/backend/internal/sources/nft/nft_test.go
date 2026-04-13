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
	if len(r.Counters) != 3 {
		t.Errorf("Counters count = %d, want 3 (input/handle 11, forward/handle 20, forward/handle 21)", len(r.Counters))
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

func TestParseRuleWithMultipleCounters(t *testing.T) {
	// nft permits multiple counter expressions in one rule (e.g. one
	// before and one after a jump). Verify the parser emits a Counter
	// for each.
	raw := []byte(`{"nftables":[
		{"chain":{"family":"inet","table":"filter","name":"input","handle":1,"type":"filter","hook":"input","prio":0,"policy":"drop"}},
		{"rule":{"family":"inet","table":"filter","chain":"input","handle":50,"comment":"two counters","expr":[
			{"counter":{"packets":1,"bytes":100}},
			{"jump":{"target":"sub"}},
			{"counter":{"packets":2,"bytes":200}}
		]}}
	]}`)
	r, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(r.Counters) != 2 {
		t.Fatalf("Counters count = %d, want 2: %+v", len(r.Counters), r.Counters)
	}
	if r.Counters[0].Bytes != 100 || r.Counters[1].Bytes != 200 {
		t.Errorf("counter byte values = %d, %d; want 100, 200", r.Counters[0].Bytes, r.Counters[1].Bytes)
	}
}

func TestParseUPnPPolicyRuleSkipped(t *testing.T) {
	// A rule in the inet/miniupnpd table that lacks a `dnat` target
	// (e.g. a policy/return rule) must NOT be emitted as a mapping.
	raw := []byte(`{"nftables":[
		{"chain":{"family":"inet","table":"miniupnpd","name":"prerouting_miniupnpd","handle":1,"type":"nat","hook":"prerouting","prio":-100,"policy":"accept"}},
		{"rule":{"family":"inet","table":"miniupnpd","chain":"prerouting_miniupnpd","handle":99,"comment":"return","expr":[
			{"return":null}
		]}}
	]}`)
	r, err := Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(r.UPnPMappings) != 0 {
		t.Fatalf("UPnPMappings count = %d, want 0 (policy rule should not produce a mapping): %+v", len(r.UPnPMappings), r.UPnPMappings)
	}
}
