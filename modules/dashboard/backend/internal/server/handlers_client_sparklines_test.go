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

func newSparklinesSrv(t *testing.T, lookup clientLookup, ct *collector.ClientTraffic, dr *collector.DnsRate, fc *collector.FlowCount) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/clients/{ip}/sparklines",
		NewClientSparklinesHandler(lookup, ct, dr, fc))
	return httptest.NewServer(mux)
}

func TestClientSparklinesDynamicAggregatesAllRings(t *testing.T) {
	client := netip.MustParseAddr("192.168.1.42")

	ct := collector.NewClientTraffic(collector.ClientTrafficOpts{TickDur: 10 * time.Second, RingSize: 4})
	ct.Track(client)
	t0 := time.Unix(0, 0)
	ct.Apply(t0, []conntrack.FlowBytes{{
		Key:        conntrack.FlowKey{OrigSrcIP: client},
		ClientIP:   client,
		Direction:  conntrack.DirOutbound,
		OrigBytes:  100,
		ReplyBytes: 200,
	}})
	ct.Apply(t0.Add(10*time.Second), []conntrack.FlowBytes{{
		Key:        conntrack.FlowKey{OrigSrcIP: client},
		ClientIP:   client,
		Direction:  conntrack.DirOutbound,
		OrigBytes:  1100,
		ReplyBytes: 1200,
	}})

	dr := collector.NewDnsRate(collector.DnsRateOpts{TickDur: 10 * time.Second})
	dr.Track(client)
	dr.Observe(client)
	dr.Tick(t0.Add(10 * time.Second))

	fc := collector.NewFlowCount(collector.FlowCountOpts{})
	fc.Track(client)
	fc.Apply(t0.Add(10*time.Second), []conntrack.FlowBytes{{ClientIP: client}})

	srv := newSparklinesSrv(t, fakeLookup{status: collector.LeaseStatusDynamic}, ct, dr, fc)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/clients/192.168.1.42/sparklines")
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
	var body model.ClientSparklines
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatal(err)
	}

	if body.ClientIP != "192.168.1.42" {
		t.Errorf("ClientIP = %q", body.ClientIP)
	}
	if body.LeaseStatus != "dynamic" {
		t.Errorf("LeaseStatus = %q", body.LeaseStatus)
	}
	if body.TickSeconds != 10 {
		t.Errorf("TickSeconds = %d", body.TickSeconds)
	}
	if len(body.Traffic) == 0 {
		t.Errorf("Traffic empty")
	}
	if len(body.DnsQps) == 0 {
		t.Errorf("DnsQps empty")
	}
	if len(body.FlowCount) == 0 {
		t.Errorf("FlowCount empty")
	}
}

func TestClientSparklinesNonDynamicReturnsNullRings(t *testing.T) {
	srv := newSparklinesSrv(t, fakeLookup{status: collector.LeaseStatusUnknown, staticOrNbr: true}, nil, nil, nil)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/clients/192.168.1.7/sparklines")
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
	var body model.ClientSparklines
	_ = json.Unmarshal(raw, &body)
	if body.LeaseStatus != "non-dynamic" {
		t.Errorf("LeaseStatus = %q", body.LeaseStatus)
	}
	if body.Traffic != nil || body.DnsQps != nil || body.FlowCount != nil {
		t.Errorf("expected all rings nil; got %+v", body)
	}
}

func TestClientSparklinesUnknownReturns404(t *testing.T) {
	srv := newSparklinesSrv(t, fakeLookup{status: collector.LeaseStatusUnknown}, nil, nil, nil)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/clients/192.168.1.250/sparklines")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}
