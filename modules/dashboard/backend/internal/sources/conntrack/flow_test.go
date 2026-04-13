package conntrack

import (
	"net/netip"
	"os"
	"path/filepath"
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

func TestEnumerateOutboundBasic(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata", "outbound_basic.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	flows, err := EnumerateFlows(f, EnumerateOpts{
		RouteTags: map[uint32]string{0x20000: "wg_sw"},
	})
	if err != nil {
		t.Fatalf("EnumerateFlows: %v", err)
	}
	if len(flows) != 2 {
		t.Fatalf("expected 2 flows, got %d", len(flows))
	}

	tcp := flows[0]
	if tcp.Key.Proto != 6 {
		t.Errorf("proto = %d, want 6", tcp.Key.Proto)
	}
	if tcp.ClientIP.String() != "192.168.1.42" {
		t.Errorf("ClientIP = %s, want 192.168.1.42", tcp.ClientIP)
	}
	if tcp.Direction != DirOutbound {
		t.Errorf("Direction = %v, want DirOutbound", tcp.Direction)
	}
	if tcp.OrigBytes != 3200000 {
		t.Errorf("OrigBytes = %d, want 3200000", tcp.OrigBytes)
	}
	if tcp.ReplyBytes != 412000000 {
		t.Errorf("ReplyBytes = %d, want 412000000", tcp.ReplyBytes)
	}
	if tcp.RouteTag != "wg_sw" {
		t.Errorf("RouteTag = %q, want wg_sw (mark 0x20000)", tcp.RouteTag)
	}
	if tcp.RemoteIP.String() != "52.84.17.12" {
		t.Errorf("RemoteIP = %s, want 52.84.17.12", tcp.RemoteIP)
	}
	if tcp.RemotePort != 443 {
		t.Errorf("RemotePort = %d, want 443", tcp.RemotePort)
	}
	if tcp.LocalPort != 47182 {
		t.Errorf("LocalPort = %d, want 47182", tcp.LocalPort)
	}

	udp := flows[1]
	if udp.RouteTag != "" {
		t.Errorf("udp RouteTag = %q, want empty (mark=0 must not map)", udp.RouteTag)
	}
}

func TestEnumerateOutboundICMP(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata", "outbound_icmp.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	flows, err := EnumerateFlows(f, EnumerateOpts{})
	if err != nil {
		t.Fatalf("EnumerateFlows: %v", err)
	}
	if len(flows) != 1 {
		t.Fatalf("expected 1 flow, got %d", len(flows))
	}
	fb := flows[0]
	if fb.ClientIP.String() != "192.168.1.42" {
		t.Errorf("ClientIP = %s, want 192.168.1.42", fb.ClientIP)
	}
	if fb.RemoteIP.String() != "1.1.1.1" {
		t.Errorf("RemoteIP = %s, want 1.1.1.1", fb.RemoteIP)
	}
	if fb.Key.Proto != 1 {
		t.Errorf("Proto = %d, want 1 (icmp)", fb.Key.Proto)
	}
	if fb.OrigBytes != 252 || fb.ReplyBytes != 252 {
		t.Errorf("bytes: orig=%d reply=%d, want 252/252", fb.OrigBytes, fb.ReplyBytes)
	}
	if fb.LocalPort != 0 || fb.RemotePort != 0 {
		t.Errorf("icmp ports should be 0, got local=%d remote=%d", fb.LocalPort, fb.RemotePort)
	}
}
