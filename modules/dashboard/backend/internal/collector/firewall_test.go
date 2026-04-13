package collector

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/model"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/state"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/topology"
)

func newTestTopology() *topology.Topology {
	return &topology.Topology{
		AllowedMACs: []string{"aa:bb:cc:dd:ee:ff"},
		PortForwards: []topology.PortForward{
			{Protocol: "tcp", ExternalPort: 35978, Destination: "192.168.20.6:32400"},
		},
		PBRSourceRules: []topology.PBRSourceRule{
			{Sources: []string{"192.168.1.225"}, Tunnel: "wg_sw"},
		},
		PBRDomainRules: []topology.PBRDomainRule{
			{Tunnel: "wg_sw", Domains: []string{"example.com"}},
		},
		PooledRules: []topology.PooledRule{
			{Sources: []string{"192.168.1.10"}, Pool: "all"},
		},
	}
}

func TestFirewallCollectorPopulatesState(t *testing.T) {
	st := state.New()
	stub := func(_ context.Context, _ ...string) ([]byte, error) {
		return []byte(`{"nftables":[
			{"chain":{"family":"inet","table":"filter","name":"input","handle":1,"type":"filter","hook":"input","prio":0,"policy":"drop"}},
			{"chain":{"family":"inet","table":"filter","name":"forward","handle":2,"type":"filter","hook":"forward","prio":0,"policy":"drop"}},
			{"rule":{"family":"inet","table":"filter","chain":"input","handle":16,"expr":[{"counter":{"packets":42,"bytes":1024}},{"drop":null}]}},
			{"rule":{"family":"inet","table":"filter","chain":"forward","handle":20,"expr":[{"counter":{"packets":100,"bytes":4096}},{"drop":null}]}}
		]}`), nil
	}
	c := NewFirewall(FirewallOpts{State: st, Topology: newTestTopology(), Run: stub})
	if err := c.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	got, ts := st.SnapshotFirewall()
	if ts.IsZero() {
		t.Error("ts zero")
	}
	if len(got.PortForwards) != 1 || got.PortForwards[0].ExternalPort != 35978 {
		t.Errorf("PortForwards = %+v", got.PortForwards)
	}
	if len(got.PBR.SourceRules) != 1 || got.PBR.SourceRules[0].Tunnel != "wg_sw" {
		t.Errorf("PBR.SourceRules = %+v", got.PBR.SourceRules)
	}
	if len(got.PBR.PooledRules) != 1 || got.PBR.PooledRules[0].Pool != "all" {
		t.Errorf("PBR.PooledRules = %+v", got.PBR.PooledRules)
	}
	if len(got.AllowedMACs) != 1 || got.AllowedMACs[0] != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("AllowedMACs = %+v", got.AllowedMACs)
	}
	if len(got.Chains) != 2 {
		t.Errorf("Chains = %+v", got.Chains)
	}
	var forward model.FirewallChain
	for _, ch := range got.Chains {
		if ch.Name == "forward" {
			forward = ch
		}
	}
	if len(forward.Counters) != 1 || forward.Counters[0].Bytes != 4096 {
		t.Errorf("forward chain counters = %+v", forward.Counters)
	}
	if c.Tier() != Medium {
		t.Errorf("Tier = %v, want Medium", c.Tier())
	}
	if c.Name() != "firewall" {
		t.Errorf("Name = %q, want firewall", c.Name())
	}
}

func TestFirewallCollectorBlockedForwardCount1h(t *testing.T) {
	// Inject a fake clock and nft stub whose forward-drop counter
	// climbs over several ticks; verify the 1h delta equals
	// current - oldest-sample-at-or-before-1h-ago.
	st := state.New()
	var counter int64
	stub := func(_ context.Context, _ ...string) ([]byte, error) {
		return []byte(fmt.Sprintf(`{"nftables":[
			{"chain":{"family":"inet","table":"filter","name":"forward","handle":1,"type":"filter","hook":"forward","prio":0,"policy":"drop"}},
			{"rule":{"family":"inet","table":"filter","chain":"forward","handle":2,"expr":[{"counter":{"packets":%d,"bytes":0}},{"drop":null}]}}
		]}`, counter)), nil
	}
	now := time.Unix(1_700_000_000, 0)
	clock := func() time.Time { return now }
	c := NewFirewall(FirewallOpts{State: st, Topology: &topology.Topology{}, Run: stub, Clock: clock})

	// t=0min:   total forward-drops = 10
	counter = 10
	if err := c.Run(context.Background()); err != nil {
		t.Fatalf("run1: %v", err)
	}
	// t=65min:  total forward-drops = 50 → 1h delta should be 50 - 10 = 40
	now = now.Add(65 * time.Minute)
	counter = 50
	if err := c.Run(context.Background()); err != nil {
		t.Fatalf("run2: %v", err)
	}
	got, _ := st.SnapshotFirewall()
	if got.BlockedForwardCount1h != 40 {
		t.Errorf("BlockedForwardCount1h = %d, want 40", got.BlockedForwardCount1h)
	}
}

func TestFirewallCollectorIgnoresAcceptCountersInForward(t *testing.T) {
	st := state.New()
	stub := func(_ context.Context, _ ...string) ([]byte, error) {
		return []byte(`{"nftables":[
			{"chain":{"family":"inet","table":"filter","name":"forward","handle":1,"type":"filter","hook":"forward","prio":0,"policy":"drop"}},
			{"rule":{"family":"inet","table":"filter","chain":"forward","handle":10,"expr":[{"counter":{"packets":99,"bytes":0}},{"accept":null}]}},
			{"rule":{"family":"inet","table":"filter","chain":"forward","handle":11,"expr":[{"counter":{"packets":7,"bytes":0}},{"drop":null}]}}
		]}`), nil
	}
	now := time.Unix(1_700_000_000, 0)
	c := NewFirewall(FirewallOpts{
		State: st, Topology: &topology.Topology{}, Run: stub,
		Clock: func() time.Time { return now },
	})
	if err := c.Run(context.Background()); err != nil {
		t.Fatalf("run1: %v", err)
	}
	now = now.Add(2 * time.Hour) // ensure 1h-warmup guard is satisfied
	if err := c.Run(context.Background()); err != nil {
		t.Fatalf("run2: %v", err)
	}
	got, _ := st.SnapshotFirewall()
	// Only the drop rule (handle 11, 7 packets) should count.
	// Both ticks see the same total (7), so the delta is 0.
	if got.BlockedForwardCount1h != 0 {
		t.Errorf("BlockedForwardCount1h = %d, want 0 (drop counter unchanged; accept counter must not count)", got.BlockedForwardCount1h)
	}
}

func TestFirewallCollectorWithholdsBlockedCountUntil1hOfHistory(t *testing.T) {
	st := state.New()
	var counter int64
	stub := func(_ context.Context, _ ...string) ([]byte, error) {
		return []byte(fmt.Sprintf(`{"nftables":[
			{"chain":{"family":"inet","table":"filter","name":"forward","handle":1,"type":"filter","hook":"forward","prio":0,"policy":"drop"}},
			{"rule":{"family":"inet","table":"filter","chain":"forward","handle":2,"expr":[{"counter":{"packets":%d,"bytes":0}},{"drop":null}]}}
		]}`, counter)), nil
	}
	now := time.Unix(1_700_000_000, 0)
	clock := func() time.Time { return now }
	c := NewFirewall(FirewallOpts{State: st, Topology: &topology.Topology{}, Run: stub, Clock: clock})

	// Sample 1 at t=0: count=10
	counter = 10
	if err := c.Run(context.Background()); err != nil {
		t.Fatalf("run1: %v", err)
	}
	// Sample 2 at t=30min: count=30 (only 30min of history → still 0)
	now = now.Add(30 * time.Minute)
	counter = 30
	if err := c.Run(context.Background()); err != nil {
		t.Fatalf("run2: %v", err)
	}
	got, _ := st.SnapshotFirewall()
	if got.BlockedForwardCount1h != 0 {
		t.Errorf("BlockedForwardCount1h = %d, want 0 (only 30min of history)", got.BlockedForwardCount1h)
	}
}
