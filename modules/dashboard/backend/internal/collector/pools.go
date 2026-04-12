package collector

import (
	"context"
	"log/slog"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/model"
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
// FlowCount is preserved from previous state (populated by cold-tier
// collector) but never overwritten here.
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
	// Read pool-health; tolerate missing/unreadable file.
	ph, err := poolhealth.ReadState(c.opts.PoolHealthPath)
	if err != nil {
		slog.Warn("pools: pool-health read failed, assuming all healthy", "err", err)
		ph = poolhealth.State{Tunnels: make(map[string]poolhealth.TunnelInfo)}
	}

	// Index tunnel fwmarks from topology for quick lookup.
	fwmarks := make(map[string]string, len(c.opts.Topology.Tunnels))
	for _, tt := range c.opts.Topology.Tunnels {
		fwmarks[tt.Name] = tt.Fwmark
	}

	// Snapshot existing pools to preserve per-member FlowCount from cold tier.
	prev, _ := c.opts.State.SnapshotPools()
	prevFlows := make(map[string]map[string]int, len(prev))
	for _, p := range prev {
		m := make(map[string]int, len(p.Members))
		for _, mem := range p.Members {
			m[mem.Tunnel] = mem.FlowCount
		}
		prevFlows[p.Name] = m
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

			mem := model.PoolMember{
				Tunnel:  name,
				Fwmark:  fwmarks[name],
				Healthy: healthy,
			}

			// Preserve flow count from previous snapshot.
			if flows, ok := prevFlows[tp.Name]; ok {
				mem.FlowCount = flows[name]
			}

			members = append(members, mem)
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

	c.opts.State.SetPools(pools)
	return nil
}
