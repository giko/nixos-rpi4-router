package conntrack

import (
	"net/netip"
	"testing"
)

func TestFlowKeyStableAcrossDirections(t *testing.T) {
	a := FlowKey{
		Proto:       6,
		OrigSrcIP:   netip.MustParseAddr("192.168.1.42"),
		OrigDstIP:   netip.MustParseAddr("52.84.17.12"),
		OrigSrcPort: 47182,
		OrigDstPort: 443,
	}
	b := a // identical
	if a != b {
		t.Fatalf("FlowKey should be comparable, got %v vs %v", a, b)
	}
}

func TestDirectionString(t *testing.T) {
	cases := []struct {
		d    Direction
		want string
	}{
		{DirOutbound, "outbound"},
		{DirInbound, "inbound"},
		{DirUnknown, "unknown"},
	}
	for _, c := range cases {
		if got := c.d.String(); got != c.want {
			t.Fatalf("Direction(%d).String() = %q, want %q", c.d, got, c.want)
		}
	}
}
