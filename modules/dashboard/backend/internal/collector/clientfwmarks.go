package collector

import (
	"context"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/conntrack"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/state"
)

// ClientFwmarksOpts configures the ClientFwmarks collector.
type ClientFwmarksOpts struct {
	Run   conntrack.Runner // nil -> DefaultRunner
	State *state.State
}

// ClientFwmarks is a cold-tier collector that caches source-IP to fwmark
// mappings from conntrack. Other collectors (e.g. Clients) can read the
// cached map via State.SnapshotClientFwmarks.
type ClientFwmarks struct {
	opts ClientFwmarksOpts
}

// NewClientFwmarks creates a ClientFwmarks collector.
func NewClientFwmarks(opts ClientFwmarksOpts) *ClientFwmarks {
	if opts.Run == nil {
		opts.Run = conntrack.DefaultRunner
	}
	return &ClientFwmarks{opts: opts}
}

func (*ClientFwmarks) Name() string { return "client-fwmarks" }
func (*ClientFwmarks) Tier() Tier   { return Cold }

// Run performs a single collection pass.
func (c *ClientFwmarks) Run(ctx context.Context) error {
	m, err := conntrack.ClientFwmarks(ctx, c.opts.Run)
	if err != nil {
		return err
	}
	c.opts.State.SetClientFwmarks(m)
	return nil
}
