package collector

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"
	"time"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/model"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/proc"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/state"
)

const ringSize = 60

// TrafficOpts configures the Traffic collector.
type TrafficOpts struct {
	NetDevPath string
	Interfaces []string
	// Roles maps interface name → role ("lan", "wan", "tunnel"). Any
	// interface whose name is in Interfaces but not in Roles gets role
	// "" in the emitted model. Keeping role assignment driven by
	// topology means the frontend can locate the WAN interface without
	// hardcoding "eth1".
	Roles map[string]string
	State *state.State
}

// Traffic reads /proc/net/dev, computes per-interface bit rates from
// counter deltas, maintains a 60-sample ring buffer, and publishes
// the result via State.SetTraffic.
type Traffic struct {
	opts     TrafficOpts
	lastRead map[string]proc.NetDevStats
	lastTS   time.Time
	rings    map[string]*ring
}

type ring struct {
	samples [ringSize]model.InterfaceSample
	len     int
}

// push appends a sample, shifting left if full.
func (r *ring) push(s model.InterfaceSample) {
	if r.len < ringSize {
		r.samples[r.len] = s
		r.len++
		return
	}
	copy(r.samples[:], r.samples[1:])
	r.samples[ringSize-1] = s
}

// slice returns a copy of the populated portion.
func (r *ring) slice() []model.InterfaceSample {
	out := make([]model.InterfaceSample, r.len)
	copy(out, r.samples[:r.len])
	return out
}

// NewTraffic creates a Traffic collector.
func NewTraffic(opts TrafficOpts) *Traffic {
	rings := make(map[string]*ring, len(opts.Interfaces))
	for _, iface := range opts.Interfaces {
		rings[iface] = &ring{}
	}
	return &Traffic{
		opts:  opts,
		rings: rings,
	}
}

func (*Traffic) Name() string { return "traffic" }
func (*Traffic) Tier() Tier   { return Hot }

// Run performs a single collection pass.
func (t *Traffic) Run(_ context.Context) error {
	stats, err := proc.ReadNetDev(t.opts.NetDevPath)
	if err != nil {
		return err
	}

	now := time.Now()
	elapsed := now.Sub(t.lastTS).Seconds()

	ifaces := make([]model.Interface, 0, len(t.opts.Interfaces))
	for _, name := range t.opts.Interfaces {
		cur, ok := stats[name]
		if !ok {
			continue
		}

		var rxBps, txBps uint64
		if t.lastRead != nil && elapsed > 0 {
			if prev, found := t.lastRead[name]; found {
				rxBps = rateBps(prev.RXBytes, cur.RXBytes, elapsed)
				txBps = rateBps(prev.TXBytes, cur.TXBytes, elapsed)
			}
		}

		r := t.rings[name]
		if r == nil {
			r = &ring{}
			t.rings[name] = r
		}
		r.push(model.InterfaceSample{RXBps: rxBps, TXBps: txBps})

		ifaces = append(ifaces, model.Interface{
			Name:         name,
			Role:         t.opts.Roles[name],
			Operstate:    readOperstate(name),
			RXBps:        rxBps,
			TXBps:        txBps,
			RXBytesTotal: cur.RXBytes,
			TXBytesTotal: cur.TXBytes,
			Samples60s:   r.slice(),
		})
	}

	t.lastRead = stats
	t.lastTS = now

	t.opts.State.SetTraffic(model.Traffic{Interfaces: ifaces})
	return nil
}

// readOperstate reads /sys/class/net/<name>/operstate. Returns "unknown"
// on any error (e.g. running on a Mac where sysfs doesn't exist).
func readOperstate(name string) string {
	b, err := os.ReadFile(fmt.Sprintf("/sys/class/net/%s/operstate", name))
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(b))
}

// rateBps computes the bit rate from a byte counter delta over elapsed seconds.
// Guards against 64-bit counter wrap-around.
func rateBps(prev, cur uint64, elapsed float64) uint64 {
	var delta uint64
	if cur >= prev {
		delta = cur - prev
	} else {
		// Counter wrapped (unlikely for 64-bit, but safe).
		delta = (math.MaxUint64 - prev) + cur + 1
	}
	return uint64(float64(delta) / elapsed * 8)
}
