package proc

import (
	"os"
	"path/filepath"
	"testing"
)

const testNetDev = `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo: 1234567     890    0    0    0     0          0         0  1234567     890    0    0    0     0       0          0
  eth0: 98765432101  56789012    0    0    0     0          0   1234 12345678901  34567890    0    0    0     0       0          0
  eth1: 5432109876  4321098    0  123    0     0          0      0  1098765432  876543    0    0    0     0       0          0
 wg_sw:  987654321   654321    0    0    0     0          0      0   123456789   456789    0    0    0     0       0          0
`

func TestReadNetDev(t *testing.T) {
	path := filepath.Join(t.TempDir(), "net_dev")
	if err := os.WriteFile(path, []byte(testNetDev), 0644); err != nil {
		t.Fatal(err)
	}

	stats, err := ReadNetDev(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(stats) != 4 {
		t.Fatalf("expected 4 interfaces, got %d", len(stats))
	}

	tests := []struct {
		iface     string
		rxBytes   uint64
		rxPackets uint64
		txBytes   uint64
		txPackets uint64
	}{
		{"lo", 1234567, 890, 1234567, 890},
		{"eth0", 98765432101, 56789012, 12345678901, 34567890},
		{"eth1", 5432109876, 4321098, 1098765432, 876543},
		{"wg_sw", 987654321, 654321, 123456789, 456789},
	}

	for _, tc := range tests {
		t.Run(tc.iface, func(t *testing.T) {
			s, ok := stats[tc.iface]
			if !ok {
				t.Fatalf("interface %q not found", tc.iface)
			}
			if s.Interface != tc.iface {
				t.Errorf("Interface = %q, want %q", s.Interface, tc.iface)
			}
			if s.RXBytes != tc.rxBytes {
				t.Errorf("RXBytes = %d, want %d", s.RXBytes, tc.rxBytes)
			}
			if s.RXPackets != tc.rxPackets {
				t.Errorf("RXPackets = %d, want %d", s.RXPackets, tc.rxPackets)
			}
			if s.TXBytes != tc.txBytes {
				t.Errorf("TXBytes = %d, want %d", s.TXBytes, tc.txBytes)
			}
			if s.TXPackets != tc.txPackets {
				t.Errorf("TXPackets = %d, want %d", s.TXPackets, tc.txPackets)
			}
		})
	}
}

func TestReadNetDev_MissingFile(t *testing.T) {
	_, err := ReadNetDev("/nonexistent/proc/net/dev")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestReadNetDev_TooFewFields(t *testing.T) {
	content := `Inter-|   Receive
 face |bytes
  eth0: 123 456
`
	path := filepath.Join(t.TempDir(), "net_dev_short")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := ReadNetDev(path)
	if err == nil {
		t.Fatal("expected error for too few fields")
	}
}
