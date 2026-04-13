package collector

import (
	"context"
	"log/slog"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/model"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/conntrack"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/poolhealth"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/state"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/topology"
)

// PoolsOpts configures the Pools collector.
type PoolsOpts struct {
	Topology       *topology.Topology
	PoolHealthPath string
	State          *state.State
}

// Pools derives pool state from the topology and pool-health file.
// FlowCount is preserved across updates because the hot pass writes via
// SetPoolsHot, which merges new topology/health data with existing
// cold-tier flow counts under the state mutex.
type Pools struct {
	opts PoolsOpts
}

// NewPools creates a Pools collector.
func NewPools(opts PoolsOpts) *Pools {
	return &Pools{opts: opts}
}

func (*Pools) Name() string { return "pools" }
func (*Pools) Tier() Tier   { return Hot }

// Run performs a single collection pass.
func (c *Pools) Run(_ context.Context) error {
	// Read pool-health. On failure preserve the previous snapshot —
	// substituting an empty map would default every member to
	// healthy=true and clear FailsafeDropActive, flipping the report to
	// "healthy" precisely when the health source is broken. Skipping the
	// write leaves the old values (and their updated_at) in place so the
	// handler's staleness check surfaces the error instead of reporting
	// fresh-but-wrong data.
	ph, err := poolhealth.ReadState(c.opts.PoolHealthPath)
	if err != nil {
		slog.Warn("pools: pool-health read failed, keeping previous pool state (will go stale)", "err", err)
		return nil
	}

	// Index tunnel fwmarks from topology for quick lookup.
	fwmarks := make(map[string]string, len(c.opts.Topology.Tunnels))
	for _, tt := range c.opts.Topology.Tunnels {
		fwmarks[tt.Name] = tt.Fwmark
	}

	pools := make([]model.Pool, 0, len(c.opts.Topology.Pools))
	for _, tp := range c.opts.Topology.Pools {
		members := make([]model.PoolMember, 0, len(tp.Members))
		allUnhealthy := true

		for _, name := range tp.Members {
			healthy := true
			if hi, ok := ph.Tunnels[name]; ok {
				healthy = hi.Healthy
			}
			if healthy {
				allUnhealthy = false
			}

			// FlowCount deliberately left zero here — SetPoolsHot merges
			// the previous cold-tier counts under the state mutex.
			members = append(members, model.PoolMember{
				Tunnel:  name,
				Fwmark:  fwmarks[name],
				Healthy: healthy,
			})
		}

		// failsafe_drop_active when ALL members are unhealthy.
		// An empty pool has no healthy member, so failsafe is active.
		failsafe := allUnhealthy

		clientIPs := c.opts.Topology.ClientIPsForPool(tp.Name)

		pools = append(pools, model.Pool{
			Name:               tp.Name,
			Members:            members,
			ClientIPs:          clientIPs,
			FailsafeDropActive: failsafe,
		})
	}

	c.opts.State.SetPoolsHot(pools)
	return nil
}

// --- PoolFlows (cold tier) ---

// PoolFlowsOpts configures the PoolFlows collector.
type PoolFlowsOpts struct {
	Topology *topology.Topology
	Run      conntrack.Runner // nil -> DefaultRunner
	State    *state.State
}

// PoolFlows is a cold-tier collector that counts per-tunnel active flows
// via conntrack and merges the counts into the existing pool state.
type PoolFlows struct {
	opts PoolFlowsOpts
}

// NewPoolFlows creates a PoolFlows collector.
func NewPoolFlows(opts PoolFlowsOpts) *PoolFlows {
	if opts.Run == nil {
		opts.Run = conntrack.DefaultRunner
	}
	return &PoolFlows{opts: opts}
}

func (*PoolFlows) Name() string { return "pool-flows" }
func (*PoolFlows) Tier() Tier   { return Cold }

// Run performs a single collection pass.
func (c *PoolFlows) Run(ctx context.Context) error {
	// Collect distinct fwmarks from the topology and count flows per tunnel.
	counts := make(map[string]int)
	seen := make(map[string]bool)

	for _, tt := range c.opts.Topology.Tunnels {
		if tt.Fwmark == "" || seen[tt.Fwmark] {
			continue
		}
		seen[tt.Fwmark] = true

		n, err := conntrack.CountByFwmark(ctx, c.opts.Run, tt.Fwmark)
		if err != nil {
			slog.Warn("pool-flows: count failed", "tunnel", tt.Name, "fwmark", tt.Fwmark, "err", err)
			continue
		}
		counts[tt.Name] = n
	}

	// Build per-pool flow maps keyed by tunnel name. A hot-tier snapshot
	// is only needed to discover which pools reference which tunnels; the
	// actual write happens under the state mutex via SetPoolFlows so a
	// concurrent hot-tier write can't race with our update.
	pools, _ := c.opts.State.SnapshotPools()
	perPool := make(map[string]map[string]int, len(pools))
	for _, p := range pools {
		m := make(map[string]int, len(p.Members))
		for _, mem := range p.Members {
			if n, ok := counts[mem.Tunnel]; ok {
				m[mem.Tunnel] = n
			}
		}
		if len(m) > 0 {
			perPool[p.Name] = m
		}
	}

	c.opts.State.SetPoolFlows(perPool)
	return nil
}
