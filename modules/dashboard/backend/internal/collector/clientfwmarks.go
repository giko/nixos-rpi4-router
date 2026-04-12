package collector

import (
	"context"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/conntrack"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/state"
)

// ClientConnsOpts configures the ClientConns collector.
type ClientConnsOpts struct {
	Run   conntrack.Runner // nil -> DefaultRunner
	State *state.State
}

// ClientConns is a cold-tier collector that caches per-source-IP
// connection info from conntrack. The Clients collector reads this
// to populate per-client connection counts and tunnel distribution.
type ClientConns struct {
	opts ClientConnsOpts
}

// NewClientConns creates a ClientConns collector.
func NewClientConns(opts ClientConnsOpts) *ClientConns {
	if opts.Run == nil {
		opts.Run = conntrack.DefaultRunner
	}
	return &ClientConns{opts: opts}
}

func (*ClientConns) Name() string { return "client-conns" }
func (*ClientConns) Tier() Tier   { return Cold }

// Run performs a single collection pass.
func (c *ClientConns) Run(ctx context.Context) error {
	m, err := conntrack.ClientConnections(ctx, c.opts.Run)
	if err != nil {
		return err
	}
	c.opts.State.SetClientConns(m)
	return nil
}
