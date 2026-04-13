package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/config"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/model"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/adguard"
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

func TestTunnelsHandler(t *testing.T) {
	st := state.New()
	st.SetTunnels([]model.Tunnel{
		{Name: "wg_sw", Healthy: true, Fwmark: "0x20000"},
	})

	h := New(&config.Config{}, st, &topology.Topology{})
	req := httptest.NewRequest(http.MethodGet, "/api/tunnels", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var env struct {
		Data struct {
			Tunnels []model.Tunnel `json:"tunnels"`
		} `json:"data"`
		UpdatedAt *string `json:"updated_at"`
		Stale     bool    `json:"stale"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(env.Data.Tunnels) != 1 {
		t.Fatalf("expected 1 tunnel, got %d", len(env.Data.Tunnels))
	}
	if env.Data.Tunnels[0].Name != "wg_sw" {
		t.Errorf("tunnel name = %q, want wg_sw", env.Data.Tunnels[0].Name)
	}
	if !env.Data.Tunnels[0].Healthy {
		t.Error("tunnel should be healthy")
	}
	if env.Data.Tunnels[0].Fwmark != "0x20000" {
		t.Errorf("tunnel fwmark = %q, want 0x20000", env.Data.Tunnels[0].Fwmark)
	}
	if env.UpdatedAt == nil {
		t.Error("updated_at should not be null")
	}
	if env.Stale {
		t.Error("stale should be false for fresh data")
	}
}

func TestPoolsHandler(t *testing.T) {
	st := state.New()
	st.SetPools([]model.Pool{
		{
			Name: "vpn_pool",
			Members: []model.PoolMember{
				{Tunnel: "wg_sw", Fwmark: "0x20000", Healthy: true, FlowCount: 0},
				{Tunnel: "wg_us", Fwmark: "0x30000", Healthy: false, FlowCount: 0},
			},
			ClientIPs:          []string{"192.168.1.10"},
			FailsafeDropActive: false,
		},
	})

	h := New(&config.Config{}, st, &topology.Topology{})
	req := httptest.NewRequest(http.MethodGet, "/api/pools", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var env struct {
		Data struct {
			Pools []model.Pool `json:"pools"`
		} `json:"data"`
		UpdatedAt *string `json:"updated_at"`
		Stale     bool    `json:"stale"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(env.Data.Pools) != 1 {
		t.Fatalf("expected 1 pool, got %d", len(env.Data.Pools))
	}
	p := env.Data.Pools[0]
	if p.Name != "vpn_pool" {
		t.Errorf("pool name = %q, want vpn_pool", p.Name)
	}
	if len(p.Members) != 2 {
		t.Fatalf("expected 2 members, got %d", len(p.Members))
	}
	if !p.Members[0].Healthy {
		t.Error("members[0] should be healthy")
	}
	if p.Members[1].Healthy {
		t.Error("members[1] should not be healthy")
	}
	if p.FailsafeDropActive {
		t.Error("failsafe_drop_active should be false")
	}
	if len(p.ClientIPs) != 1 || p.ClientIPs[0] != "192.168.1.10" {
		t.Errorf("client_ips = %v, want [192.168.1.10]", p.ClientIPs)
	}
	if env.UpdatedAt == nil {
		t.Error("updated_at should not be null")
	}
	if env.Stale {
		t.Error("stale should be false for fresh data")
	}
}

func TestClientsHandler(t *testing.T) {
	st := state.New()
	st.SetClients([]model.Client{
		{Hostname: "desktop", IP: "192.168.1.10", MAC: "aa:bb:cc:00:00:01", LeaseType: "static", Route: "pool:vpn_pool"},
		{Hostname: "phone", IP: "192.168.1.50", MAC: "aa:bb:cc:dd:ee:01", LeaseType: "dynamic", Route: "wan"},
	})

	h := New(&config.Config{}, st, &topology.Topology{})
	req := httptest.NewRequest(http.MethodGet, "/api/clients", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}

	var env struct {
		Data struct {
			Clients []model.Client `json:"clients"`
		} `json:"data"`
		UpdatedAt *string `json:"updated_at"`
		Stale     bool    `json:"stale"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(env.Data.Clients) != 2 {
		t.Fatalf("expected 2 clients, got %d", len(env.Data.Clients))
	}
	if env.Data.Clients[0].Hostname != "desktop" {
		t.Errorf("clients[0].Hostname = %q, want desktop", env.Data.Clients[0].Hostname)
	}
	if env.UpdatedAt == nil {
		t.Error("updated_at should not be null")
	}
	if env.Stale {
		t.Error("stale should be false for fresh data")
	}
}

func TestClientDetailHandler(t *testing.T) {
	st := state.New()
	st.SetClients([]model.Client{
		{Hostname: "desktop", IP: "192.168.1.10", MAC: "aa:bb:cc:00:00:01", LeaseType: "static"},
	})

	h := New(&config.Config{}, st, &topology.Topology{})

	// Found.
	req := httptest.NewRequest(http.MethodGet, "/api/clients/192.168.1.10", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("detail found: status = %d, want 200", rec.Code)
	}

	var env struct {
		Data      model.Client `json:"data"`
		UpdatedAt *string      `json:"updated_at"`
		Stale     bool         `json:"stale"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Data.IP != "192.168.1.10" {
		t.Errorf("IP = %q, want 192.168.1.10", env.Data.IP)
	}
	if env.Data.Hostname != "desktop" {
		t.Errorf("Hostname = %q, want desktop", env.Data.Hostname)
	}

	// Not found.
	req = httptest.NewRequest(http.MethodGet, "/api/clients/10.0.0.1", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("detail not found: status = %d, want 404", rec.Code)
	}

	var errEnv struct {
		Data struct {
			Error string `json:"error"`
			IP    string `json:"ip"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&errEnv); err != nil {
		t.Fatalf("decode 404: %v", err)
	}
	if errEnv.Data.Error != "client not found" {
		t.Errorf("error = %q, want 'client not found'", errEnv.Data.Error)
	}
	if errEnv.Data.IP != "10.0.0.1" {
		t.Errorf("ip = %q, want 10.0.0.1", errEnv.Data.IP)
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

func TestAdguardQueryLogCacheSingleflight(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		time.Sleep(30 * time.Millisecond)
		_, _ = w.Write([]byte(`{"data":[]}`))
	}))
	defer srv.Close()

	cache := newQueryLogCache(adguard.NewClient(srv.URL, srv.Client()))

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r := httptest.NewRequest("GET", "/api/adguard/querylog?limit=10", nil)
			_, err := cache.fetch(r)
			if err != nil {
				t.Errorf("fetch: %v", err)
			}
		}()
	}
	wg.Wait()

	if hits.Load() > 2 {
		t.Errorf("expected <=2 upstream hits (singleflight), got %d", hits.Load())
	}
}

func TestFirewallRulesHandler(t *testing.T) {
	st := state.New()
	st.SetFirewall(model.Firewall{
		PortForwards:          []model.PortForward{{Protocol: "tcp", ExternalPort: 35978, Destination: "192.168.20.6:32400"}},
		PBR:                   model.PBR{SourceRules: []model.PBRSourceRule{{Sources: []string{"192.168.1.225"}, Tunnel: "wg_sw"}}},
		AllowedMACs:           []string{"aa:bb:cc:dd:ee:ff"},
		BlockedForwardCount1h: 7,
	})
	h := New(&config.Config{}, st, &topology.Topology{})
	req := httptest.NewRequest(http.MethodGet, "/api/firewall/rules", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var env struct {
		Data struct {
			PortForwards          []map[string]any `json:"port_forwards"`
			PBR                   map[string]any   `json:"pbr"`
			AllowedMACs           []string         `json:"allowed_macs"`
			BlockedForwardCount1h float64          `json:"blocked_forward_count_1h"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(env.Data.PortForwards) != 1 || env.Data.PortForwards[0]["external_port"].(float64) != 35978 {
		t.Errorf("port_forwards = %+v", env.Data.PortForwards)
	}
	if env.Data.PBR == nil {
		t.Fatal("pbr missing")
	}
	if len(env.Data.AllowedMACs) != 1 || env.Data.AllowedMACs[0] != "aa:bb:cc:dd:ee:ff" {
		t.Errorf("allowed_macs = %+v", env.Data.AllowedMACs)
	}
	if env.Data.BlockedForwardCount1h != 7 {
		t.Errorf("blocked_forward_count_1h = %v, want 7", env.Data.BlockedForwardCount1h)
	}
}

func TestFirewallCountersHandler(t *testing.T) {
	st := state.New()
	st.SetFirewall(model.Firewall{
		Chains: []model.FirewallChain{{
			Family: "inet", Table: "filter", Name: "input", Hook: "input", Policy: "drop",
			Counters: []model.RuleCounter{{Handle: 16, Packets: 100, Bytes: 4096, Comment: "drop"}},
		}},
	})
	h := New(&config.Config{}, st, &topology.Topology{})
	req := httptest.NewRequest(http.MethodGet, "/api/firewall/counters", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var env struct {
		Data struct {
			Chains []struct {
				Name     string `json:"name"`
				Counters []struct {
					Handle  float64 `json:"handle"`
					Bytes   float64 `json:"bytes"`
					Comment string  `json:"comment"`
				} `json:"counters"`
			} `json:"chains"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(env.Data.Chains) != 1 || env.Data.Chains[0].Name != "input" {
		t.Errorf("chains = %+v", env.Data.Chains)
	}
	if len(env.Data.Chains[0].Counters) != 1 || env.Data.Chains[0].Counters[0].Bytes != 4096 {
		t.Errorf("counters = %+v", env.Data.Chains[0].Counters)
	}
}

func TestUPnPHandler(t *testing.T) {
	st := state.New()
	st.SetFirewall(model.Firewall{
		UPnPLeases: []model.UPnPLease{{Protocol: "tcp", ExternalPort: 35978, InternalAddr: "192.168.20.6", InternalPort: 32400, Description: "plex"}},
	})
	h := New(&config.Config{}, st, &topology.Topology{})
	req := httptest.NewRequest(http.MethodGet, "/api/upnp", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var env struct {
		Data struct {
			Leases []map[string]any `json:"leases"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(env.Data.Leases) != 1 || env.Data.Leases[0]["external_port"].(float64) != 35978 {
		t.Errorf("leases = %+v", env.Data.Leases)
	}
}

func TestQoSHandler(t *testing.T) {
	st := state.New()
	eg := model.QdiscStats{Kind: "cake", SentBytes: 1234, BandwidthBps: 100_000_000}
	st.SetQoS(model.QoS{Egress: &eg})
	h := New(&config.Config{}, st, &topology.Topology{})
	req := httptest.NewRequest(http.MethodGet, "/api/qos", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var env struct {
		Data struct {
			Egress *struct {
				Kind      string `json:"kind"`
				SentBytes int64  `json:"sent_bytes"`
			} `json:"wan_egress"`
			Ingress any `json:"wan_ingress"`
		} `json:"data"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Data.Egress == nil || env.Data.Egress.Kind != "cake" || env.Data.Egress.SentBytes != 1234 {
		t.Errorf("egress = %+v", env.Data.Egress)
	}
	if env.Data.Ingress != nil {
		t.Errorf("ingress should be nil when not collected, got %+v", env.Data.Ingress)
	}
}
