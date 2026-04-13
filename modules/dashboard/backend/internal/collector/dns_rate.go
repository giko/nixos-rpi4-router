package collector

import (
	"net/netip"
	"sync"
	"time"
)

type DnsRateSample struct {
	T                time.Time
	QueriesPerWindow uint32
}

type DnsRateOpts struct {
	TickDur  time.Duration
	RingSize int
}

type dnsRateRing struct {
	samples []DnsRateSample
	head    int
	filled  int
}

func (r *dnsRateRing) push(s DnsRateSample) {
	r.samples[r.head] = s
	r.head = (r.head + 1) % len(r.samples)
	if r.filled < len(r.samples) {
		r.filled++
	}
}

type DnsRate struct {
	mu       sync.RWMutex
	opts     DnsRateOpts
	rings    map[netip.Addr]*dnsRateRing
	tracked  map[netip.Addr]struct{}
	counters map[netip.Addr]uint32
}

func NewDnsRate(opts DnsRateOpts) *DnsRate {
	if opts.TickDur == 0 {
		opts.TickDur = 10 * time.Second
	}
	if opts.RingSize == 0 {
		opts.RingSize = 60
	}
	return &DnsRate{
		opts:     opts,
		rings:    make(map[netip.Addr]*dnsRateRing),
		tracked:  make(map[netip.Addr]struct{}),
		counters: make(map[netip.Addr]uint32),
	}
}

func (d *DnsRate) Track(ip netip.Addr) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.tracked[ip] = struct{}{}
	if _, ok := d.rings[ip]; !ok {
		d.rings[ip] = &dnsRateRing{samples: make([]DnsRateSample, d.opts.RingSize)}
	}
}

func (d *DnsRate) Drop(ip netip.Addr) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.tracked, ip)
	delete(d.rings, ip)
	delete(d.counters, ip)
}

// Observe is called by the DNS ingest OnEntry callback for each IngestedEntry.
func (d *DnsRate) Observe(clientIP netip.Addr) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, tracked := d.tracked[clientIP]; !tracked {
		return
	}
	d.counters[clientIP]++
}

// Tick closes the current window, appends a sample per tracked client, and resets counters.
func (d *DnsRate) Tick(now time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for ip := range d.tracked {
		d.rings[ip].push(DnsRateSample{T: now, QueriesPerWindow: d.counters[ip]})
		d.counters[ip] = 0
	}
}

func (d *DnsRate) Snapshot(ip netip.Addr) ([]DnsRateSample, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	r, ok := d.rings[ip]
	if !ok {
		return nil, false
	}
	out := make([]DnsRateSample, 0, r.filled)
	start := (r.head - r.filled + len(r.samples)) % len(r.samples)
	for i := 0; i < r.filled; i++ {
		idx := (start + i) % len(r.samples)
		out = append(out, r.samples[idx])
	}
	return out, true
}
