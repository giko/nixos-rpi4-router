package collector

import (
	"net/netip"
	"sync"
	"time"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/conntrack"
)

type FlowCountSample struct {
	T         time.Time
	OpenFlows uint32
}

type FlowCountOpts struct {
	TickDur  time.Duration
	RingSize int
}

type flowCountRing struct {
	samples []FlowCountSample
	head    int
	filled  int
}

func (r *flowCountRing) push(s FlowCountSample) {
	r.samples[r.head] = s
	r.head = (r.head + 1) % len(r.samples)
	if r.filled < len(r.samples) {
		r.filled++
	}
}

type FlowCount struct {
	mu      sync.RWMutex
	opts    FlowCountOpts
	rings   map[netip.Addr]*flowCountRing
	tracked map[netip.Addr]struct{}
}

func NewFlowCount(opts FlowCountOpts) *FlowCount {
	if opts.TickDur == 0 {
		opts.TickDur = 10 * time.Second
	}
	if opts.RingSize == 0 {
		opts.RingSize = 60
	}
	return &FlowCount{
		opts:    opts,
		rings:   make(map[netip.Addr]*flowCountRing),
		tracked: make(map[netip.Addr]struct{}),
	}
}

func (c *FlowCount) Track(ip netip.Addr) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.tracked[ip] = struct{}{}
	if _, ok := c.rings[ip]; !ok {
		c.rings[ip] = &flowCountRing{samples: make([]FlowCountSample, c.opts.RingSize)}
	}
}

func (c *FlowCount) Drop(ip netip.Addr) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.tracked, ip)
	delete(c.rings, ip)
}

func (c *FlowCount) Apply(now time.Time, snapshot []conntrack.FlowBytes) {
	c.mu.Lock()
	defer c.mu.Unlock()
	counts := make(map[netip.Addr]uint32)
	for _, fb := range snapshot {
		if _, tracked := c.tracked[fb.ClientIP]; !tracked {
			continue
		}
		counts[fb.ClientIP]++
	}
	for ip := range c.tracked {
		c.rings[ip].push(FlowCountSample{T: now, OpenFlows: counts[ip]})
	}
}

func (c *FlowCount) Snapshot(ip netip.Addr) ([]FlowCountSample, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	r, ok := c.rings[ip]
	if !ok {
		return nil, false
	}
	out := make([]FlowCountSample, 0, r.filled)
	start := (r.head - r.filled + len(r.samples)) % len(r.samples)
	for i := 0; i < r.filled; i++ {
		idx := (start + i) % len(r.samples)
		out = append(out, r.samples[idx])
	}
	return out, true
}
