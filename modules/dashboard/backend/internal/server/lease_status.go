package server

import (
	"net/netip"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/collector"
)

// clientLookup is the small surface a client-detail handler needs to
// resolve an IP's lease status. Production wires LifecycleTracker for
// Status (dynamic/expired/unknown) and a state-backed adapter for
// IsStatic / IsStaticOrNeighbor; tests pass a fake.
type clientLookup interface {
	Status(netip.Addr) collector.LeaseStatus
	IsStatic(netip.Addr) bool
	IsStaticOrNeighbor(netip.Addr) bool
}

// resolveLeaseStatus folds the lifecycle signal (dynamic / expired / unknown)
// and the client-registry signals (static / neighbor / conntrack) into a
// single LeaseStatus. Static hosts are distinguished from neighbor-only
// hosts because the runtime pre-registers static IPs with the per-client
// collectors at startup, so their rings exist and handlers can serve real
// data; neighbor / conntrack hosts stay on the non-dynamic short-circuit
// path. Returns LeaseStatusUnknown when no source recognises the IP —
// callers 404 in that case.
func resolveLeaseStatus(lookup clientLookup, ip netip.Addr) collector.LeaseStatus {
	s := lookup.Status(ip)
	if s == collector.LeaseStatusDynamic || s == collector.LeaseStatusExpired {
		return s
	}
	if lookup.IsStatic(ip) {
		return collector.LeaseStatusStatic
	}
	if lookup.IsStaticOrNeighbor(ip) {
		return collector.LeaseStatusNonDynamic
	}
	return collector.LeaseStatusUnknown
}
