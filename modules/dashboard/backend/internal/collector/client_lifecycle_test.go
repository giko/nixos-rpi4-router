package collector

import (
	"net/netip"
	"testing"
	"time"
)

func TestLifecycleStatusTransitions(t *testing.T) {
	ip := netip.MustParseAddr("192.168.1.42")
	clock := time.Date(2026, 4, 13, 12, 0, 0, 0, time.UTC)
	l := NewLifecycleTracker(15 * time.Minute)

	if got := l.Status(ip); got != LeaseStatusUnknown {
		t.Errorf("initial status = %v, want unknown", got)
	}

	l.OnScan([]netip.Addr{ip}, clock)
	if got := l.Status(ip); got != LeaseStatusDynamic {
		t.Errorf("after lease appears: %v, want dynamic", got)
	}

	clock = clock.Add(1 * time.Minute)
	l.OnScan([]netip.Addr{}, clock)
	if got := l.Status(ip); got != LeaseStatusExpired {
		t.Errorf("after lease disappears: %v, want expired", got)
	}

	clock = clock.Add(1 * time.Second)
	l.OnScan([]netip.Addr{ip}, clock)
	if got := l.Status(ip); got != LeaseStatusDynamic {
		t.Errorf("after lease re-appears: %v, want dynamic", got)
	}

	clock = clock.Add(1 * time.Minute)
	l.OnScan([]netip.Addr{}, clock)
	clock = clock.Add(16 * time.Minute)
	reaped := l.Reap(clock)
	if len(reaped) != 1 || reaped[0] != ip {
		t.Errorf("expected ip reaped after 16min tombstone, got %v", reaped)
	}
	if got := l.Status(ip); got != LeaseStatusUnknown {
		t.Errorf("after reap: %v, want unknown", got)
	}
}

func TestLifecycleSameIPReclaimedDuringTombstone(t *testing.T) {
	ip := netip.MustParseAddr("192.168.1.42")
	clock := time.Date(2026, 4, 13, 12, 0, 0, 0, time.UTC)
	l := NewLifecycleTracker(15 * time.Minute)

	l.OnScan([]netip.Addr{ip}, clock)
	clock = clock.Add(2 * time.Minute)
	l.OnScan([]netip.Addr{}, clock)
	clock = clock.Add(3 * time.Minute)

	// Re-lease during tombstone window — caller must treat this as a fresh client.
	clock = clock.Add(1 * time.Second)
	events := l.OnScan([]netip.Addr{ip}, clock)
	if len(events.Rebirths) != 1 || events.Rebirths[0] != ip {
		t.Errorf("Rebirths = %v, want [%s]", events.Rebirths, ip)
	}
}
