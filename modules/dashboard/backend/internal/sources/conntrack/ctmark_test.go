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
