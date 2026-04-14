package collector

import (
	"net/netip"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/conntrack"
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
	flows                   uint32
}

type minuteSlot struct {
	start     time.Time
	perDomain map[string]minuteDelta
	// flowKeys dedupes flows observed in this minute bucket. Scoped to
	// (domain → flow 5-tuple) so the same conntrack entry seen on
	// successive ticks within the same minute only increments Flows once.
	// Cleared when the bucket is rotated out of the 60-minute window.
	flowKeys map[flowDedupKey]struct{}
}

// flowDedupKey identifies a flow for per-minute dedup in the flows
// counter. The conntrack 5-tuple is stable for the lifetime of a flow;
// pairing it with the attributed domain prevents a single flow from
// double-counting if it somehow maps to multiple domains in the same
// minute (shouldn't happen in practice — belt + suspenders).
type flowDedupKey struct {
	key    conntrack.FlowKey
	domain string
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

// RecordFlow registers one observation of flowKey targeting the domain
// backing remote IP. Repeated calls with the same flowKey inside the
// same minute bucket are deduplicated, so a long-lived flow seen on
// successive ticks only increments Flows once per minute. Bucket
// rotation later subtracts the per-bucket flow count from the 60-minute
// rollup, keeping Snapshot().Flows consistent with the sliding window.
func (td *TopDestinations) RecordFlow(ip netip.Addr, question string, flowKey conntrack.FlowKey, now time.Time) {
	domain := registrableDomain(question)
	td.mu.RLock()
	ca, ok := td.agg[ip]
	td.mu.RUnlock()
	if !ok {
		return
	}
	ca.mu.Lock()
	defer ca.mu.Unlock()

	bucketIdx := ca.rotate(now, td.opts.MinuteBuckets)
	if bucketIdx < 0 {
		return // outside the sliding window; nothing to attribute
	}

	slot := &ca.buckets[bucketIdx]
	if slot.flowKeys == nil {
		slot.flowKeys = make(map[flowDedupKey]struct{})
	}
	dk := flowDedupKey{key: flowKey, domain: domain}
	if _, seen := slot.flowKeys[dk]; seen {
		return // already counted this flow in this minute
	}
	slot.flowKeys[dk] = struct{}{}

	if slot.perDomain == nil {
		slot.perDomain = make(map[string]minuteDelta)
	}
	s := slot.perDomain[domain]
	s.flows++
	slot.perDomain[domain] = s

	t := ca.totals[domain]
	if t == nil {
		t = &TopDestination{Domain: domain}
		ca.totals[domain] = t
	}
	t.Flows++
	if now.After(t.LastSeen) {
		t.LastSeen = now
	}
	ca.evictOverCap(td.opts.PerClientCap)
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

	bucketIdx := ca.rotate(now, td.opts.MinuteBuckets)
	if bucketIdx < 0 {
		return // too old; outside the 1h window
	}

	slot := &ca.buckets[bucketIdx]
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
	if now.After(t.LastSeen) {
		t.LastSeen = now
	}

	ca.evictOverCap(td.opts.PerClientCap)
}

// evictOverCap trims the per-client totals map down to PerClientCap by
// dropping the least-recently-seen domains. Purges matching entries
// from every minute bucket (both perDomain deltas and flowKey dedup
// sets) so a later rotate() doesn't subtract stale deltas and a
// re-appearing domain doesn't carry ghost flow-dedup state.
func (ca *clientAgg) evictOverCap(cap int) {
	for len(ca.totals) > cap {
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
		for i := range ca.buckets {
			delete(ca.buckets[i].perDomain, oldestName)
			for dk := range ca.buckets[i].flowKeys {
				if dk.domain == oldestName {
					delete(ca.buckets[i].flowKeys, dk)
				}
			}
		}
	}
}

// rotate updates curMin forward if 'minute' is newer, or returns the
// index of the bucket representing 'minute' if it falls within the
// current window. Returns -1 when the minute is older than the full
// buckets-minute window (caller should drop the record).
func (ca *clientAgg) rotate(now time.Time, buckets int) int {
	minute := now.Truncate(time.Minute)
	if ca.curMin.IsZero() {
		ca.curMin = minute
		ca.buckets[ca.head].start = minute
		return ca.head
	}
	if minute.Equal(ca.curMin) {
		return ca.head
	}
	if minute.Before(ca.curMin) {
		// How many minutes behind curMin?
		diff := ca.curMin.Sub(minute) / time.Minute
		if int(diff) >= buckets {
			return -1 // outside window; record too old to account
		}
		idx := (ca.head - int(diff) + buckets) % buckets
		return idx
	}
	// Advance forward, expiring old buckets as they age out.
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
			if t.Flows >= v.flows {
				t.Flows -= v.flows
			} else {
				t.Flows = 0
			}
			if t.Queries == 0 && t.Blocked == 0 && t.Bytes == 0 && t.Flows == 0 {
				delete(ca.totals, domain)
			}
		}
		old.perDomain = nil
		old.flowKeys = nil
		old.start = nextMinute
		ca.curMin = nextMinute
	}
	return ca.head
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
