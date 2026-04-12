package conntrack

import (
	"context"
	"fmt"
	"testing"
)

const ctmarkFixture = `ipv4     2 tcp      6 60 src=192.168.1.10 dst=1.1.1.1 sport=5 dport=443 mark=131072
ipv4     2 tcp      6 60 src=192.168.1.11 dst=8.8.8.8 sport=5 dport=443 mark=196608
ipv4     2 udp      17 30 src=192.168.1.12 dst=1.0.0.1 sport=5 dport=53 mark=0`

func TestClientFwmarks(t *testing.T) {
	fakeRun := func(_ context.Context, _ ...string) (string, error) {
		return ctmarkFixture, nil
	}

	m, err := ClientFwmarks(context.Background(), fakeRun)
	if err != nil {
		t.Fatal(err)
	}

	// 131072 = 0x20000, 196608 = 0x30000
	if got, ok := m["192.168.1.10"]; !ok || got != "0x20000" {
		t.Errorf("192.168.1.10 = %q, want %q", got, "0x20000")
	}
	if got, ok := m["192.168.1.11"]; !ok || got != "0x30000" {
		t.Errorf("192.168.1.11 = %q, want %q", got, "0x30000")
	}
	if _, ok := m["192.168.1.12"]; ok {
		t.Error("192.168.1.12 should be absent (mark=0)")
	}
}

func TestClientFwmarksFirstWins(t *testing.T) {
	fixture := "ipv4     2 tcp      6 60 src=192.168.1.10 dst=1.1.1.1 sport=5 dport=443 mark=131072\n" +
		"ipv4     2 tcp      6 60 src=192.168.1.10 dst=2.2.2.2 sport=6 dport=80 mark=196608\n"

	fakeRun := func(_ context.Context, _ ...string) (string, error) {
		return fixture, nil
	}

	m, err := ClientFwmarks(context.Background(), fakeRun)
	if err != nil {
		t.Fatal(err)
	}

	// First-seen mark wins: 131072 = 0x20000
	if got := m["192.168.1.10"]; got != "0x20000" {
		t.Errorf("192.168.1.10 = %q, want %q (first-seen wins)", got, "0x20000")
	}
}

func TestClientFwmarksEmpty(t *testing.T) {
	fakeRun := func(_ context.Context, _ ...string) (string, error) {
		return "", nil
	}

	m, err := ClientFwmarks(context.Background(), fakeRun)
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 0 {
		t.Errorf("expected empty map, got %d entries", len(m))
	}
}

func TestClientFwmarksRunnerError(t *testing.T) {
	fakeRun := func(_ context.Context, _ ...string) (string, error) {
		return "", fmt.Errorf("permission denied")
	}

	_, err := ClientFwmarks(context.Background(), fakeRun)
	if err == nil {
		t.Fatal("expected error from runner failure")
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
		{"sport", "5"},
		{"dport", "443"},
		{"mark", "131072"},
		{"missing", ""},
	}

	for _, tc := range tests {
		got := extractField(line, tc.key)
		if got != tc.want {
			t.Errorf("extractField(%q) = %q, want %q", tc.key, got, tc.want)
		}
	}
}
