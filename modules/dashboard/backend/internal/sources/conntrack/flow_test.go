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
		LANPrefixes: []netip.Prefix{
			netip.MustParsePrefix("192.168.1.0/24"),
		},
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
	flows, err := EnumerateFlows(f, EnumerateOpts{
		LANPrefixes: []netip.Prefix{
			netip.MustParsePrefix("192.168.1.0/24"),
		},
	})
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

func TestEnumerateInboundDNAT(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata", "inbound_dnat.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	flows, err := EnumerateFlows(f, EnumerateOpts{
		RouteTags: map[uint32]string{0x10000: "WAN"},
		LANPrefixes: []netip.Prefix{
			netip.MustParsePrefix("192.168.1.0/24"),
		},
	})
	if err != nil {
		t.Fatalf("EnumerateFlows: %v", err)
	}
	if len(flows) != 1 {
		t.Fatalf("expected 1 flow, got %d", len(flows))
	}
	fb := flows[0]
	if fb.Direction != DirInbound {
		t.Errorf("Direction = %v, want DirInbound", fb.Direction)
	}
	if fb.ClientIP.String() != "192.168.1.50" {
		t.Errorf("ClientIP = %s, want 192.168.1.50", fb.ClientIP)
	}
	if fb.RemoteIP.String() != "203.0.113.9" {
		t.Errorf("RemoteIP = %s, want 203.0.113.9", fb.RemoteIP)
	}
	if fb.RemotePort != 55102 {
		t.Errorf("RemotePort = %d, want 55102", fb.RemotePort)
	}
	if fb.NATPublicIP.String() != "198.51.100.4" {
		t.Errorf("NATPublicIP = %s, want 198.51.100.4", fb.NATPublicIP)
	}
	if fb.NATPublicPort != 32400 {
		t.Errorf("NATPublicPort = %d, want 32400", fb.NATPublicPort)
	}
	if fb.LocalPort != 32400 {
		t.Errorf("LocalPort = %d, want 32400", fb.LocalPort)
	}
	if fb.RouteTag != "WAN" {
		t.Errorf("RouteTag = %q, want WAN", fb.RouteTag)
	}
	// Outbound orig bytes = client RX, reply bytes = client TX on inbound flows
	// (from client's perspective: data going to the client came from the peer
	// via orig; data leaving the client went out via reply).
	if fb.OrigBytes != 1_100_000 {
		t.Errorf("OrigBytes = %d, want 1_100_000 (peer → client)", fb.OrigBytes)
	}
	if fb.ReplyBytes != 82_000 {
		t.Errorf("ReplyBytes = %d, want 82_000 (client → peer)", fb.ReplyBytes)
	}
}

// TestEnumerateDoubleNATRouterSessionIsNotDNAT guards against a subtle
// misclassification: when the router's WAN address is itself in RFC1918
// / CGNAT space (double-NAT or ISP-issued 100.64.0.0/10), an inbound SSH
// session terminating on the router has a private reply src — but that
// reply src is the router itself, not a LAN host. The old IsPrivate()
// heuristic wrongly labelled this as inbound DNAT with ClientIP = the
// router's WAN IP. With explicit LAN prefixes that exclude 100.64/10, the
// flow must be dropped by parseLine's default branch.
func TestEnumerateDoubleNATRouterSessionIsNotDNAT(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata", "double_nat_router_session.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	flows, err := EnumerateFlows(f, EnumerateOpts{
		RouteTags: map[uint32]string{0x10000: "WAN"},
		LANPrefixes: []netip.Prefix{
			netip.MustParsePrefix("192.168.1.0/24"),
			netip.MustParsePrefix("192.168.20.0/24"),
		},
	})
	if err != nil {
		t.Fatalf("EnumerateFlows: %v", err)
	}
	if len(flows) != 0 {
		t.Fatalf("router-terminated session should be skipped (not a LAN host), got %d flows", len(flows))
	}
}

// TestEnumerateRoutedInboundIsNotDNAT guards against another
// misclassification: a peer on a non-LAN, non-public range (e.g. another
// private site reachable via a site-to-site VPN at 10.10.0.5) reaches a
// LAN host (192.168.1.50) directly, without NAT. The tuple shape matches
// the inbound-DNAT heuristic (orig src non-LAN, reply src LAN) but no
// NAT rewrite happened — orig dst is already the LAN host. Labelling
// this as DirInbound with NATPublicIP set would fabricate port-forward
// metadata. The additional guard (orig dst must NOT be LAN) drops the
// row through the default branch.
func TestEnumerateRoutedInboundIsNotDNAT(t *testing.T) {
	f, err := os.Open(filepath.Join("testdata", "routed_inbound_no_nat.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	flows, err := EnumerateFlows(f, EnumerateOpts{
		LANPrefixes: []netip.Prefix{
			netip.MustParsePrefix("192.168.1.0/24"),
		},
	})
	if err != nil {
		t.Fatalf("EnumerateFlows: %v", err)
	}
	if len(flows) != 0 {
		t.Fatalf("routed LAN flow should be skipped (not DNAT), got %d flows", len(flows))
	}
}
