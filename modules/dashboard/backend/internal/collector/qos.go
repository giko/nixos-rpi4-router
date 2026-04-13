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
	if c.opts.EgressInterface != "" {
		eg, err := tc.CollectCAKE(ctx, c.opts.Run, c.opts.EgressInterface)
		if err != nil {
			slog.Warn("qos: egress collect failed", "iface", c.opts.EgressInterface, "err", err)
		} else {
			eq := toModel(eg)
			out.Egress = &eq
		}
	}
	if c.opts.IngressInterface != "" {
		ig, err := tc.CollectHTB(ctx, c.opts.Run, c.opts.IngressInterface)
		if err != nil {
			slog.Warn("qos: ingress collect failed", "iface", c.opts.IngressInterface, "err", err)
		} else {
			iq := toModel(ig)
			out.Ingress = &iq
		}
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
