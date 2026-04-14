package collector

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/netip"
	"os/exec"
	"sync"
	"sync/atomic"
	"time"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/conntrack"
)

// ClientDetailOpts wires the runtime. Sources are interfaces so tests
// can plug in fakes; production wires /proc readers + a real AdGuard
// client.
type ClientDetailOpts struct {
	// LANPrefixes is the set of subnets considered "ours" for direction
	// attribution. Production passes the router's LAN subnets; tests can
	// pass nil to use IsPrivate-based fallback.
	LANPrefixes []netip.Prefix

	// RouteTags maps a numeric conntrack mark to a tunnel name (e.g.
	// 0x10000 → "WAN", 0x20000 → "wg_sw"). Built from topology in
	// production; tests pass an empty map.
	RouteTags map[uint32]string

	// ConntrackReader returns a fresh /proc/net/nf_conntrack reader
	// each call. Caller closes. nil disables hot-tier flow ingestion
	// (used in unit tests that drive Tick directly).
	ConntrackReader func() (io.ReadCloser, error)

	// AdguardIngest is the AdGuard client surface used by DnsIngest.
	// nil disables DNS ingestion.
	AdguardIngest AdguardClient

	// LeaseGrace is the LifecycleTracker grace duration. Default 15m.
	LeaseGrace time.Duration

	// TickDur is the hot-tier tick. Default 10s — also used as the
	// per-collector ring tick width (TrafficSample width, DnsRate
	// window, FlowCount tick).
	TickDur time.Duration
}

// ClientDetailRuntime bundles every primitive needed by the per-client
// HTTP handlers and orchestrates the ticker fan-out. Public methods are
// safe to call concurrently — the underlying primitives carry their own
// locking; the flow snapshot uses atomic.Value.
type ClientDetailRuntime struct {
	opts ClientDetailOpts

	Lifecycle       *LifecycleTracker
	Traffic         *ClientTraffic
	FlowCount       *FlowCount
	DnsRate         *DnsRate
	TopDestinations *TopDestinations
	Domains         *DomainEnricher
	DnsIngest       *DnsIngest

	flows atomic.Value // []conntrack.FlowBytes — never nil after first Tick
	mu    sync.Mutex   // serialises Tick to keep the snapshot atomic
}

// NewClientDetailRuntime wires every primitive and registers the
// lifecycle callbacks that fan births / deaths / rebirths into the
// per-client collectors.
func NewClientDetailRuntime(opts ClientDetailOpts) *ClientDetailRuntime {
	if opts.LeaseGrace == 0 {
		opts.LeaseGrace = 15 * time.Minute
	}
	if opts.TickDur == 0 {
		opts.TickDur = 10 * time.Second
	}

	rt := &ClientDetailRuntime{opts: opts}
	rt.flows.Store([]conntrack.FlowBytes{})

	rt.Lifecycle = NewLifecycleTracker(opts.LeaseGrace)
	rt.Traffic = NewClientTraffic(ClientTrafficOpts{TickDur: opts.TickDur})
	rt.FlowCount = NewFlowCount(FlowCountOpts{TickDur: opts.TickDur})
	rt.DnsRate = NewDnsRate(DnsRateOpts{TickDur: opts.TickDur})
	rt.TopDestinations = NewTopDestinations(TopDestOpts{})
	rt.Domains = NewDomainEnricher(EnricherOpts{})

	rt.Lifecycle.OnBirth = func(ip netip.Addr) {
		rt.Traffic.Track(ip)
		rt.FlowCount.Track(ip)
		rt.DnsRate.Track(ip)
		rt.TopDestinations.Track(ip)
	}
	rt.Lifecycle.OnRebirth = func(ip netip.Addr) {
		rt.Traffic.Drop(ip)
		rt.FlowCount.Drop(ip)
		rt.DnsRate.Drop(ip)
		rt.TopDestinations.Drop(ip)
		rt.Domains.Drop(ip)

		rt.Traffic.Track(ip)
		rt.FlowCount.Track(ip)
		rt.DnsRate.Track(ip)
		rt.TopDestinations.Track(ip)
	}
	// OnDeath intentionally nil — tombstone preserves state during the
	// 15-minute grace; final cleanup happens via Reap.

	if opts.AdguardIngest != nil {
		rt.DnsIngest = NewDnsIngest(DnsIngestOpts{
			Adguard: opts.AdguardIngest,
			OnEntry: rt.handleDnsEntry,
		})
	}

	return rt
}

// handleDnsEntry fans one ingested DNS row into enricher + DnsRate +
// TopDestinations. Called by DnsIngest.
func (rt *ClientDetailRuntime) handleDnsEntry(e IngestedEntry) {
	rt.DnsRate.Observe(e.ClientIP)
	rt.TopDestinations.RecordQuery(e.ClientIP, e.Question, e.Blocked, e.Time)
	for _, ans := range e.Answers {
		rt.Domains.Record(e.ClientIP, ans.IP, e.Question, time.Duration(ans.TTL)*time.Second, e.Time)
	}
}

// Tick runs the hot-tier fan-out: read conntrack, store flow snapshot,
// feed Traffic + FlowCount + TopDestinations bytes attribution. Safe
// for the timer goroutine to call every TickDur.
func (rt *ClientDetailRuntime) Tick(now time.Time) error {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if rt.opts.ConntrackReader == nil {
		return nil
	}
	r, err := rt.opts.ConntrackReader()
	if err != nil {
		return err
	}
	defer r.Close()

	flows, err := conntrack.EnumerateFlows(r, conntrack.EnumerateOpts{
		RouteTags:   rt.opts.RouteTags,
		LANPrefixes: rt.opts.LANPrefixes,
	})
	if err != nil {
		return err
	}
	rt.flows.Store(flows)

	rt.Traffic.Apply(now, flows)
	rt.FlowCount.Apply(now, flows)
	rt.DnsRate.Tick(now)

	// Bytes attribution for top-destinations: route each flow's
	// client-side outbound bytes (the cumulative reverse-direction count
	// — i.e., bytes the client received from the remote) to the
	// remote IP's resolved domain when the enricher knows it.
	for _, fb := range flows {
		if !fb.RemoteIP.IsValid() {
			continue
		}
		domain, ok := rt.Domains.Lookup(fb.ClientIP, fb.RemoteIP, now)
		if !ok {
			continue
		}
		var bytes uint64
		switch fb.Direction {
		case conntrack.DirOutbound:
			bytes = fb.ReplyBytes
		case conntrack.DirInbound:
			bytes = fb.OrigBytes
		}
		rt.TopDestinations.RecordFlow(fb.ClientIP, domain, fb.Key, now)
		rt.TopDestinations.RecordBytes(fb.ClientIP, domain, bytes, now)
	}
	rt.TopDestinations.Advance(now)
	return nil
}

// IngestTick runs one DNS ingestion pass. No-op when no AdGuard client
// was provided.
func (rt *ClientDetailRuntime) IngestTick(ctx context.Context, now time.Time) error {
	if rt.DnsIngest == nil {
		return nil
	}
	return rt.DnsIngest.Tick(ctx, now)
}

// OnLeaseScan is called by the lease-scanner goroutine each pass. It
// updates the LifecycleTracker (which fires Track/Drop/Drop+Track via
// the registered callbacks).
func (rt *ClientDetailRuntime) OnLeaseScan(leased []netip.Addr, now time.Time) LifecycleEvents {
	return rt.Lifecycle.OnScan(leased, now)
}

// Reap removes tombstoned IPs that have aged past the grace window and
// drops their per-client state from every collector. Call from the
// 1-minute background goroutine.
func (rt *ClientDetailRuntime) Reap(now time.Time) []netip.Addr {
	reaped := rt.Lifecycle.Reap(now)
	for _, ip := range reaped {
		rt.Traffic.Drop(ip)
		rt.FlowCount.Drop(ip)
		rt.DnsRate.Drop(ip)
		rt.TopDestinations.Drop(ip)
		rt.Domains.Drop(ip)
	}
	return reaped
}

// Snapshot returns the most recent conntrack snapshot. Implements the
// server-side FlowSource interface.
func (rt *ClientDetailRuntime) Snapshot() []conntrack.FlowBytes {
	v := rt.flows.Load()
	if v == nil {
		return nil
	}
	return v.([]conntrack.FlowBytes)
}

// DefaultConntrackReader returns a conntrack-snapshot opener suitable
// for production use. It shells out to `conntrack -L -o extended`
// instead of reading /proc/net/nf_conntrack directly: the proc file is
// mode 0640 root:root and the dashboard runs as a DynamicUser, so the
// Unix DAC check rejects the open before CAP_NET_ADMIN is consulted.
// The extended-format CLI output is identical to the proc file for
// ipv4 rows, so the existing parser needs no changes.
func DefaultConntrackReader() func() (io.ReadCloser, error) {
	return func() (io.ReadCloser, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "conntrack", "-L", "-o", "extended")
		out, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("conntrack -L: %w", err)
		}
		return io.NopCloser(bytes.NewReader(out)), nil
	}
}
