package dnsmasq

import (
	"os"
	"path/filepath"
	"testing"
)

const leasesFixture = `1712900000 aa:bb:cc:dd:ee:ff 192.168.1.10 giko-iphone 01:aa:bb:cc:dd:ee:ff
1712850000 11:22:33:44:55:66 192.168.1.20 * *
1712800000 AA:AA:BB:BB:CC:CC 192.168.1.30 sonos-beam 01:aa:aa:bb:bb:cc:cc`

func TestReadLeases(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dnsmasq.leases")
	if err := os.WriteFile(path, []byte(leasesFixture), 0644); err != nil {
		t.Fatal(err)
	}

	leases, err := ReadLeases(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(leases) != 3 {
		t.Fatalf("got %d leases, want 3", len(leases))
	}

	// First lease: normal fields.
	if leases[0].ExpireUnix != 1712900000 {
		t.Errorf("lease[0].ExpireUnix = %d, want 1712900000", leases[0].ExpireUnix)
	}
	if leases[0].MAC != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("lease[0].MAC = %q, want %q", leases[0].MAC, "aa:bb:cc:dd:ee:ff")
	}
	if leases[0].IP != "192.168.1.10" {
		t.Errorf("lease[0].IP = %q, want %q", leases[0].IP, "192.168.1.10")
	}
	if leases[0].Hostname != "giko-iphone" {
		t.Errorf("lease[0].Hostname = %q, want %q", leases[0].Hostname, "giko-iphone")
	}
	if leases[0].ClientID != "01:aa:bb:cc:dd:ee:ff" {
		t.Errorf("lease[0].ClientID = %q, want %q", leases[0].ClientID, "01:aa:bb:cc:dd:ee:ff")
	}

	// ExpiresAt sanity check.
	if leases[0].ExpiresAt().Unix() != 1712900000 {
		t.Errorf("ExpiresAt().Unix() = %d, want 1712900000", leases[0].ExpiresAt().Unix())
	}

	// Second lease: hostname "*" and clientID "*" become "".
	if leases[1].Hostname != "" {
		t.Errorf("lease[1].Hostname = %q, want empty", leases[1].Hostname)
	}
	if leases[1].ClientID != "" {
		t.Errorf("lease[1].ClientID = %q, want empty", leases[1].ClientID)
	}

	// Third lease: MAC should be lowercased.
	if leases[2].MAC != "aa:aa:bb:bb:cc:cc" {
		t.Errorf("lease[2].MAC = %q, want %q", leases[2].MAC, "aa:aa:bb:bb:cc:cc")
	}
}

func TestReadLeasesMissing(t *testing.T) {
	leases, err := ReadLeases(filepath.Join(t.TempDir(), "nonexistent"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if leases != nil {
		t.Errorf("got %v, want nil", leases)
	}
}
