package collector

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/state"
)

const netdevFixture1 = `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
  eth0: 1000000     1000    0    0    0     0          0         0  2000000     2000    0    0    0     0       0          0
  eth1:  500000      500    0    0    0     0          0         0  1000000     1000    0    0    0     0       0          0
`

const netdevFixture2 = `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
  eth0: 1002000     1010    0    0    0     0          0         0  2004000     2020    0    0    0     0       0          0
  eth1:  501000      505    0    0    0     0          0         0  1002000     1010    0    0    0     0       0          0
`

func TestTrafficCollector(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "net_dev")

	if err := os.WriteFile(path, []byte(netdevFixture1), 0644); err != nil {
		t.Fatal(err)
	}

	st := state.New()
	tc := NewTraffic(TrafficOpts{
		NetDevPath: path,
		Interfaces: []string{"eth0", "eth1"},
		State:      st,
	})

	ctx := context.Background()

	// First run: rates should be 0 (no previous sample).
	if err := tc.Run(ctx); err != nil {
		t.Fatalf("first Run: %v", err)
	}

	snap, updated := st.SnapshotTraffic()
	if updated.IsZero() {
		t.Fatal("traffic updated_at is zero after first run")
	}
	if len(snap.Interfaces) != 2 {
		t.Fatalf("expected 2 interfaces, got %d", len(snap.Interfaces))
	}

	for _, iface := range snap.Interfaces {
		if iface.RXBps != 0 || iface.TXBps != 0 {
			t.Errorf("%s: first run rates should be 0, got rx=%d tx=%d", iface.Name, iface.RXBps, iface.TXBps)
		}
		if len(iface.Samples60s) != 1 {
			t.Errorf("%s: expected 1 sample, got %d", iface.Name, len(iface.Samples60s))
		}
		// Verify total counters.
		switch iface.Name {
		case "eth0":
			if iface.RXBytesTotal != 1000000 {
				t.Errorf("eth0 RXBytesTotal = %d, want 1000000", iface.RXBytesTotal)
			}
			if iface.TXBytesTotal != 2000000 {
				t.Errorf("eth0 TXBytesTotal = %d, want 2000000", iface.TXBytesTotal)
			}
		case "eth1":
			if iface.RXBytesTotal != 500000 {
				t.Errorf("eth1 RXBytesTotal = %d, want 500000", iface.RXBytesTotal)
			}
			if iface.TXBytesTotal != 1000000 {
				t.Errorf("eth1 TXBytesTotal = %d, want 1000000", iface.TXBytesTotal)
			}
		}
	}

	// Write second fixture with increased counters.
	if err := os.WriteFile(path, []byte(netdevFixture2), 0644); err != nil {
		t.Fatal(err)
	}

	// Second run: should compute non-zero rates.
	if err := tc.Run(ctx); err != nil {
		t.Fatalf("second Run: %v", err)
	}

	snap, _ = st.SnapshotTraffic()
	for _, iface := range snap.Interfaces {
		// Rates should be positive (exact values depend on elapsed time,
		// but the delta is non-zero so rates must be > 0).
		if iface.RXBps == 0 {
			t.Errorf("%s: second run RXBps should be > 0", iface.Name)
		}
		if iface.TXBps == 0 {
			t.Errorf("%s: second run TXBps should be > 0", iface.Name)
		}
		if len(iface.Samples60s) != 2 {
			t.Errorf("%s: expected 2 samples, got %d", iface.Name, len(iface.Samples60s))
		}
		// Updated total counters.
		switch iface.Name {
		case "eth0":
			if iface.RXBytesTotal != 1002000 {
				t.Errorf("eth0 RXBytesTotal = %d, want 1002000", iface.RXBytesTotal)
			}
		case "eth1":
			if iface.RXBytesTotal != 501000 {
				t.Errorf("eth1 RXBytesTotal = %d, want 501000", iface.RXBytesTotal)
			}
		}
	}
}

func TestTrafficCollectorSetsRoleFromMap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "net_dev")
	if err := os.WriteFile(path, []byte(netdevFixture1), 0644); err != nil {
		t.Fatal(err)
	}

	st := state.New()
	tc := NewTraffic(TrafficOpts{
		NetDevPath: path,
		Interfaces: []string{"eth0", "eth1"},
		Roles:      map[string]string{"eth0": "lan", "eth1": "wan"},
		State:      st,
	})

	if err := tc.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	snap, _ := st.SnapshotTraffic()
	want := map[string]string{"eth0": "lan", "eth1": "wan"}
	for _, iface := range snap.Interfaces {
		if iface.Role != want[iface.Name] {
			t.Errorf("%s role = %q, want %q", iface.Name, iface.Role, want[iface.Name])
		}
	}
}

func TestTrafficCollectorRoleMissingYieldsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "net_dev")
	if err := os.WriteFile(path, []byte(netdevFixture1), 0644); err != nil {
		t.Fatal(err)
	}

	st := state.New()
	// Roles not provided — Role must be "" (the zero value) for every iface.
	tc := NewTraffic(TrafficOpts{
		NetDevPath: path,
		Interfaces: []string{"eth0"},
		State:      st,
	})
	if err := tc.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	snap, _ := st.SnapshotTraffic()
	if len(snap.Interfaces) != 1 {
		t.Fatalf("len = %d, want 1", len(snap.Interfaces))
	}
	if snap.Interfaces[0].Role != "" {
		t.Errorf("Role = %q, want empty", snap.Interfaces[0].Role)
	}
}

func TestTrafficRingBufferFull(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "net_dev")

	st := state.New()
	tc := NewTraffic(TrafficOpts{
		NetDevPath: path,
		Interfaces: []string{"eth0"},
		State:      st,
	})

	ctx := context.Background()

	// Run 65 times to exceed the 60-slot ring buffer.
	for i := 0; i < 65; i++ {
		rxBytes := 1000000 + uint64(i)*1000
		txBytes := 2000000 + uint64(i)*2000
		content := "Inter-|   Receive                                                |  Transmit\n" +
			" face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed\n"
		content += fmtNetDevLine("eth0", rxBytes, txBytes)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		if err := tc.Run(ctx); err != nil {
			t.Fatalf("Run %d: %v", i, err)
		}
	}

	snap, _ := st.SnapshotTraffic()
	if len(snap.Interfaces) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(snap.Interfaces))
	}
	if len(snap.Interfaces[0].Samples60s) != 60 {
		t.Errorf("expected 60 samples (ring capped), got %d", len(snap.Interfaces[0].Samples60s))
	}
}

func fmtNetDevLine(name string, rxBytes, txBytes uint64) string {
	// Minimal valid /proc/net/dev line with 16 fields after the colon.
	return "  " + name + ": " +
		uitoa(rxBytes) + " 100 0 0 0 0 0 0 " +
		uitoa(txBytes) + " 100 0 0 0 0 0 0\n"
}

func uitoa(v uint64) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte(v%10) + '0'
		v /= 10
	}
	return string(buf[i:])
}
