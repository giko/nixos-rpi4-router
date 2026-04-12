package proc

import (
	"os"
	"path/filepath"
	"testing"
)

const testMeminfo = `MemTotal:        3884292 kB
MemFree:          152340 kB
MemAvailable:    2145678 kB
Buffers:          123456 kB
Cached:          1654321 kB
SwapCached:            0 kB
Active:          1234567 kB
Inactive:         987654 kB
Active(anon):     567890 kB
Inactive(anon):   123456 kB
Active(file):     666677 kB
Inactive(file):   864198 kB
`

func TestReadMeminfo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "meminfo")
	if err := os.WriteFile(path, []byte(testMeminfo), 0644); err != nil {
		t.Fatal(err)
	}

	m, err := ReadMeminfo(path)
	if err != nil {
		t.Fatal(err)
	}

	// Values in kB * 1024 = bytes
	wantTotal := uint64(3884292) * 1024
	wantAvailable := uint64(2145678) * 1024
	wantFree := uint64(152340) * 1024

	if m.TotalBytes != wantTotal {
		t.Errorf("TotalBytes = %d, want %d", m.TotalBytes, wantTotal)
	}
	if m.AvailableBytes != wantAvailable {
		t.Errorf("AvailableBytes = %d, want %d", m.AvailableBytes, wantAvailable)
	}
	if m.FreeBytes != wantFree {
		t.Errorf("FreeBytes = %d, want %d", m.FreeBytes, wantFree)
	}
}

func TestReadMeminfo_MissingFile(t *testing.T) {
	_, err := ReadMeminfo("/nonexistent/proc/meminfo")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestReadMeminfo_IncompleteData(t *testing.T) {
	content := "MemTotal:        3884292 kB\nMemFree:          152340 kB\n"
	path := filepath.Join(t.TempDir(), "meminfo_incomplete")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := ReadMeminfo(path)
	if err == nil {
		t.Fatal("expected error for incomplete meminfo")
	}
}
