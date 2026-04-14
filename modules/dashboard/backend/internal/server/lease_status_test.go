package server

import (
	"net/netip"
	"testing"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/collector"
)

type fakeLookup struct {
	status      collector.LeaseStatus
	static      bool
	staticOrNbr bool
}

func (f fakeLookup) Status(netip.Addr) collector.LeaseStatus { return f.status }
func (f fakeLookup) IsStatic(netip.Addr) bool                { return f.static }
func (f fakeLookup) IsStaticOrNeighbor(netip.Addr) bool      { return f.staticOrNbr }

func TestResolveLeaseStatus(t *testing.T) {
	ip := netip.MustParseAddr("192.168.1.10")
	cases := []struct {
		name string
		look fakeLookup
		want collector.LeaseStatus
	}{
		{"dynamic", fakeLookup{status: collector.LeaseStatusDynamic}, collector.LeaseStatusDynamic},
		{"expired", fakeLookup{status: collector.LeaseStatusExpired}, collector.LeaseStatusExpired},
		{"static", fakeLookup{status: collector.LeaseStatusUnknown, static: true, staticOrNbr: true}, collector.LeaseStatusStatic},
		{"neighbor-or-conntrack", fakeLookup{status: collector.LeaseStatusUnknown, staticOrNbr: true}, collector.LeaseStatusNonDynamic},
		{"unknown", fakeLookup{status: collector.LeaseStatusUnknown}, collector.LeaseStatusUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveLeaseStatus(tc.look, ip)
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}
