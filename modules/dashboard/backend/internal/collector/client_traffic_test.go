package collector

import (
	"net/netip"
	"testing"
	"time"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/conntrack"
)

func TestTrafficFirstTickSeedsThenDeltas(t *testing.T) {
	client := netip.MustParseAddr("192.168.1.42")
	key := conntrack.FlowKey{Proto: 6, OrigSrcIP: client,
		OrigDstIP: netip.MustParseAddr("1.2.3.4"), OrigSrcPort: 47182, OrigDstPort: 443}
	c := NewClientTraffic(ClientTrafficOpts{TickDur: 10 * time.Second})
	c.Track(client)
	c.Apply(time.Unix(0, 0), []conntrack.FlowBytes{{Key: key, ClientIP: client,
		Direction: conntrack.DirOutbound, OrigBytes: 1_000_000, ReplyBytes: 10_000_000}})
	samples, _, ok := c.Snapshot(client)
	if !ok {
		t.Fatal("client should be tracked")
	}
	if len(samples) != 0 {
		t.Fatalf("first tick should emit no sample, got %d", len(samples))
	}
	c.Apply(time.Unix(10, 0), []conntrack.FlowBytes{{Key: key, ClientIP: client,
		Direction: conntrack.DirOutbound, OrigBytes: 1_100_000, ReplyBytes: 12_000_000}})
	samples, _, _ = c.Snapshot(client)
	if len(samples) != 1 {
		t.Fatalf("expected 1 sample, got %d", len(samples))
	}
	if samples[0].TxBps != 80_000 {
		t.Errorf("tx_bps = %d, want 80000", samples[0].TxBps)
	}
	if samples[0].RxBps != 1_600_000 {
		t.Errorf("rx_bps = %d, want 1600000", samples[0].RxBps)
	}
}

func TestTrafficInterleavedFlowChurn(t *testing.T) {
	client := netip.MustParseAddr("192.168.1.42")
	keyA := conntrack.FlowKey{Proto: 6, OrigSrcIP: client,
		OrigDstIP: netip.MustParseAddr("1.2.3.4"), OrigSrcPort: 100, OrigDstPort: 443}
	keyB := conntrack.FlowKey{Proto: 6, OrigSrcIP: client,
		OrigDstIP: netip.MustParseAddr("5.6.7.8"), OrigSrcPort: 200, OrigDstPort: 443}
	c := NewClientTraffic(ClientTrafficOpts{TickDur: 10 * time.Second})
	c.Track(client)
	c.Apply(time.Unix(0, 0), []conntrack.FlowBytes{
		{Key: keyA, ClientIP: client, Direction: conntrack.DirOutbound, OrigBytes: 50_000_000}})
	c.Apply(time.Unix(10, 0), []conntrack.FlowBytes{
		{Key: keyB, ClientIP: client, Direction: conntrack.DirOutbound, OrigBytes: 200_000_000}})
	samples, _, _ := c.Snapshot(client)
	if len(samples) != 1 {
		t.Fatalf("samples = %d, want 1", len(samples))
	}
	if samples[0].TxBps != 0 {
		t.Errorf("tx = %d, want 0 (flow churn must not double-count)", samples[0].TxBps)
	}
}

func TestTrafficInboundAttributesRxCorrectly(t *testing.T) {
	client := netip.MustParseAddr("192.168.1.50")
	key := conntrack.FlowKey{Proto: 6,
		OrigSrcIP: netip.MustParseAddr("203.0.113.9"),
		OrigDstIP: netip.MustParseAddr("198.51.100.4"),
		OrigSrcPort: 55102, OrigDstPort: 32400}
	c := NewClientTraffic(ClientTrafficOpts{TickDur: 10 * time.Second})
	c.Track(client)
	c.Apply(time.Unix(0, 0), []conntrack.FlowBytes{{Key: key, ClientIP: client,
		Direction: conntrack.DirInbound, OrigBytes: 1_000_000, ReplyBytes: 100_000}})
	c.Apply(time.Unix(10, 0), []conntrack.FlowBytes{{Key: key, ClientIP: client,
		Direction: conntrack.DirInbound, OrigBytes: 4_000_000, ReplyBytes: 300_000}})
	samples, _, _ := c.Snapshot(client)
	if len(samples) != 1 {
		t.Fatalf("want 1 sample")
	}
	if samples[0].RxBps != 2_400_000 {
		t.Errorf("rx = %d, want 2400000 (3MB orig is client RX on inbound)", samples[0].RxBps)
	}
	if samples[0].TxBps != 160_000 {
		t.Errorf("tx = %d, want 160000 (200KB reply is client TX on inbound)", samples[0].TxBps)
	}
}

func TestTrafficDropThenReTrackStartsFresh(t *testing.T) {
	client := netip.MustParseAddr("192.168.1.42")
	key := conntrack.FlowKey{Proto: 6, OrigSrcIP: client,
		OrigDstIP: netip.MustParseAddr("1.2.3.4"), OrigSrcPort: 1, OrigDstPort: 443}
	c := NewClientTraffic(ClientTrafficOpts{TickDur: 10 * time.Second})
	c.Track(client)
	c.Apply(time.Unix(0, 0), []conntrack.FlowBytes{{Key: key, ClientIP: client,
		Direction: conntrack.DirOutbound, OrigBytes: 1_000_000, ReplyBytes: 0}})
	c.Apply(time.Unix(10, 0), []conntrack.FlowBytes{{Key: key, ClientIP: client,
		Direction: conntrack.DirOutbound, OrigBytes: 2_000_000, ReplyBytes: 0}})
	// Drop, same flow persists in conntrack with higher bytes
	c.Drop(client)
	c.Track(client)
	c.Apply(time.Unix(20, 0), []conntrack.FlowBytes{{Key: key, ClientIP: client,
		Direction: conntrack.DirOutbound, OrigBytes: 5_000_000, ReplyBytes: 0}})
	c.Apply(time.Unix(30, 0), []conntrack.FlowBytes{{Key: key, ClientIP: client,
		Direction: conntrack.DirOutbound, OrigBytes: 5_500_000, ReplyBytes: 0}})
	samples, _, _ := c.Snapshot(client)
	// The re-tracked client's first-post-drop tick is a seed (no sample).
	// The next tick shows delta = 500_000 bytes over 10s = 400_000 bps tx.
	if len(samples) != 1 {
		t.Fatalf("want 1 sample post-drop, got %d", len(samples))
	}
	if samples[0].TxBps != 400_000 {
		t.Errorf("tx = %d, want 400_000 (fresh start, 500KB delta)", samples[0].TxBps)
	}
}

func TestTrafficPerClientSeed(t *testing.T) {
	a := netip.MustParseAddr("192.168.1.10")
	b := netip.MustParseAddr("192.168.1.20")
	keyA := conntrack.FlowKey{Proto: 6, OrigSrcIP: a,
		OrigDstIP: netip.MustParseAddr("1.2.3.4"), OrigSrcPort: 1, OrigDstPort: 443}
	keyB := conntrack.FlowKey{Proto: 6, OrigSrcIP: b,
		OrigDstIP: netip.MustParseAddr("5.6.7.8"), OrigSrcPort: 2, OrigDstPort: 443}
	c := NewClientTraffic(ClientTrafficOpts{TickDur: 10 * time.Second})
	c.Track(a)
	// Tick 1 + Tick 2 for A → A has one sample, B not yet tracked.
	c.Apply(time.Unix(0, 0), []conntrack.FlowBytes{{Key: keyA, ClientIP: a,
		Direction: conntrack.DirOutbound, OrigBytes: 1_000_000}})
	c.Apply(time.Unix(10, 0), []conntrack.FlowBytes{{Key: keyA, ClientIP: a,
		Direction: conntrack.DirOutbound, OrigBytes: 2_000_000}})
	sA, _, _ := c.Snapshot(a)
	if len(sA) != 1 {
		t.Fatalf("A: want 1 sample, got %d", len(sA))
	}
	// Now add B. Its first Apply should be a seed-only for B, but A must still get a sample.
	c.Track(b)
	c.Apply(time.Unix(20, 0), []conntrack.FlowBytes{
		{Key: keyA, ClientIP: a, Direction: conntrack.DirOutbound, OrigBytes: 3_000_000},
		{Key: keyB, ClientIP: b, Direction: conntrack.DirOutbound, OrigBytes: 5_000_000},
	})
	sA2, _, _ := c.Snapshot(a)
	if len(sA2) != 2 {
		t.Errorf("A: want 2 samples after B joins, got %d", len(sA2))
	}
	sB, _, _ := c.Snapshot(b)
	if len(sB) != 0 {
		t.Errorf("B: want 0 samples (first tick is seed-only), got %d", len(sB))
	}
}
