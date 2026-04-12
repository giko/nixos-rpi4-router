package collector

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/adguard"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/state"
)

const adguardStatsFixture = `{
  "num_dns_queries": 10000,
  "num_blocked_filtering": 2500,
  "top_blocked_domains": [
    {"ads.example.com": 500},
    {"tracker.example.net": 300}
  ],
  "top_queried_domains": [
    {"google.com": 2000}
  ],
  "top_clients": [
    {"192.168.1.10": 5000},
    {"192.168.1.20": 3000}
  ],
  "dns_queries": [100, 200, 150],
  "blocked_filtering": [10, 20, 15]
}`

func TestAdguardStatsCollector(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/control/stats" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(adguardStatsFixture))
	}))
	defer srv.Close()

	st := state.New()
	c := NewAdguardStats(AdguardStatsOpts{
		Client: adguard.NewClient(srv.URL, srv.Client()),
		State:  st,
	})

	if c.Name() != "adguard-stats" {
		t.Errorf("Name() = %q, want adguard-stats", c.Name())
	}
	if c.Tier() != Medium {
		t.Errorf("Tier() = %v, want Medium", c.Tier())
	}

	if err := c.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	snap, updated := st.SnapshotAdguard()
	if updated.IsZero() {
		t.Fatal("updated_at is zero after Run")
	}

	if snap.Queries24h != 10000 {
		t.Errorf("Queries24h = %d, want 10000", snap.Queries24h)
	}
	if snap.Blocked24h != 2500 {
		t.Errorf("Blocked24h = %d, want 2500", snap.Blocked24h)
	}

	wantRate := 25.0
	if math.Abs(snap.BlockRate-wantRate) > 0.01 {
		t.Errorf("BlockRate = %f, want %f", snap.BlockRate, wantRate)
	}

	if len(snap.TopBlocked) != 2 {
		t.Fatalf("TopBlocked len = %d, want 2", len(snap.TopBlocked))
	}
	if snap.TopBlocked[0].Domain != "ads.example.com" || snap.TopBlocked[0].Count != 500 {
		t.Errorf("TopBlocked[0] = %+v, want ads.example.com:500", snap.TopBlocked[0])
	}

	if len(snap.TopClients) != 2 {
		t.Fatalf("TopClients len = %d, want 2", len(snap.TopClients))
	}
	if snap.TopClients[0].IP != "192.168.1.10" || snap.TopClients[0].Count != 5000 {
		t.Errorf("TopClients[0] = %+v, want 192.168.1.10:5000", snap.TopClients[0])
	}

	if len(snap.QueryDensity24h) != 3 {
		t.Fatalf("QueryDensity24h len = %d, want 3", len(snap.QueryDensity24h))
	}
	if snap.QueryDensity24h[0].Queries != 100 || snap.QueryDensity24h[0].Blocked != 10 {
		t.Errorf("QueryDensity24h[0] = %+v, want {0, 100, 10}", snap.QueryDensity24h[0])
	}
}

func TestAdguardStatsCollectorHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	st := state.New()
	c := NewAdguardStats(AdguardStatsOpts{
		Client: adguard.NewClient(srv.URL, srv.Client()),
		State:  st,
	})

	err := c.Run(context.Background())
	if err == nil {
		t.Fatal("expected error for HTTP 500, got nil")
	}

	// State should not be updated on error.
	_, updated := st.SnapshotAdguard()
	if !updated.IsZero() {
		t.Error("state should not be updated after error")
	}
}
