package ipneigh

import (
	"context"
	"fmt"
	"testing"
)

const neighFixture = `192.168.1.10 dev eth0 lladdr aa:bb:cc:dd:ee:ff REACHABLE
192.168.1.20 dev eth0 lladdr 11:22:33:44:55:66 STALE
192.168.1.30 dev eth0  FAILED
169.254.0.1 dev eth1 lladdr de:ad:be:ef:00:00 PERMANENT`

func TestParseNeigh(t *testing.T) {
	m := parseNeigh(neighFixture)
	if len(m) != 3 {
		t.Fatalf("got %d entries, want 3", len(m))
	}

	want := map[string]string{
		"192.168.1.10": "aa:bb:cc:dd:ee:ff",
		"192.168.1.20": "11:22:33:44:55:66",
		"169.254.0.1":  "de:ad:be:ef:00:00",
	}
	for ip, mac := range want {
		got, ok := m[ip]
		if !ok {
			t.Errorf("missing entry for %s", ip)
			continue
		}
		if got != mac {
			t.Errorf("m[%q] = %q, want %q", ip, got, mac)
		}
	}

	// FAILED entry must be absent.
	if _, ok := m["192.168.1.30"]; ok {
		t.Error("192.168.1.30 (FAILED) should be filtered out")
	}
}

func TestParseNeighEmpty(t *testing.T) {
	m := parseNeigh("")
	if len(m) != 0 {
		t.Errorf("got %d entries, want 0", len(m))
	}
}

func TestParseNeighIncomplete(t *testing.T) {
	m := parseNeigh("192.168.1.50 dev eth0 INCOMPLETE\n")
	if len(m) != 0 {
		t.Errorf("got %d entries, want 0", len(m))
	}
}

func TestParseNeighMACLowercase(t *testing.T) {
	m := parseNeigh("192.168.1.10 dev eth0 lladdr AA:BB:CC:DD:EE:FF REACHABLE\n")
	if m["192.168.1.10"] != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("MAC not lowercased: got %q", m["192.168.1.10"])
	}
}

func TestCollect(t *testing.T) {
	fakeRun := func(_ context.Context, args ...string) (string, error) {
		return neighFixture, nil
	}

	m, err := Collect(context.Background(), fakeRun)
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 3 {
		t.Fatalf("got %d entries, want 3", len(m))
	}
}

func TestCollectRunnerError(t *testing.T) {
	fakeRun := func(_ context.Context, _ ...string) (string, error) {
		return "", fmt.Errorf("command not found")
	}

	_, err := Collect(context.Background(), fakeRun)
	if err == nil {
		t.Fatal("expected error from runner failure")
	}
}
