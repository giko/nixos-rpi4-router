package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/collector"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/config"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/server"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/ipneigh"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/state"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/topology"
)

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

	// Build interface list: physical + tunnel interfaces.
	ifaces := []string{"eth0", "eth1"}
	for _, tun := range topo.Tunnels {
		ifaces = append(ifaces, tun.Interface)
	}

	collectors := []collector.Collector{
		collector.NewTraffic(collector.TrafficOpts{
			NetDevPath: "/proc/net/dev",
			Interfaces: ifaces,
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
		collector.NewClientFwmarks(collector.ClientFwmarksOpts{State: st}),
		collector.NewPoolFlows(collector.PoolFlowsOpts{Topology: topo, State: st}),
		collector.NewClients(collector.ClientsOpts{
			Topology:   topo,
			LeasesPath: "/var/lib/dnsmasq/dnsmasq.leases",
			State:      st,
			Neigh: func(ctx context.Context) (map[string]string, error) {
				return ipneigh.Collect(ctx, ipneigh.DefaultRunner)
			},
		}),
		collector.NewSystemMedium(collector.SystemMediumOpts{
			Units: []string{
				"nftables.service", "dnsmasq.service", "adguardhome.service",
				"wg-pool-health.service", "flow-offload.service",
				"cake-qos.service", "chronyd.service", "policy-routing.service",
			},
			State: st,
		}),
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	runner := collector.NewRunner(logger, collectors)
	runner.Start(ctx)

	httpServer := &http.Server{
		Addr:              cfg.Bind,
		Handler:           server.New(cfg, st, topo),
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
