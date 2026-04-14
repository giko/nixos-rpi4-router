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

type fakeFlows struct{ list []conntrack.FlowBytes }

func (f fakeFlows) Snapshot() []conntrack.FlowBytes { return f.list }

type fakeDomains struct {
	hit map[string]string // remote-ip -> domain
}

func (f fakeDomains) Lookup(_, remote netip.Addr, _ time.Time) (string, bool) {
	d, ok := f.hit[remote.String()]
	return d, ok
}

func newConnsSrv(t *testing.T, lookup clientLookup, flows FlowSource, domains DomainLookup) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/clients/{ip}/connections", NewClientConnectionsHandler(lookup, flows, domains))
	return httptest.NewServer(mux)
}

func TestClientConnectionsOutboundEnriched(t *testing.T) {
	client := netip.MustParseAddr("192.168.1.42")
	remote := netip.MustParseAddr("142.250.190.78")
	flows := fakeFlows{list: []conntrack.FlowBytes{
		{
			Key:        conntrack.FlowKey{Proto: 6, OrigSrcIP: client, OrigDstIP: remote, OrigSrcPort: 51000, OrigDstPort: 443},
			ClientIP:   client,
			Direction:  conntrack.DirOutbound,
			OrigBytes:  500,
			ReplyBytes: 30000,
			RouteTag:   "WAN",
			LocalPort:  51000,
			RemoteIP:   remote,
			RemotePort: 443,
			State:      "ESTABLISHED",
		},
	}}
	domains := fakeDomains{hit: map[string]string{"142.250.190.78": "google.com"}}
	srv := newConnsSrv(t, fakeLookup{status: collector.LeaseStatusDynamic}, flows, domains)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/clients/192.168.1.42/connections")
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
	var body model.ClientConnections
	_ = json.Unmarshal(raw, &body)
	if body.Count != 1 {
		t.Fatalf("Count = %d", body.Count)
	}
	f := body.Flows[0]
	if f.Proto != "tcp" || f.Direction != "outbound" || f.LocalIP != "192.168.1.42" {
		t.Errorf("orientation wrong: %+v", f)
	}
	if f.Domain != "google.com" {
		t.Errorf("Domain = %q", f.Domain)
	}
	if f.ClientTxBytes != 500 || f.ClientRxBytes != 30000 {
		t.Errorf("tx/rx wrong: tx=%d rx=%d", f.ClientTxBytes, f.ClientRxBytes)
	}
	if f.RouteTag != "WAN" {
		t.Errorf("RouteTag = %q", f.RouteTag)
	}
	if f.NATPublicIP != "" {
		t.Errorf("NATPublicIP should be empty for outbound: %q", f.NATPublicIP)
	}
}

func TestClientConnectionsInboundDNAT(t *testing.T) {
	client := netip.MustParseAddr("192.168.1.50")
	remote := netip.MustParseAddr("203.0.113.5")
	natIP := netip.MustParseAddr("198.51.100.10")
	flows := fakeFlows{list: []conntrack.FlowBytes{
		{
			Key:           conntrack.FlowKey{Proto: 6, OrigSrcIP: remote, OrigDstIP: natIP, OrigSrcPort: 41000, OrigDstPort: 32400},
			ClientIP:      client,
			Direction:     conntrack.DirInbound,
			OrigBytes:     900,
			ReplyBytes:    1200,
			RouteTag:      "WAN",
			NATPublicIP:   natIP,
			NATPublicPort: 32400,
			LocalPort:     32400,
			RemoteIP:      remote,
			RemotePort:    41000,
			State:         "ESTABLISHED",
		},
	}}
	srv := newConnsSrv(t, fakeLookup{status: collector.LeaseStatusDynamic}, flows, fakeDomains{})
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/clients/192.168.1.50/connections")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var env envelope.Response
	_ = json.NewDecoder(resp.Body).Decode(&env)
	raw, _ := json.Marshal(env.Data)
	var body model.ClientConnections
	_ = json.Unmarshal(raw, &body)
	if len(body.Flows) != 1 {
		t.Fatalf("Flows count %d", len(body.Flows))
	}
	f := body.Flows[0]
	if f.Direction != "inbound" {
		t.Errorf("Direction = %q", f.Direction)
	}
	if f.NATPublicIP != "198.51.100.10" || f.NATPublicPort != 32400 {
		t.Errorf("NAT fields wrong: %+v", f)
	}
	if f.ClientRxBytes != 900 || f.ClientTxBytes != 1200 {
		t.Errorf("inbound tx/rx wrong: rx=%d tx=%d", f.ClientRxBytes, f.ClientTxBytes)
	}
}

func TestClientConnectionsMissingDomainStaysEmpty(t *testing.T) {
	client := netip.MustParseAddr("192.168.1.42")
	remote := netip.MustParseAddr("8.8.8.8")
	flows := fakeFlows{list: []conntrack.FlowBytes{{
		Key:       conntrack.FlowKey{Proto: 17, OrigSrcIP: client, OrigDstIP: remote, OrigSrcPort: 33333, OrigDstPort: 53},
		ClientIP:  client,
		Direction: conntrack.DirOutbound,
		LocalPort: 33333, RemoteIP: remote, RemotePort: 53, State: "ASSURED",
	}}}
	srv := newConnsSrv(t, fakeLookup{status: collector.LeaseStatusDynamic}, flows, fakeDomains{})
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/clients/192.168.1.42/connections")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var env envelope.Response
	_ = json.NewDecoder(resp.Body).Decode(&env)
	raw, _ := json.Marshal(env.Data)
	var body model.ClientConnections
	_ = json.Unmarshal(raw, &body)
	if body.Flows[0].Domain != "" {
		t.Errorf("Domain should be empty when enricher misses, got %q", body.Flows[0].Domain)
	}
}

func TestClientConnectionsNonDynamicStillReturnsFlows(t *testing.T) {
	client := netip.MustParseAddr("192.168.1.7")
	remote := netip.MustParseAddr("9.9.9.9")
	flows := fakeFlows{list: []conntrack.FlowBytes{{
		Key:       conntrack.FlowKey{Proto: 6, OrigSrcIP: client, OrigDstIP: remote, OrigSrcPort: 5000, OrigDstPort: 443},
		ClientIP:  client,
		Direction: conntrack.DirOutbound,
		LocalPort: 5000, RemoteIP: remote, RemotePort: 443, State: "ESTABLISHED",
	}}}
	srv := newConnsSrv(t, fakeLookup{status: collector.LeaseStatusUnknown, staticOrNbr: true}, flows, fakeDomains{})
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/clients/192.168.1.7/connections")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var env envelope.Response
	_ = json.NewDecoder(resp.Body).Decode(&env)
	raw, _ := json.Marshal(env.Data)
	var body model.ClientConnections
	_ = json.Unmarshal(raw, &body)
	if body.LeaseStatus != "non-dynamic" {
		t.Errorf("LeaseStatus = %q", body.LeaseStatus)
	}
	if body.Count != 1 {
		t.Errorf("Count = %d", body.Count)
	}
}
