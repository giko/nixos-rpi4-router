package collector

import (
	"context"
	"log/slog"
	"time"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/model"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/poolhealth"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/wireguard"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/state"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/topology"
)

// ShowFunc executes `wg show <iface> dump` and returns the parsed result.
// Injectable for testing.
type ShowFunc func(ctx context.Context, iface string) (wireguard.Dump, error)

// TunnelsOpts configures the Tunnels collector.
type TunnelsOpts struct {
	Topology       *topology.Topology
	PoolHealthPath string
	State          *state.State
	Show           ShowFunc
}

// Tunnels collects WireGuard tunnel status by calling `wg show` for each
// tunnel in the topology, overlaying pool-health state, and publishing
// the result via State.SetTunnels.
type Tunnels struct {
	opts TunnelsOpts
}

// NewTunnels creates a Tunnels collector. If opts.Show is nil it defaults
// to wireguard.Show.
func NewTunnels(opts TunnelsOpts) *Tunnels {
	if opts.Show == nil {
		opts.Show = wireguard.Show
	}
	return &Tunnels{opts: opts}
}

func (*Tunnels) Name() string { return "tunnels" }
func (*Tunnels) Tier() Tier   { return Hot }

// Run performs a single collection pass.
func (t *Tunnels) Run(ctx context.Context) error {
	// Read pool-health; tolerate missing/unreadable file.
	ph, err := poolhealth.ReadState(t.opts.PoolHealthPath)
	if err != nil {
		slog.Warn("tunnels: pool-health read failed, assuming all healthy", "err", err)
		ph = poolhealth.State{Tunnels: make(map[string]poolhealth.TunnelInfo)}
	}

	tunnels := make([]model.Tunnel, 0, len(t.opts.Topology.Tunnels))
	now := time.Now().Unix()

	for _, tt := range t.opts.Topology.Tunnels {
		dump, err := t.opts.Show(ctx, tt.Interface)
		if err != nil {
			slog.Warn("tunnels: wg show failed", "iface", tt.Interface, "err", err)
			continue
		}

		mt := model.Tunnel{
			Name:         tt.Name,
			Interface:    tt.Interface,
			Fwmark:       tt.Fwmark,
			RoutingTable: tt.RoutingTable,
		}

		// Extract first peer's data.
		if len(dump.Peers) > 0 {
			p := dump.Peers[0]
			mt.PublicKey = p.PublicKey
			mt.Endpoint = p.Endpoint
			mt.RXBytes = p.RXBytes
			mt.TXBytes = p.TXBytes

			if p.LatestHandshakeUnix == 0 {
				mt.LatestHandshakeSecondsAgo = -1
			} else {
				mt.LatestHandshakeSecondsAgo = now - p.LatestHandshakeUnix
			}
		}

		// Overlay pool-health status.
		if hi, ok := ph.Tunnels[tt.Name]; ok {
			mt.Healthy = hi.Healthy
			mt.ConsecutiveFailures = hi.ConsecutiveFailures
		} else {
			// Not in pool-health file — assume healthy.
			mt.Healthy = true
		}

		tunnels = append(tunnels, mt)
	}

	t.opts.State.SetTunnels(tunnels)
	return nil
}
