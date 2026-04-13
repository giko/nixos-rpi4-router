package collector

import (
	"net/netip"
	"sync"
	"time"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/conntrack"
)

type TrafficSample struct {
	T     time.Time
	RxBps uint64
	TxBps uint64
}

type TrafficStats struct {
	CurrentRx, CurrentTx uint64
	PeakRx, PeakTx       uint64
	TotalRxBytes         uint64
	TotalTxBytes         uint64
}

type ClientTrafficOpts struct {
	TickDur  time.Duration
	RingSize int
}

type trafficRing struct {
	samples []TrafficSample
	head    int
	filled  int
}

func (r *trafficRing) push(s TrafficSample) {
	r.samples[r.head] = s
	r.head = (r.head + 1) % len(r.samples)
	if r.filled < len(r.samples) {
		r.filled++
	}
}

type flowSnap struct {
	orig, reply uint64
}

type ClientTraffic struct {
	mu       sync.RWMutex
	opts     ClientTrafficOpts
	baseline map[conntrack.FlowKey]flowSnap
	rings    map[netip.Addr]*trafficRing
	tracked  map[netip.Addr]struct{}
	seeded   bool
}

func NewClientTraffic(opts ClientTrafficOpts) *ClientTraffic {
	if opts.TickDur == 0 {
		opts.TickDur = 10 * time.Second
	}
	if opts.RingSize == 0 {
		opts.RingSize = 60
	}
	return &ClientTraffic{
		opts:     opts,
		baseline: make(map[conntrack.FlowKey]flowSnap),
		rings:    make(map[netip.Addr]*trafficRing),
		tracked:  make(map[netip.Addr]struct{}),
	}
}

func (c *ClientTraffic) Track(ip netip.Addr) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tracked[ip] = struct{}{}
	if _, ok := c.rings[ip]; !ok {
		c.rings[ip] = &trafficRing{samples: make([]TrafficSample, c.opts.RingSize)}
	}
}

func (c *ClientTraffic) Drop(ip netip.Addr) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.tracked, ip)
	delete(c.rings, ip)
}

func (c *ClientTraffic) Apply(now time.Time, snapshot []conntrack.FlowBytes) {
	c.mu.Lock()
	defer c.mu.Unlock()

	rxDelta := make(map[netip.Addr]uint64)
	txDelta := make(map[netip.Addr]uint64)
	next := make(map[conntrack.FlowKey]flowSnap, len(snapshot))

	for _, fb := range snapshot {
		prev, seen := c.baseline[fb.Key]
		var orig, reply uint64
		if seen {
			if fb.OrigBytes >= prev.orig {
				orig = fb.OrigBytes - prev.orig
			}
			if fb.ReplyBytes >= prev.reply {
				reply = fb.ReplyBytes - prev.reply
			}
		}
		next[fb.Key] = flowSnap{orig: fb.OrigBytes, reply: fb.ReplyBytes}

		if _, tracked := c.tracked[fb.ClientIP]; !tracked {
			continue
		}
		switch fb.Direction {
		case conntrack.DirOutbound:
			txDelta[fb.ClientIP] += orig
			rxDelta[fb.ClientIP] += reply
		case conntrack.DirInbound:
			rxDelta[fb.ClientIP] += orig
			txDelta[fb.ClientIP] += reply
		}
	}
	c.baseline = next

	if !c.seeded {
		c.seeded = true
		return
	}

	secs := uint64(c.opts.TickDur / time.Second)
	if secs == 0 {
		secs = 1
	}
	for ip := range c.tracked {
		r := c.rings[ip]
		rx := rxDelta[ip] * 8 / secs
		tx := txDelta[ip] * 8 / secs
		r.push(TrafficSample{T: now, RxBps: rx, TxBps: tx})
	}
}

func (c *ClientTraffic) Snapshot(ip netip.Addr) ([]TrafficSample, TrafficStats, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	r, ok := c.rings[ip]
	if !ok {
		return nil, TrafficStats{}, false
	}
	out := make([]TrafficSample, 0, r.filled)
	start := (r.head - r.filled + len(r.samples)) % len(r.samples)
	for i := 0; i < r.filled; i++ {
		idx := (start + i) % len(r.samples)
		out = append(out, r.samples[idx])
	}
	var stats TrafficStats
	secs := uint64(c.opts.TickDur / time.Second)
	if secs == 0 {
		secs = 1
	}
	for _, s := range out {
		if s.RxBps > stats.PeakRx {
			stats.PeakRx = s.RxBps
		}
		if s.TxBps > stats.PeakTx {
			stats.PeakTx = s.TxBps
		}
		stats.TotalRxBytes += s.RxBps * secs / 8
		stats.TotalTxBytes += s.TxBps * secs / 8
	}
	if n := len(out); n > 0 {
		stats.CurrentRx = out[n-1].RxBps
		stats.CurrentTx = out[n-1].TxBps
	}
	return out, stats, true
}
