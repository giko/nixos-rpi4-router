package server

import (
	"net/netip"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/collector"
)

// clientLookup is the small surface a client-detail handler needs to
// resolve an IP's lease status. Production wires LifecycleTracker for
// Status (dynamic/expired/unknown) and a state-backed adapter for
// IsStaticOrNeighbor; tests pass a fake.
type clientLookup interface {
	Status(netip.Addr) collector.LeaseStatus
	IsStaticOrNeighbor(netip.Addr) bool
}

// resolveLeaseStatus folds the two signals into a single LeaseStatus.
// Returns LeaseStatusUnknown when neither the lifecycle tracker nor the
// static/neighbor source recognises the IP — callers should 404 in that
// case before calling this helper for a body field.
func resolveLeaseStatus(lookup clientLookup, ip netip.Addr) collector.LeaseStatus {
	s := lookup.Status(ip)
	if s == collector.LeaseStatusDynamic || s == collector.LeaseStatusExpired {
		return s
	}
	if lookup.IsStaticOrNeighbor(ip) {
		return collector.LeaseStatusNonDynamic
	}
	return collector.LeaseStatusUnknown
}
