package collector

import (
	"net/netip"
	"sync"
	"time"
)

type LeaseStatus string

const (
	LeaseStatusUnknown    LeaseStatus = "unknown"
	LeaseStatusDynamic    LeaseStatus = "dynamic"
	LeaseStatusStatic     LeaseStatus = "static"
	LeaseStatusNonDynamic LeaseStatus = "non-dynamic"
	LeaseStatusExpired    LeaseStatus = "expired"
)

// LifecycleEvents summarizes what changed in a single OnScan call.
type LifecycleEvents struct {
	Births   []netip.Addr // newly-leased (never seen, or reaped long ago)
	Rebirths []netip.Addr // same IP re-leased while tombstoned — callers must discard state
	Deaths   []netip.Addr // leases that disappeared (tombstone starts)
}

type lifecycleEntry struct {
	firstSeen     time.Time
	leasedNow     bool
	tombstoneFrom time.Time // zero unless currently tombstoned
}

type LifecycleTracker struct {
	mu       sync.RWMutex
	entries  map[netip.Addr]*lifecycleEntry
	graceDur time.Duration

	OnBirth   func(netip.Addr)
	OnDeath   func(netip.Addr)
	OnRebirth func(netip.Addr)
}

func NewLifecycleTracker(grace time.Duration) *LifecycleTracker {
	return &LifecycleTracker{
		entries:  make(map[netip.Addr]*lifecycleEntry),
		graceDur: grace,
	}
}

// OnScan updates tracker state from a fresh lease scan and fires any
// registered callbacks outside the internal lock. Returns the event set
// for inspection.
func (l *LifecycleTracker) OnScan(leased []netip.Addr, now time.Time) LifecycleEvents {
	l.mu.Lock()

	var ev LifecycleEvents
	seen := make(map[netip.Addr]struct{}, len(leased))
	for _, ip := range leased {
		seen[ip] = struct{}{}
		e, ok := l.entries[ip]
		switch {
		case !ok:
			l.entries[ip] = &lifecycleEntry{firstSeen: now, leasedNow: true}
			ev.Births = append(ev.Births, ip)
		case !e.leasedNow && !e.tombstoneFrom.IsZero():
			// Rebirth: same IP re-leased during tombstone → discard state.
			e.tombstoneFrom = time.Time{}
			e.leasedNow = true
			e.firstSeen = now
			ev.Rebirths = append(ev.Rebirths, ip)
		default:
			e.leasedNow = true
		}
	}
	for ip, e := range l.entries {
		if _, stillLeased := seen[ip]; stillLeased {
			continue
		}
		if e.leasedNow {
			e.leasedNow = false
			e.tombstoneFrom = now
			ev.Deaths = append(ev.Deaths, ip)
		}
	}

	onBirth := l.OnBirth
	onDeath := l.OnDeath
	onRebirth := l.OnRebirth
	l.mu.Unlock()

	for _, ip := range ev.Births {
		if onBirth != nil {
			onBirth(ip)
		}
	}
	for _, ip := range ev.Deaths {
		if onDeath != nil {
			onDeath(ip)
		}
	}
	for _, ip := range ev.Rebirths {
		if onRebirth != nil {
			onRebirth(ip)
		}
	}
	return ev
}

// Status reports dynamic, expired, or unknown. Non-dynamic is never
// returned by this tracker; callers classify non-dynamic elsewhere
// (static hosts, neighbor-only entries).
func (l *LifecycleTracker) Status(ip netip.Addr) LeaseStatus {
	l.mu.RLock()
	defer l.mu.RUnlock()
	e, ok := l.entries[ip]
	if !ok {
		return LeaseStatusUnknown
	}
	if e.leasedNow {
		return LeaseStatusDynamic
	}
	return LeaseStatusExpired
}

// Reap removes tombstoned entries whose grace window has elapsed.
// Returns the list of reaped IPs so the caller can discard their state.
func (l *LifecycleTracker) Reap(now time.Time) []netip.Addr {
	l.mu.Lock()
	defer l.mu.Unlock()
	var reaped []netip.Addr
	for ip, e := range l.entries {
		if e.leasedNow || e.tombstoneFrom.IsZero() {
			continue
		}
		if now.Sub(e.tombstoneFrom) >= l.graceDur {
			delete(l.entries, ip)
			reaped = append(reaped, ip)
		}
	}
	return reaped
}
