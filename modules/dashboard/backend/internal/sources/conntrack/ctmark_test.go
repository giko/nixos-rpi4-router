package conntrack

import (
	"context"
	"fmt"
	"testing"
)

const ctmarkFixture = `ipv4     2 tcp      6 60 src=192.168.1.10 dst=1.1.1.1 sport=5 dport=443 mark=131072
ipv4     2 tcp      6 60 src=192.168.1.10 dst=2.2.2.2 sport=6 dport=80 mark=196608
ipv4     2 tcp      6 60 src=192.168.1.11 dst=8.8.8.8 sport=5 dport=443 mark=196608
ipv4     2 udp      17 30 src=192.168.1.12 dst=1.0.0.1 sport=5 dport=53 mark=0`

func TestClientConnections(t *testing.T) {
	fakeRun := func(_ context.Context, _ ...string) (string, error) {
		return ctmarkFixture, nil
	}

	m, err := ClientConnections(context.Background(), fakeRun)
	if err != nil {
		t.Fatal(err)
	}

	// .10 has 2 connections: one on 0x20000, one on 0x30000
	info10 := m["192.168.1.10"]
	if info10.TotalConns != 2 {
		t.Errorf(".10 total = %d, want 2", info10.TotalConns)
	}
	if info10.TunnelConns["0x20000"] != 1 {
		t.Errorf(".10 0x20000 = %d, want 1", info10.TunnelConns["0x20000"])
	}
	if info10.TunnelConns["0x30000"] != 1 {
		t.Errorf(".10 0x30000 = %d, want 1", info10.TunnelConns["0x30000"])
	}

	// .11 has 1 connection on 0x30000
	info11 := m["192.168.1.11"]
	if info11.TotalConns != 1 {
		t.Errorf(".11 total = %d, want 1", info11.TotalConns)
	}

	// .12 has 1 connection with mark=0 (WAN), still counted in total
	info12 := m["192.168.1.12"]
	if info12.TotalConns != 1 {
		t.Errorf(".12 total = %d, want 1", info12.TotalConns)
	}
	if len(info12.TunnelConns) != 0 {
		t.Errorf(".12 tunnel conns should be empty (mark=0), got %v", info12.TunnelConns)
	}
}

func TestClientConnectionsEmpty(t *testing.T) {
	fakeRun := func(_ context.Context, _ ...string) (string, error) {
		return "", nil
	}
	m, err := ClientConnections(context.Background(), fakeRun)
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 0 {
		t.Errorf("expected empty, got %d", len(m))
	}
}

func TestClientConnectionsRunnerError(t *testing.T) {
	fakeRun := func(_ context.Context, _ ...string) (string, error) {
		return "", fmt.Errorf("permission denied")
	}
	_, err := ClientConnections(context.Background(), fakeRun)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClientConnectionsAttributesInboundDNATToLANHost(t *testing.T) {
	// conntrack -L prints the original tuple src/dst followed by the
	// reply tuple src/dst. For inbound DNAT (port forward), the
	// original src is the remote peer's public IP — attributing to it
	// leaves the LAN host at conn_count=0. We want the reply src (LAN).
	//
	// Layout on one line:
	//   orig src=<remote> dst=<wan_ip> sport=... dport=80
	//   reply src=<lan_host> dst=<remote> sport=8080 dport=...
	fixture := "ipv4     2 tcp      6 60 src=203.0.113.50 dst=198.51.100.2 sport=54321 dport=80 " +
		"src=192.168.1.200 dst=203.0.113.50 sport=8080 dport=54321 [ASSURED] mark=0\n" +
		// A regular outbound session should still attribute to orig src.
		"ipv4     2 tcp      6 60 src=192.168.1.10 dst=1.1.1.1 sport=5 dport=443 " +
		"src=1.1.1.1 dst=198.51.100.2 sport=443 dport=5 mark=131072\n"

	fakeRun := func(_ context.Context, _ ...string) (string, error) {
		return fixture, nil
	}
	m, err := ClientConnections(context.Background(), fakeRun)
	if err != nil {
		t.Fatal(err)
	}
	// DNAT session: LAN host (reply src) must own the count.
	if m["192.168.1.200"].TotalConns != 1 {
		t.Errorf(".200 (DNAT) total = %d, want 1; map = %+v", m["192.168.1.200"].TotalConns, m)
	}
	// Remote peer public IP must NOT become a client.
	if _, ok := m["203.0.113.50"]; ok {
		t.Errorf("remote peer 203.0.113.50 should not be attributed")
	}
	// Outbound session still attributes normally.
	if m["192.168.1.10"].TotalConns != 1 {
		t.Errorf(".10 outbound total = %d, want 1", m["192.168.1.10"].TotalConns)
	}
	if m["192.168.1.10"].TunnelConns["0x20000"] != 1 {
		t.Errorf(".10 tunnel count = %d, want 1", m["192.168.1.10"].TunnelConns["0x20000"])
	}
}

func TestExtractField(t *testing.T) {
	line := "ipv4     2 tcp      6 60 src=192.168.1.10 dst=1.1.1.1 sport=5 dport=443 mark=131072"
	tests := []struct {
		key  string
		want string
	}{
		{"src", "192.168.1.10"},
		{"dst", "1.1.1.1"},
		{"mark", "131072"},
		{"missing", ""},
	}
	for _, tc := range tests {
		if got := extractField(line, tc.key); got != tc.want {
			t.Errorf("extractField(%q) = %q, want %q", tc.key, got, tc.want)
		}
	}
}
