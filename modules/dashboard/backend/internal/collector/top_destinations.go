package collector

import (
	"net/netip"
	"sort"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/publicsuffix"
)

type TopDestination struct {
	Domain   string
	Queries  uint64
	Blocked  uint64
	Bytes    uint64
	Flows    uint32
	LastSeen time.Time
}

type TopDestOpts struct {
	MinuteBuckets int // default 60
	PerClientCap  int // default 50
}

type minuteDelta struct {
	queries, blocked, bytes uint64
}

type minuteSlot struct {
	start     time.Time
	perDomain map[string]minuteDelta
}

type clientAgg struct {
	mu      sync.Mutex
	totals  map[string]*TopDestination
	buckets []minuteSlot
	head    int
	curMin  time.Time
}

type TopDestinations struct {
	mu   sync.RWMutex
	opts TopDestOpts
	agg  map[netip.Addr]*clientAgg
}

func NewTopDestinations(opts TopDestOpts) *TopDestinations {
	if opts.MinuteBuckets == 0 {
		opts.MinuteBuckets = 60
	}
	if opts.PerClientCap == 0 {
		opts.PerClientCap = 50
	}
	return &TopDestinations{opts: opts, agg: make(map[netip.Addr]*clientAgg)}
}

func (td *TopDestinations) Track(ip netip.Addr) {
	td.mu.Lock()
	defer td.mu.Unlock()
	if _, ok := td.agg[ip]; !ok {
		td.agg[ip] = &clientAgg{
			totals:  make(map[string]*TopDestination),
			buckets: make([]minuteSlot, td.opts.MinuteBuckets),
		}
	}
}

func (td *TopDestinations) Drop(ip netip.Addr) {
	td.mu.Lock()
	defer td.mu.Unlock()
	delete(td.agg, ip)
}

func registrableDomain(question string) string {
	q := strings.TrimSuffix(question, ".")
	d, err := publicsuffix.EffectiveTLDPlusOne(q)
	if err != nil {
		return q
	}
	return d
}

func (td *TopDestinations) RecordQuery(ip netip.Addr, question string, blocked bool, now time.Time) {
	td.record(ip, registrableDomain(question), blocked, 0, now, true)
}

func (td *TopDestinations) RecordBytes(ip netip.Addr, question string, bytes uint64, now time.Time) {
	if bytes == 0 {
		return
	}
	td.record(ip, registrableDomain(question), false, bytes, now, false)
}

func (td *TopDestinations) record(ip netip.Addr, domain string, blocked bool, bytes uint64, now time.Time, isQuery bool) {
	td.mu.RLock()
	ca, ok := td.agg[ip]
	td.mu.RUnlock()
	if !ok {
		return
	}
	ca.mu.Lock()
	defer ca.mu.Unlock()
	ca.rotate(now, td.opts.MinuteBuckets)

	slot := &ca.buckets[ca.head]
	if slot.perDomain == nil {
		slot.perDomain = make(map[string]minuteDelta)
	}
	s := slot.perDomain[domain]
	if isQuery {
		s.queries++
		if blocked {
			s.blocked++
		}
	} else {
		s.bytes += bytes
	}
	slot.perDomain[domain] = s

	t := ca.totals[domain]
	if t == nil {
		t = &TopDestination{Domain: domain}
		ca.totals[domain] = t
	}
	if isQuery {
		t.Queries++
		if blocked {
			t.Blocked++
		}
	} else {
		t.Bytes += bytes
	}
	t.LastSeen = now

	// Evict least-recent when over cap.
	for len(ca.totals) > td.opts.PerClientCap {
		var oldestName string
		var oldestTime time.Time
		first := true
		for n, v := range ca.totals {
			if first || v.LastSeen.Before(oldestTime) {
				oldestName, oldestTime = n, v.LastSeen
				first = false
			}
		}
		if oldestName == "" {
			break
		}
		delete(ca.totals, oldestName)
	}
}

func (ca *clientAgg) rotate(now time.Time, buckets int) {
	minute := now.Truncate(time.Minute)
	if ca.curMin.IsZero() {
		ca.curMin = minute
		ca.buckets[ca.head].start = minute
		return
	}
	if minute.Equal(ca.curMin) {
		return
	}
	for !ca.curMin.Equal(minute) {
		ca.head = (ca.head + 1) % buckets
		nextMinute := ca.curMin.Add(time.Minute)
		old := &ca.buckets[ca.head]
		for domain, v := range old.perDomain {
			t := ca.totals[domain]
			if t == nil {
				continue
			}
			if t.Queries >= v.queries {
				t.Queries -= v.queries
			} else {
				t.Queries = 0
			}
			if t.Blocked >= v.blocked {
				t.Blocked -= v.blocked
			} else {
				t.Blocked = 0
			}
			if t.Bytes >= v.bytes {
				t.Bytes -= v.bytes
			} else {
				t.Bytes = 0
			}
			if t.Queries == 0 && t.Blocked == 0 && t.Bytes == 0 {
				delete(ca.totals, domain)
			}
		}
		old.perDomain = nil
		old.start = nextMinute
		ca.curMin = nextMinute
	}
}

func (td *TopDestinations) Advance(now time.Time) {
	td.mu.RLock()
	defer td.mu.RUnlock()
	for _, ca := range td.agg {
		ca.mu.Lock()
		ca.rotate(now, td.opts.MinuteBuckets)
		ca.mu.Unlock()
	}
}

func (td *TopDestinations) Snapshot(ip netip.Addr) []TopDestination {
	td.mu.RLock()
	ca, ok := td.agg[ip]
	td.mu.RUnlock()
	if !ok {
		return nil
	}
	ca.mu.Lock()
	defer ca.mu.Unlock()
	out := make([]TopDestination, 0, len(ca.totals))
	for _, v := range ca.totals {
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Bytes != out[j].Bytes {
			return out[i].Bytes > out[j].Bytes
		}
		if out[i].Blocked != out[j].Blocked {
			return out[i].Blocked > out[j].Blocked
		}
		return out[i].Queries > out[j].Queries
	})
	return out
}
