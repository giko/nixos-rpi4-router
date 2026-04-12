package collector

import (
	"context"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/model"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/adguard"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/state"
)

// AdguardStatsOpts configures the AdguardStats collector.
type AdguardStatsOpts struct {
	Client *adguard.Client
	State  *state.State
}

// AdguardStats fetches DNS statistics from AdGuard Home and publishes
// them via State.SetAdguard.
type AdguardStats struct {
	opts AdguardStatsOpts
}

// NewAdguardStats creates an AdguardStats collector.
func NewAdguardStats(opts AdguardStatsOpts) *AdguardStats {
	return &AdguardStats{opts: opts}
}

func (*AdguardStats) Name() string { return "adguard-stats" }
func (*AdguardStats) Tier() Tier   { return Medium }

// Run performs a single collection pass.
func (c *AdguardStats) Run(ctx context.Context) error {
	raw, err := c.opts.Client.FetchStats(ctx)
	if err != nil {
		return err
	}

	var blockRate float64
	if raw.NumDNSQueries > 0 {
		blockRate = float64(raw.NumBlocked) / float64(raw.NumDNSQueries) * 100
	}

	topBlocked := make([]model.TopDomain, len(raw.TopBlocked))
	for i, d := range raw.TopBlocked {
		topBlocked[i] = model.TopDomain{Domain: d.Domain, Count: d.Count}
	}

	topClients := make([]model.TopClient, len(raw.TopClients))
	for i, cl := range raw.TopClients {
		topClients[i] = model.TopClient{IP: cl.IP, Count: cl.Count}
	}

	density := make([]model.DensityBin, len(raw.Density))
	for i, b := range raw.Density {
		density[i] = model.DensityBin{
			StartHour: b.StartHour,
			Queries:   b.Queries,
			Blocked:   b.Blocked,
		}
	}

	c.opts.State.SetAdguard(model.AdguardStats{
		Queries24h:      raw.NumDNSQueries,
		Blocked24h:      raw.NumBlocked,
		BlockRate:       blockRate,
		TopBlocked:      topBlocked,
		TopClients:      topClients,
		QueryDensity24h: density,
	})

	return nil
}
