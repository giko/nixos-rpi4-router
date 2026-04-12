package envelope

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWriteJSONStaleFalse(t *testing.T) {
	rec := httptest.NewRecorder()
	ts := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)

	WriteJSON(rec, 200, map[string]string{"hello": "world"}, ts, false)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("Content-Type = %q", ct)
	}

	var env struct {
		Data      map[string]string `json:"data"`
		UpdatedAt string            `json:"updated_at"`
		Stale     bool              `json:"stale"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Data["hello"] != "world" {
		t.Errorf("data.hello = %q, want %q", env.Data["hello"], "world")
	}
	if env.UpdatedAt != "2025-06-15T12:00:00Z" {
		t.Errorf("updated_at = %q, want %q", env.UpdatedAt, "2025-06-15T12:00:00Z")
	}
	if env.Stale {
		t.Errorf("stale = true, want false")
	}
}

func TestWriteJSONStaleTrue(t *testing.T) {
	rec := httptest.NewRecorder()
	ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)

	WriteJSON(rec, 200, "anything", ts, true)

	var env struct {
		Stale bool `json:"stale"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !env.Stale {
		t.Errorf("stale = false, want true")
	}
}

func TestWriteJSONZeroUpdatedAt(t *testing.T) {
	rec := httptest.NewRecorder()

	WriteJSON(rec, 200, 42, time.Time{}, false)

	var raw map[string]json.RawMessage
	if err := json.NewDecoder(rec.Body).Decode(&raw); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if string(raw["updated_at"]) != "null" {
		t.Errorf("updated_at = %s, want null", raw["updated_at"])
	}
}
