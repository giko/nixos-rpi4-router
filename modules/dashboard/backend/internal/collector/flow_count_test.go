package collector

import (
	"net/netip"
	"testing"
	"time"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/conntrack"
)

func TestFlowCountCountsPerClient(t *testing.T) {
	a := netip.MustParseAddr("192.168.1.42")
	b := netip.MustParseAddr("192.168.1.50")
	c := NewFlowCount(FlowCountOpts{TickDur: 10 * time.Second})
	c.Track(a)
	c.Track(b)
	c.Apply(time.Unix(0, 0), []conntrack.FlowBytes{
		{ClientIP: a}, {ClientIP: a}, {ClientIP: a},
		{ClientIP: b},
	})
	sA, _ := c.Snapshot(a)
	if len(sA) != 1 || sA[0].OpenFlows != 3 {
		t.Fatalf("a samples = %+v, want 1 sample with OpenFlows=3", sA)
	}
	sB, _ := c.Snapshot(b)
	if len(sB) != 1 || sB[0].OpenFlows != 1 {
		t.Fatalf("b samples = %+v, want 1 sample with OpenFlows=1", sB)
	}
}
