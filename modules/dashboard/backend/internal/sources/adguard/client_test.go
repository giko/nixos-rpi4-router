package adguard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

const statsFixture = `{
  "num_dns_queries": 54321,
  "num_blocked_filtering": 1234,
  "top_blocked_domains": [
    {"ads.example.com": 500},
    {"tracker.example.net": 300}
  ],
  "top_queried_domains": [
    {"google.com": 2000},
    {"github.com": 1500}
  ],
  "top_clients": [
    {"192.168.1.10": 10000},
    {"192.168.1.20": 8000}
  ],
  "dns_queries": [100, 200, 150, 180],
  "blocked_filtering": [10, 20, 15, 18]
}`

func TestFetchStats(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/control/stats" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(statsFixture))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, srv.Client())

	stats, err := c.FetchStats(context.Background())
	if err != nil {
		t.Fatalf("FetchStats: %v", err)
	}

	if stats.NumDNSQueries != 54321 {
		t.Errorf("NumDNSQueries = %d, want 54321", stats.NumDNSQueries)
	}
	if stats.NumBlocked != 1234 {
		t.Errorf("NumBlocked = %d, want 1234", stats.NumBlocked)
	}

	// TopBlocked
	if len(stats.TopBlocked) != 2 {
		t.Fatalf("TopBlocked len = %d, want 2", len(stats.TopBlocked))
	}
	if stats.TopBlocked[0].Domain != "ads.example.com" || stats.TopBlocked[0].Count != 500 {
		t.Errorf("TopBlocked[0] = %+v, want ads.example.com:500", stats.TopBlocked[0])
	}

	// TopQueried
	if len(stats.TopQueried) != 2 {
		t.Fatalf("TopQueried len = %d, want 2", len(stats.TopQueried))
	}

	// TopClients
	if len(stats.TopClients) != 2 {
		t.Fatalf("TopClients len = %d, want 2", len(stats.TopClients))
	}
	if stats.TopClients[0].IP != "192.168.1.10" || stats.TopClients[0].Count != 10000 {
		t.Errorf("TopClients[0] = %+v, want 192.168.1.10:10000", stats.TopClients[0])
	}

	// Density
	if len(stats.Density) != 4 {
		t.Fatalf("Density len = %d, want 4", len(stats.Density))
	}
	if stats.Density[0].StartHour != 0 || stats.Density[0].Queries != 100 || stats.Density[0].Blocked != 10 {
		t.Errorf("Density[0] = %+v, want {0, 100, 10}", stats.Density[0])
	}
	if stats.Density[3].Queries != 180 || stats.Density[3].Blocked != 18 {
		t.Errorf("Density[3] = %+v, want {3, 180, 18}", stats.Density[3])
	}
}

func TestFetchStatsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, srv.Client())

	_, err := c.FetchStats(context.Background())
	if err == nil {
		t.Fatal("expected error for HTTP 500, got nil")
	}
}

func TestFetchQueryLog(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/control/querylog" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}

		// Verify query parameters.
		if got := r.URL.Query().Get("limit"); got != "5" {
			t.Errorf("limit = %q, want 5", got)
		}
		if got := r.URL.Query().Get("search"); got != "192.168.1.10 example.com" {
			t.Errorf("search = %q, want '192.168.1.10 example.com'", got)
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"data": [{"question": "example.com"}], "oldest": ""}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, srv.Client())

	data, err := c.FetchQueryLog(context.Background(), QueryLogOptions{
		Limit:  5,
		Client: "192.168.1.10",
		Domain: "example.com",
	})
	if err != nil {
		t.Fatalf("FetchQueryLog: %v", err)
	}

	var entries []map[string]string
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatalf("unmarshal data: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries len = %d, want 1", len(entries))
	}
	if entries[0]["question"] != "example.com" {
		t.Errorf("question = %q, want example.com", entries[0]["question"])
	}
}

func TestBuildSearch(t *testing.T) {
	tests := []struct {
		client, domain, want string
	}{
		{"", "", ""},
		{"1.2.3.4", "", "1.2.3.4"},
		{"", "example.com", "example.com"},
		{"1.2.3.4", "example.com", "1.2.3.4 example.com"},
	}
	for _, tt := range tests {
		got := buildSearch(tt.client, tt.domain)
		if got != tt.want {
			t.Errorf("buildSearch(%q, %q) = %q, want %q", tt.client, tt.domain, got, tt.want)
		}
	}
}
