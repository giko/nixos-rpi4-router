package poolhealth

import (
	"os"
	"path/filepath"
	"testing"
)

const stateFixture = `{
  "updated_at": "2026-04-11T12:00:00Z",
  "tunnels": {
    "wg_sw": { "healthy": true, "consecutive_failures": 0 },
    "wg_us": { "healthy": false, "consecutive_failures": 3 }
  }
}`

func TestReadState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(stateFixture), 0644); err != nil {
		t.Fatal(err)
	}

	s, err := ReadState(path)
	if err != nil {
		t.Fatal(err)
	}

	if s.UpdatedAt != "2026-04-11T12:00:00Z" {
		t.Errorf("UpdatedAt = %q, want %q", s.UpdatedAt, "2026-04-11T12:00:00Z")
	}

	if len(s.Tunnels) != 2 {
		t.Fatalf("expected 2 tunnels, got %d", len(s.Tunnels))
	}

	sw, ok := s.Tunnels["wg_sw"]
	if !ok {
		t.Fatal("wg_sw not found in tunnels")
	}
	if !sw.Healthy {
		t.Error("wg_sw should be healthy")
	}
	if sw.ConsecutiveFailures != 0 {
		t.Errorf("wg_sw ConsecutiveFailures = %d, want 0", sw.ConsecutiveFailures)
	}

	us, ok := s.Tunnels["wg_us"]
	if !ok {
		t.Fatal("wg_us not found in tunnels")
	}
	if us.Healthy {
		t.Error("wg_us should not be healthy")
	}
	if us.ConsecutiveFailures != 3 {
		t.Errorf("wg_us ConsecutiveFailures = %d, want 3", us.ConsecutiveFailures)
	}
}

func TestReadStateMissingFile(t *testing.T) {
	s, err := ReadState("/nonexistent/path/state.json")
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}

	if s.Tunnels == nil {
		t.Fatal("Tunnels map should be initialized, not nil")
	}
	if len(s.Tunnels) != 0 {
		t.Errorf("expected 0 tunnels, got %d", len(s.Tunnels))
	}
	if s.UpdatedAt != "" {
		t.Errorf("UpdatedAt = %q, want empty string", s.UpdatedAt)
	}
}

func TestReadStateMalformedJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte("{not valid json"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := ReadState(path)
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestReadStateNilTunnels(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nil_tunnels.json")
	if err := os.WriteFile(path, []byte(`{"updated_at":"now"}`), 0644); err != nil {
		t.Fatal(err)
	}

	s, err := ReadState(path)
	if err != nil {
		t.Fatal(err)
	}

	if s.Tunnels == nil {
		t.Fatal("Tunnels map should be initialized even when absent from JSON")
	}
	if len(s.Tunnels) != 0 {
		t.Errorf("expected 0 tunnels, got %d", len(s.Tunnels))
	}
}
