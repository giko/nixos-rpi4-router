package collector

import (
	"context"
	"sync"
	"time"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/model"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/nft"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/state"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/topology"
)

// FirewallOpts configures the Firewall collector.
type FirewallOpts struct {
	State    *state.State
	Topology *topology.Topology // static config source for port_forwards / pbr / allowed_macs
	Run      nft.Runner         // nil → DefaultRunner
	Clock    func() time.Time   // nil → time.Now
}

// Firewall is a medium-tier collector that runs `nft --json list ruleset`,
// projects the parse into model.Firewall (chains-with-nested-counters +
// UPnP leases), merges the static topology config (port forwards / PBR /
// allowlist), and rolls the 1h blocked-forward counter forward via an
// in-memory ring.
type Firewall struct {
	opts FirewallOpts

	mu      sync.Mutex
	samples []forwardDropSample // trimmed to 1h window on each Run
}

type forwardDropSample struct {
	ts    time.Time
	count int64
}

// NewFirewall creates a Firewall collector.
func NewFirewall(opts FirewallOpts) *Firewall {
	if opts.Run == nil {
		opts.Run = nft.DefaultRunner
	}
	if opts.Clock == nil {
		opts.Clock = time.Now
	}
	return &Firewall{opts: opts}
}

func (*Firewall) Name() string { return "firewall" }
func (*Firewall) Tier() Tier   { return Medium }

func (c *Firewall) Run(ctx context.Context) error {
	r, err := nft.Collect(ctx, c.opts.Run)
	if err != nil {
		return err
	}

	// Index counters by (family,table,chain,handle) so we can nest
	// them under their owning chain.
	type key struct {
		family, table, chain string
		handle               int
	}
	countersByRule := map[key][]model.RuleCounter{}
	forwardDropTotal := int64(0)
	for _, ct := range r.Counters {
		k := key{ct.Family, ct.Table, ct.ChainName, ct.Handle}
		countersByRule[k] = append(countersByRule[k], model.RuleCounter{
			Handle: ct.Handle, Comment: ct.Comment,
			Packets: ct.Packets, Bytes: ct.Bytes,
		})
		// Only "drop" rules count toward "blocked" forwards. A
		// `counter accept` or `counter return` rule's hits are
		// not blocked traffic.
		if ct.Family == "inet" && ct.Table == "filter" && ct.ChainName == "forward" && ct.Verdict == "drop" {
			forwardDropTotal += ct.Packets
		}
	}

	// Append-or-prune the forward-drop ring under lock. Oldest sample
	// at-or-before (now - 1h) is our baseline; 1h delta is current total
	// minus that baseline's recorded count.
	now := c.opts.Clock()
	oneHourAgo := now.Add(-time.Hour)
	c.mu.Lock()
	c.samples = append(c.samples, forwardDropSample{ts: now, count: forwardDropTotal})
	// Keep at most one sample older than 1h (our baseline).
	var trimmed []forwardDropSample
	var baseline *forwardDropSample
	for i := range c.samples {
		s := c.samples[i]
		if s.ts.Before(oneHourAgo) {
			b := s
			baseline = &b
			continue
		}
		trimmed = append(trimmed, s)
	}
	if baseline != nil {
		trimmed = append([]forwardDropSample{*baseline}, trimmed...)
	}
	c.samples = trimmed
	// Only publish a non-zero 1h delta once we actually have ≥1h of
	// samples; otherwise the value would represent "since startup"
	// rather than "in the last 1h". The oldest sample's age must be
	// >= 1h before this metric is meaningful.
	var delta int64
	if len(c.samples) > 1 {
		oldest := c.samples[0]
		if !oldest.ts.After(oneHourAgo) {
			delta = c.samples[len(c.samples)-1].count - oldest.count
			if delta < 0 {
				delta = 0 // counter reset (nftables reload) — start over.
				c.samples = []forwardDropSample{c.samples[len(c.samples)-1]}
			}
		}
	}
	c.mu.Unlock()

	// Init every container slice as a non-nil empty slice so JSON
	// serialization always emits `[]` rather than `null`. The frontend
	// types declare these as arrays; nulls would crash `.length` /
	// `.map` calls on the consuming pages.
	out := model.Firewall{
		BlockedForwardCount1h: delta,
		PortForwards:          []model.PortForward{},
		PBR: model.PBR{
			SourceRules:       []model.PBRSourceRule{},
			DomainRules:       []model.PBRDomainRule{},
			SourceDomainRules: []model.PBRSourceDomainRule{},
			PooledRules:       []model.PBRPooledRule{},
		},
		AllowedMACs: []string{},
		Chains:      make([]model.FirewallChain, 0, len(r.Chains)),
		UPnPLeases:  make([]model.UPnPLease, 0, len(r.UPnPMappings)),
	}

	// Merge topology-sourced static fields.
	if topo := c.opts.Topology; topo != nil {
		if len(topo.AllowedMACs) > 0 {
			out.AllowedMACs = append(out.AllowedMACs, topo.AllowedMACs...)
		}
		for _, pf := range topo.PortForwards {
			out.PortForwards = append(out.PortForwards, model.PortForward{
				Protocol:     pf.Protocol,
				ExternalPort: pf.ExternalPort,
				Destination:  pf.Destination,
			})
		}
		for _, r := range topo.PBRSourceRules {
			sources := []string{}
			if len(r.Sources) > 0 {
				sources = append(sources, r.Sources...)
			}
			out.PBR.SourceRules = append(out.PBR.SourceRules, model.PBRSourceRule{
				Sources: sources,
				Tunnel:  r.Tunnel,
			})
		}
		for _, r := range topo.PBRDomainRules {
			domains := []string{}
			if len(r.Domains) > 0 {
				domains = append(domains, r.Domains...)
			}
			out.PBR.DomainRules = append(out.PBR.DomainRules, model.PBRDomainRule{
				Tunnel:  r.Tunnel,
				Domains: domains,
			})
		}
		for _, r := range topo.PBRSourceDomainRules {
			out.PBR.SourceDomainRules = append(out.PBR.SourceDomainRules, model.PBRSourceDomainRule{
				Source:    r.Source,
				DomainSet: r.DomainSet,
				Tunnel:    r.Tunnel,
			})
		}
		for _, r := range topo.PooledRules {
			sources := []string{}
			if len(r.Sources) > 0 {
				sources = append(sources, r.Sources...)
			}
			out.PBR.PooledRules = append(out.PBR.PooledRules, model.PBRPooledRule{
				Sources: sources,
				Pool:    r.Pool,
			})
		}
	}

	// Chains + nested counters. Each chain's Counters is initialized as
	// a non-nil empty slice for the same JSON-shape reason.
	for _, ch := range r.Chains {
		mc := model.FirewallChain{
			Family: ch.Family, Table: ch.Table, Name: ch.Name, Type: ch.Type,
			Hook: ch.Hook, Priority: ch.Priority, Policy: ch.Policy,
			Handle:   ch.Handle,
			Counters: []model.RuleCounter{},
		}
		// Attach every per-rule counter belonging to this chain. We don't
		// know the chain's rule handles directly, so we filter countersByRule
		// by matching chain identity.
		for k, cs := range countersByRule {
			if k.family == ch.Family && k.table == ch.Table && k.chain == ch.Name {
				mc.Counters = append(mc.Counters, cs...)
			}
		}
		out.Chains = append(out.Chains, mc)
	}

	// UPnP leases derived from the miniupnpd table.
	for _, m := range r.UPnPMappings {
		out.UPnPLeases = append(out.UPnPLeases, model.UPnPLease{
			Protocol: m.Protocol, ExternalPort: m.ExternalPort,
			InternalAddr: m.InternalAddr, InternalPort: m.InternalPort,
			Description: m.Description,
		})
	}

	c.opts.State.SetFirewall(out)
	return nil
}
