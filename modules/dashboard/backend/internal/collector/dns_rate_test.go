package collector

import (
	"net/netip"
	"testing"
	"time"
)

func TestDnsRateCountsObserved(t *testing.T) {
	a := netip.MustParseAddr("192.168.1.42")
	b := netip.MustParseAddr("192.168.1.50")
	d := NewDnsRate(DnsRateOpts{TickDur: 10 * time.Second})
	d.Track(a)
	d.Track(b)
	for i := 0; i < 42; i++ {
		d.Observe(a)
	}
	for i := 0; i < 7; i++ {
		d.Observe(b)
	}
	d.Tick(time.Unix(0, 0))
	sA, _ := d.Snapshot(a)
	if len(sA) != 1 || sA[0].QueriesPerWindow != 42 {
		t.Fatalf("a samples = %+v, want 1 sample with QueriesPerWindow=42", sA)
	}
	sB, _ := d.Snapshot(b)
	if len(sB) != 1 || sB[0].QueriesPerWindow != 7 {
		t.Fatalf("b samples = %+v, want 1 sample with QueriesPerWindow=7", sB)
	}
}

func TestDnsRateResetsCountersAfterTick(t *testing.T) {
	a := netip.MustParseAddr("192.168.1.42")
	d := NewDnsRate(DnsRateOpts{TickDur: 10 * time.Second})
	d.Track(a)
	for i := 0; i < 10; i++ {
		d.Observe(a)
	}
	d.Tick(time.Unix(0, 0))
	d.Tick(time.Unix(10, 0)) // second tick with no observations → 0
	s, _ := d.Snapshot(a)
	if len(s) != 2 {
		t.Fatalf("want 2 samples, got %d", len(s))
	}
	if s[0].QueriesPerWindow != 10 || s[1].QueriesPerWindow != 0 {
		t.Errorf("samples = %+v, want [10, 0]", s)
	}
}

func TestDnsRateIgnoresUntrackedClients(t *testing.T) {
	a := netip.MustParseAddr("192.168.1.42")
	d := NewDnsRate(DnsRateOpts{TickDur: 10 * time.Second})
	d.Observe(a) // not tracked — should be a no-op
	d.Track(a)
	d.Tick(time.Unix(0, 0))
	s, _ := d.Snapshot(a)
	if len(s) != 1 || s[0].QueriesPerWindow != 0 {
		t.Errorf("samples = %+v, want 1 sample with 0", s)
	}
}
