package conntrack

import (
	"context"
	"fmt"
	"testing"
)

func TestCountByFwmark(t *testing.T) {
	fakeRun := func(_ context.Context, args ...string) (string, error) {
		return "ipv4     2 tcp      6 60 src=192.168.1.10 dst=1.1.1.1 sport=5 dport=443 mark=131072\n" +
			"ipv4     2 tcp      6 60 src=192.168.1.11 dst=8.8.8.8 sport=5 dport=443 mark=131072\n" +
			"ipv4     2 udp      17 30 src=192.168.1.12 dst=1.0.0.1 sport=5 dport=53 mark=131072\n", nil
	}

	n, err := CountByFwmark(context.Background(), fakeRun, "0x20000")
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("count = %d, want 3", n)
	}
}

func TestCountByFwmarkEmpty(t *testing.T) {
	fakeRun := func(_ context.Context, _ ...string) (string, error) {
		return "", nil
	}

	n, err := CountByFwmark(context.Background(), fakeRun, "0x20000")
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("count = %d, want 0", n)
	}
}

func TestCountByFwmarkTrailingNewline(t *testing.T) {
	fakeRun := func(_ context.Context, _ ...string) (string, error) {
		return "line1\nline2\n", nil
	}

	n, err := CountByFwmark(context.Background(), fakeRun, "0x20000")
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("count = %d, want 2", n)
	}
}

func TestCountByFwmarkRunnerError(t *testing.T) {
	fakeRun := func(_ context.Context, _ ...string) (string, error) {
		return "", fmt.Errorf("connection refused")
	}

	_, err := CountByFwmark(context.Background(), fakeRun, "0x20000")
	if err == nil {
		t.Fatal("expected error from runner failure")
	}
}
