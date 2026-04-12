package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/config"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/model"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/state"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/topology"
)

func TestTrafficHandlerReturnsStateSnapshot(t *testing.T) {
	st := state.New()
	st.SetTraffic(model.Traffic{
		Interfaces: []model.Interface{
			{
				Name:         "eth0",
				RXBps:        8000,
				TXBps:        4000,
				RXBytesTotal: 100000,
				TXBytesTotal: 50000,
			},
		},
	})

	h := New(&config.Config{}, st, &topology.Topology{})
	req := httptest.NewRequest(http.MethodGet, "/api/traffic", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var env struct {
		Data      model.Traffic `json:"data"`
		UpdatedAt *string       `json:"updated_at"`
		Stale     bool          `json:"stale"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(env.Data.Interfaces) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(env.Data.Interfaces))
	}
	if env.Data.Interfaces[0].Name != "eth0" {
		t.Errorf("interface name = %q, want eth0", env.Data.Interfaces[0].Name)
	}
	if env.Data.Interfaces[0].RXBps != 8000 {
		t.Errorf("RXBps = %d, want 8000", env.Data.Interfaces[0].RXBps)
	}
	if env.UpdatedAt == nil {
		t.Error("updated_at should not be null")
	}
	if env.Stale {
		t.Error("stale should be false for fresh data")
	}
}

func TestSystemHandlerReturnsStateSnapshot(t *testing.T) {
	st := state.New()
	st.SetSystem(model.SystemStats{
		CPU: model.CPUStats{
			PercentUser:   25.0,
			PercentSystem: 10.0,
			PercentIdle:   60.0,
			PercentIOWait: 5.0,
		},
		TemperatureC:  52.3,
		UptimeSeconds: 12345.67,
	})

	h := New(&config.Config{}, st, &topology.Topology{})
	req := httptest.NewRequest(http.MethodGet, "/api/system", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var env struct {
		Data      model.SystemStats `json:"data"`
		UpdatedAt *string           `json:"updated_at"`
		Stale     bool              `json:"stale"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Data.CPU.PercentUser != 25.0 {
		t.Errorf("PercentUser = %f, want 25.0", env.Data.CPU.PercentUser)
	}
	if env.Data.TemperatureC != 52.3 {
		t.Errorf("TemperatureC = %f, want 52.3", env.Data.TemperatureC)
	}
	if env.UpdatedAt == nil {
		t.Error("updated_at should not be null")
	}
	if env.Stale {
		t.Error("stale should be false for fresh data")
	}
}

func TestTrafficHandlerStaleWhenNeverUpdated(t *testing.T) {
	st := state.New()
	// Never call SetTraffic -- data is zero-time, should be stale.

	h := New(&config.Config{}, st, &topology.Topology{})
	req := httptest.NewRequest(http.MethodGet, "/api/traffic", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	var env struct {
		Stale bool `json:"stale"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !env.Stale {
		t.Error("stale should be true when data was never updated")
	}
}
