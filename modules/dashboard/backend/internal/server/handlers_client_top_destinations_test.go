package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/collector"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/envelope"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/model"
)

func newTopDestSrv(t *testing.T, lookup clientLookup, td *collector.TopDestinations) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/clients/{ip}/top-destinations", NewClientTopDestinationsHandler(lookup, td))
	return httptest.NewServer(mux)
}

func TestClientTopDestinationsDynamicReturnsRows(t *testing.T) {
	client := netip.MustParseAddr("192.168.1.42")
	td := collector.NewTopDestinations(collector.TopDestOpts{})
	td.Track(client)
	now := time.Now().UTC()
	td.RecordQuery(client, "ads.doubleclick.net", true, now)
	td.RecordQuery(client, "google.com", false, now)
	td.RecordBytes(client, "google.com", 50_000, now)

	srv := newTopDestSrv(t, fakeLookup{status: collector.LeaseStatusDynamic}, td)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/clients/192.168.1.42/top-destinations")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var env envelope.Response
	_ = json.NewDecoder(resp.Body).Decode(&env)
	raw, _ := json.Marshal(env.Data)
	var body model.ClientTopDestinations
	_ = json.Unmarshal(raw, &body)
	if body.WindowSeconds != 3600 {
		t.Errorf("WindowSeconds = %d", body.WindowSeconds)
	}
	if body.LeaseStatus != "dynamic" {
		t.Errorf("LeaseStatus = %q", body.LeaseStatus)
	}
	if body.Count == 0 || len(body.Destinations) == 0 {
		t.Fatalf("Destinations empty; body=%+v", body)
	}
}

func TestClientTopDestinationsNonDynamicReturnsNullList(t *testing.T) {
	td := collector.NewTopDestinations(collector.TopDestOpts{})
	srv := newTopDestSrv(t, fakeLookup{status: collector.LeaseStatusUnknown, staticOrNbr: true}, td)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/clients/192.168.1.7/top-destinations")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var env envelope.Response
	_ = json.NewDecoder(resp.Body).Decode(&env)
	raw, _ := json.Marshal(env.Data)
	var body model.ClientTopDestinations
	_ = json.Unmarshal(raw, &body)
	if body.LeaseStatus != "non-dynamic" {
		t.Errorf("LeaseStatus = %q", body.LeaseStatus)
	}
	if body.Destinations != nil {
		t.Errorf("Destinations should be nil for non-dynamic; got %v", body.Destinations)
	}
}

func TestClientTopDestinationsUnknownReturns404(t *testing.T) {
	td := collector.NewTopDestinations(collector.TopDestOpts{})
	srv := newTopDestSrv(t, fakeLookup{status: collector.LeaseStatusUnknown}, td)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/clients/192.168.1.250/top-destinations")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}
