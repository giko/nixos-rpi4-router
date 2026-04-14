package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/netip"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/collector"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/config"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/server"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/adguard"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/dnsmasq"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/ipneigh"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/state"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/topology"
)

// ifbForWAN returns the IFB device name conventionally created by the
// QoS module for ingress shaping on the given WAN interface. Returns
// "" when the WAN interface is unset (dev mode), which the QoS
// collector treats as "skip ingress".
func ifbForWAN(wan string) string {
	if wan == "" {
		return ""
	}
	return "ifb4" + wan
}

// clientLookupAdapter satisfies server.clientLookup by combining the
// LifecycleTracker (dynamic/expired) with the state-backed client list
// (static/neighbor). Kept here — not in package server — because it
// depends on both state.State and the client-detail runtime wiring.
type clientLookupAdapter struct {
	state   *state.State
	runtime *collector.ClientDetailRuntime
}

// Status returns the lifecycle tracker's view (dynamic / expired /
// unknown). Non-dynamic is resolved by the server helper via
// IsStaticOrNeighbor.
func (a *clientLookupAdapter) Status(ip netip.Addr) collector.LeaseStatus {
	return a.runtime.Lifecycle.Status(ip)
}

// IsStaticOrNeighbor walks the latest clients snapshot for a matching
// IP whose lease_type indicates a non-dynamic origin.
func (a *clientLookupAdapter) IsStaticOrNeighbor(ip netip.Addr) bool {
	target := ip.String()
	clients, _ := a.state.SnapshotClients()
	for _, c := range clients {
		if c.IP != target {
			continue
		}
		switch c.LeaseType {
		case "static", "neighbor":
			return true
		}
	}
	return false
}

// buildRouteTags composes the numeric-mark → tunnel-name table used for
// conntrack flow attribution. Always includes the implicit WAN mark
// (0x10000); every tunnel with a parseable Fwmark string contributes an
// entry keyed by its numeric value.
func buildRouteTags(topo *topology.Topology) map[uint32]string {
	tags := map[uint32]string{0x10000: "WAN"}
	for _, tun := range topo.Tunnels {
		n, err := strconv.ParseUint(tun.Fwmark, 0, 32)
		if err != nil {
			continue
		}
		tags[uint32(n)] = tun.Name
	}
	return tags
}

func main() {
	cfg, err := config.FromFlags(os.Args[1:])
	if err != nil {
		// No logger yet — cfg hasn't told us what level to use. Write to stderr.
		fmt.Fprintf(os.Stderr, "dashboard: config error: %v\n", err)
		os.Exit(1)
	}

	// Honor the configured log level. slog.Level.UnmarshalText accepts
	// DEBUG / INFO / WARN / ERROR (uppercase); we ToUpper cfg.LogLevel so
	// the user-facing lowercase form ("debug", "info", ...) works. Invalid
	// values emit a warning and fall back to INFO rather than crash — a
	// typo in a config file should not take down the dashboard.
	var level slog.Level
	if uerr := level.UnmarshalText([]byte(strings.ToUpper(cfg.LogLevel))); uerr != nil {
		fmt.Fprintf(os.Stderr, "dashboard: invalid --log-level %q, defaulting to info\n", cfg.LogLevel)
		level = slog.LevelInfo
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	topo, err := topology.Load(cfg.ConfigFile)
	if err != nil {
		slog.Error("load topology", "err", err)
		os.Exit(1)
	}

	st := state.New()

	// Build the interface list and role map from the topology. A missing
	// LAN/WAN name (dev mode with no topology.json) falls back to the
	// historical "eth0"/"eth1" defaults so local runs keep working.
	lanIf := topo.LANInterface
	if lanIf == "" {
		lanIf = "eth0"
	}
	wanIf := topo.WANInterface
	if wanIf == "" {
		wanIf = "eth1"
	}
	ifaces := []string{lanIf, wanIf}
	roles := map[string]string{
		lanIf: "lan",
		wanIf: "wan",
	}
	for _, tun := range topo.Tunnels {
		ifaces = append(ifaces, tun.Interface)
		roles[tun.Interface] = "tunnel"
	}

	// Shared AdGuard client for stats + per-client query log + ingest.
	// Sharing the underlying *http.Client keeps connection reuse sane.
	adguardClient := adguard.NewClient(cfg.AdguardURL, nil)

	// LAN prefixes for direction attribution. TODO: revisit when the
	// topology gains a lan_subnets field — today the router serves two
	// subnets on the LAN port and both are statically known.
	lanPrefixes := []netip.Prefix{
		netip.MustParsePrefix("192.168.1.0/24"),
		netip.MustParsePrefix("192.168.20.0/24"),
	}

	runtime := collector.NewClientDetailRuntime(collector.ClientDetailOpts{
		LANPrefixes:     lanPrefixes,
		RouteTags:       buildRouteTags(topo),
		ConntrackReader: collector.DefaultConntrackReader(),
		AdguardIngest:   adguardClient,
	})

	collectors := []collector.Collector{
		collector.NewTraffic(collector.TrafficOpts{
			NetDevPath: "/proc/net/dev",
			Interfaces: ifaces,
			Roles:      roles,
			State:      st,
		}),
		collector.NewSystem(collector.SystemOpts{
			StatPath:    "/proc/stat",
			MeminfoPath: "/proc/meminfo",
			ThermalPath: "/sys/class/thermal/thermal_zone0/temp",
			UptimePath:  "/proc/uptime",
			State:       st,
		}),
		collector.NewTunnels(collector.TunnelsOpts{
			Topology:       topo,
			PoolHealthPath: "/run/wg-pool-health/state.json",
			State:          st,
		}),
		collector.NewPools(collector.PoolsOpts{
			Topology:       topo,
			PoolHealthPath: "/run/wg-pool-health/state.json",
			State:          st,
		}),
		collector.NewClientConns(collector.ClientConnsOpts{State: st}),
		collector.NewPoolFlows(collector.PoolFlowsOpts{Topology: topo, State: st}),
		collector.NewClients(collector.ClientsOpts{
			Topology:   topo,
			LeasesPath: "/var/lib/dnsmasq/dnsmasq.leases",
			State:      st,
			Neigh: func(ctx context.Context) (map[string]ipneigh.Entry, error) {
				return ipneigh.Collect(ctx, ipneigh.DefaultRunner)
			},
		}),
		collector.NewAdguardStats(collector.AdguardStatsOpts{
			Client: adguardClient,
			State:  st,
		}),
		collector.NewSystemMedium(collector.SystemMediumOpts{
			Units: []string{
				"nftables.service", "dnsmasq.service", "adguardhome.service",
				"wg-pool-health.service", "flow-offload.service",
				"cake-qos.service", "chronyd.service", "policy-routing.service",
			},
			State: st,
		}),
		collector.NewFirewall(collector.FirewallOpts{State: st, Topology: topo}),
		collector.NewQoS(collector.QoSOpts{
			State:            st,
			EgressInterface:  topo.WANInterface,
			IngressInterface: ifbForWAN(topo.WANInterface),
		}),
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	runner := collector.NewRunner(logger, collectors)
	runner.Start(ctx)

	// Client-detail runtime: 10s hot tick drives lease scan, conntrack
	// Tick, and DNS ingest; 1-minute tick reaps tombstoned clients.
	go func() {
		t := time.NewTicker(10 * time.Second)
		defer t.Stop()
		leasesPath := "/var/lib/dnsmasq/dnsmasq.leases"
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-t.C:
				leases, err := dnsmasq.ReadLeases(leasesPath)
				if err != nil {
					slog.Warn("client-detail: read leases", "err", err)
					continue
				}
				ips := make([]netip.Addr, 0, len(leases))
				for _, l := range leases {
					if a, perr := netip.ParseAddr(l.IP); perr == nil {
						ips = append(ips, a)
					}
				}
				runtime.OnLeaseScan(ips, now)
				if err := runtime.Tick(now); err != nil {
					slog.Warn("client-detail: conntrack tick", "err", err)
				}
				if err := runtime.IngestTick(ctx, now); err != nil {
					slog.Warn("client-detail: dns ingest", "err", err)
				}
			}
		}
	}()

	go func() {
		t := time.NewTicker(time.Minute)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-t.C:
				if reaped := runtime.Reap(now); len(reaped) > 0 {
					slog.Info("client-detail: reaped tombstoned clients", "count", len(reaped))
				}
			}
		}
	}()

	httpServer := &http.Server{
		Addr: cfg.Bind,
		Handler: server.NewWithDeps(cfg, st, topo, server.Deps{
			ClientLookup:    &clientLookupAdapter{state: st, runtime: runtime},
			ClientTraffic:   runtime.Traffic,
			AdguardQueryLog: adguardClient,
			Flows:           runtime,
			Domains:         runtime.Domains,
			TopDestinations: runtime.TopDestinations,
			DnsRate:         runtime.DnsRate,
			FlowCount:       runtime.FlowCount,
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("dashboard starting", "bind", cfg.Bind)
		errCh <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("listen", "err", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		slog.Info("dashboard stopping")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			slog.Error("shutdown", "err", err)
			os.Exit(1)
		}
	}

	runner.Wait()
}
