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
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/conntrack"
)

func newTrafficSrv(t *testing.T, lookup clientLookup, ct *collector.ClientTraffic) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/clients/{ip}/traffic", NewClientTrafficHandler(lookup, ct))
	return httptest.NewServer(mux)
}

func TestClientTrafficDynamicReturnsRing(t *testing.T) {
	client := netip.MustParseAddr("192.168.1.42")
	ct := collector.NewClientTraffic(collector.ClientTrafficOpts{TickDur: 10 * time.Second, RingSize: 4})
	ct.Track(client)
	t0 := time.Unix(0, 0)
	ct.Apply(t0, []conntrack.FlowBytes{{
		Key:        conntrack.FlowKey{OrigSrcIP: client},
		ClientIP:   client,
		Direction:  conntrack.DirOutbound,
		OrigBytes:  1_000_000,
		ReplyBytes: 10_000_000,
	}})
	ct.Apply(t0.Add(10*time.Second), []conntrack.FlowBytes{{
		Key:        conntrack.FlowKey{OrigSrcIP: client},
		ClientIP:   client,
		Direction:  conntrack.DirOutbound,
		OrigBytes:  1_100_000,
		ReplyBytes: 12_000_000,
	}})

	srv := newTrafficSrv(t, fakeLookup{status: collector.LeaseStatusDynamic}, ct)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/clients/192.168.1.42/traffic")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var env envelope.Response
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(env.Data)
	var body model.ClientTraffic
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}
	if body.ClientIP != "192.168.1.42" {
		t.Errorf("ClientIP = %q", body.ClientIP)
	}
	if body.LeaseStatus != "dynamic" {
		t.Errorf("LeaseStatus = %q", body.LeaseStatus)
	}
	if len(body.Samples) == 0 {
		t.Errorf("Samples is empty")
	}
	if body.TickSeconds != 10 {
		t.Errorf("TickSeconds = %d", body.TickSeconds)
	}
}

func TestClientTrafficNonDynamicReturnsNullSamples(t *testing.T) {
	srv := newTrafficSrv(t, fakeLookup{status: collector.LeaseStatusUnknown, staticOrNbr: true}, collector.NewClientTraffic(collector.ClientTrafficOpts{}))
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/clients/192.168.1.99/traffic")
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
	var body model.ClientTraffic
	_ = json.Unmarshal(raw, &body)
	if body.LeaseStatus != "non-dynamic" {
		t.Errorf("LeaseStatus = %q", body.LeaseStatus)
	}
	if body.Samples != nil {
		t.Errorf("Samples should be nil, got %v", body.Samples)
	}
}

func TestClientTrafficUnknownReturns404(t *testing.T) {
	srv := newTrafficSrv(t, fakeLookup{status: collector.LeaseStatusUnknown}, collector.NewClientTraffic(collector.ClientTrafficOpts{}))
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/clients/192.168.1.250/traffic")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}
