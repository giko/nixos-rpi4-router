package collector

import (
	"context"
	"log/slog"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/model"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/tc"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/state"
)

// QoSOpts configures the QoS collector.
type QoSOpts struct {
	State            *state.State
	Run              tc.Runner // nil → DefaultRunner
	EgressInterface  string    // typically "eth1"
	IngressInterface string    // typically "ifb4eth1"
}

// QoS is a medium-tier collector that snapshots the egress CAKE qdisc
// on the WAN physical interface and the HTB+fq_codel qdisc on its IFB.
// One of either side failing is non-fatal — the other side still gets
// published.
type QoS struct {
	opts QoSOpts
}

// NewQoS creates a QoS collector.
func NewQoS(opts QoSOpts) *QoS {
	if opts.Run == nil {
		opts.Run = tc.DefaultRunner
	}
	return &QoS{opts: opts}
}

func (*QoS) Name() string { return "qos" }
func (*QoS) Tier() Tier   { return Medium }

func (c *QoS) Run(ctx context.Context) error {
	out := model.QoS{}
	var (
		egressErr, ingressErr error
		anySuccess            bool
	)
	if c.opts.EgressInterface != "" {
		eg, err := tc.CollectCAKE(ctx, c.opts.Run, c.opts.EgressInterface)
		if err != nil {
			slog.Warn("qos: egress collect failed", "iface", c.opts.EgressInterface, "err", err)
			egressErr = err
		} else {
			eq := toModel(eg)
			out.Egress = &eq
			anySuccess = true
		}
	}
	if c.opts.IngressInterface != "" {
		ig, err := tc.CollectHTB(ctx, c.opts.Run, c.opts.IngressInterface)
		if err != nil {
			slog.Warn("qos: ingress collect failed", "iface", c.opts.IngressInterface, "err", err)
			ingressErr = err
		} else {
			iq := toModel(ig)
			out.Ingress = &iq
			anySuccess = true
		}
	}
	// If at least one side delivered a snapshot, publish — the other
	// side staying nil correctly signals "this tick failed for that
	// half" without dropping good data. If both sides failed (or both
	// interfaces were empty strings), DO NOT publish — keep the
	// previous snapshot so IsStale can surface the outage instead of
	// us replacing it with a fresh-but-empty QoS{}.
	if !anySuccess {
		if egressErr != nil {
			return egressErr
		}
		if ingressErr != nil {
			return ingressErr
		}
		// Both interfaces unset (dev-mode): nothing to do.
		return nil
	}
	c.opts.State.SetQoS(out)
	return nil
}

func toModel(s tc.QdiscStats) model.QdiscStats {
	dst := model.QdiscStats{
		Kind: s.Kind, BandwidthBps: s.BandwidthBps,
		SentBytes: s.SentBytes, SentPackets: s.SentPackets,
		Dropped: s.Dropped, Overlimits: s.Overlimits, Requeues: s.Requeues,
		BacklogBytes: s.BacklogBytes, BacklogPkts: s.BacklogPkts,
		NewFlowCount:  s.NewFlowCount,
		OldFlowsLen:   s.OldFlowsLen,
		NewFlowsLen:   s.NewFlowsLen,
		ECNMark:       s.ECNMark,
		DropOverlimit: s.DropOverlimit,
	}
	for _, t := range s.Tins {
		dst.Tins = append(dst.Tins, model.CAKETin{
			Name: t.Name, ThreshKbit: t.ThreshKbit,
			TargetUs: t.TargetUs, IntervalUs: t.IntervalUs,
			PeakDelayUs: t.PeakDelayUs, AvgDelayUs: t.AvgDelayUs,
			BacklogBytes: t.BacklogBytes,
			Packets:      t.Packets, Bytes: t.Bytes,
			Drops: t.Drops, Marks: t.Marks,
		})
	}
	return dst
}
