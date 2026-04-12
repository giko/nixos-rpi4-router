package collector

import (
	"context"
	"fmt"
	"testing"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/state"
)

func TestClientConnsCollector(t *testing.T) {
	fakeRun := func(_ context.Context, _ ...string) (string, error) {
		return "ipv4     2 tcp      6 60 src=192.168.1.10 dst=1.1.1.1 sport=5 dport=443 mark=131072\n", nil
	}

	st := state.New()
	c := NewClientConns(ClientConnsOpts{Run: fakeRun, State: st})

	if c.Name() != "client-conns" {
		t.Errorf("Name() = %q, want %q", c.Name(), "client-conns")
	}
	if c.Tier() != Cold {
		t.Errorf("Tier() = %v, want Cold", c.Tier())
	}

	if err := c.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	got, ts := st.SnapshotClientConns()
	if ts.IsZero() {
		t.Fatal("expected non-zero updated time")
	}
	// 131072 = 0x20000
	info := got["192.168.1.10"]
	if info.TotalConns != 1 {
		t.Errorf("TotalConns = %d, want 1", info.TotalConns)
	}
	if info.TunnelConns["0x20000"] != 1 {
		t.Errorf("TunnelConns[0x20000] = %d, want 1", info.TunnelConns["0x20000"])
	}
}

func TestClientConnsCollectorRunnerError(t *testing.T) {
	fakeRun := func(_ context.Context, _ ...string) (string, error) {
		return "", fmt.Errorf("permission denied")
	}

	st := state.New()
	c := NewClientConns(ClientConnsOpts{Run: fakeRun, State: st})

	if err := c.Run(context.Background()); err == nil {
		t.Fatal("expected error from runner failure")
	}

	// State should not have been updated.
	_, ts := st.SnapshotClientConns()
	if !ts.IsZero() {
		t.Error("state should not be updated on error")
	}
}
