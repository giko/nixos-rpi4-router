package proc

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestReadThermal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "temp")
	if err := os.WriteFile(path, []byte("52314\n"), 0644); err != nil {
		t.Fatal(err)
	}

	temp, err := ReadThermal(path)
	if err != nil {
		t.Fatal(err)
	}

	want := 52.314
	if math.Abs(temp-want) > 0.001 {
		t.Errorf("ReadThermal = %f, want %f", temp, want)
	}
}

func TestReadThermal_NoNewline(t *testing.T) {
	path := filepath.Join(t.TempDir(), "temp")
	if err := os.WriteFile(path, []byte("45000"), 0644); err != nil {
		t.Fatal(err)
	}

	temp, err := ReadThermal(path)
	if err != nil {
		t.Fatal(err)
	}

	if math.Abs(temp-45.0) > 0.001 {
		t.Errorf("ReadThermal = %f, want 45.0", temp)
	}
}

func TestReadThermal_MissingFile(t *testing.T) {
	_, err := ReadThermal("/nonexistent/sys/thermal")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestReadUptime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "uptime")
	if err := os.WriteFile(path, []byte("12345.67 9876.54\n"), 0644); err != nil {
		t.Fatal(err)
	}

	uptime, err := ReadUptime(path)
	if err != nil {
		t.Fatal(err)
	}

	want := 12345.67
	if math.Abs(uptime-want) > 0.001 {
		t.Errorf("ReadUptime = %f, want %f", uptime, want)
	}
}

func TestReadUptime_LargeValue(t *testing.T) {
	path := filepath.Join(t.TempDir(), "uptime")
	if err := os.WriteFile(path, []byte("8640000.12 4320000.56\n"), 0644); err != nil {
		t.Fatal(err)
	}

	uptime, err := ReadUptime(path)
	if err != nil {
		t.Fatal(err)
	}

	want := 8640000.12
	if math.Abs(uptime-want) > 0.001 {
		t.Errorf("ReadUptime = %f, want %f", uptime, want)
	}
}

func TestReadUptime_MissingFile(t *testing.T) {
	_, err := ReadUptime("/nonexistent/proc/uptime")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestReadUptime_EmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "uptime_empty")
	if err := os.WriteFile(path, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := ReadUptime(path)
	if err == nil {
		t.Fatal("expected error for empty file")
	}
}
