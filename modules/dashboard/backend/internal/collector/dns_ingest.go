package collector

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/adguard"
)

// IngestedEntry is one normalized DNS query-log record handed to
// downstream consumers (domain enricher, top-destinations aggregator).
type IngestedEntry struct {
	Time     time.Time
	ClientIP netip.Addr
	Question string
	QType    string
	Blocked  bool
	Answers  []IngestedAnswer
}

// IngestedAnswer is a parsed A/AAAA record from the query's response.
type IngestedAnswer struct {
	IP  netip.Addr
	TTL uint32
}

// AdguardClient is the minimal AdGuard-client surface used by DnsIngest.
// It lets tests inject a fake without depending on the full *adguard.Client.
type AdguardClient interface {
	FetchQueryLogPage(ctx context.Context, olderThan time.Time, limit int) (adguard.QueryLogResponse, error)
}

// DnsIngestOpts configures a DnsIngest collector.
type DnsIngestOpts struct {
	Adguard  AdguardClient
	OnEntry  func(IngestedEntry)
	PageSize int           // default 500
	Overlap  time.Duration // default 30s
}

// DnsIngest polls AdGuard's query log in pages and hands each unique
// entry to OnEntry exactly once. Overlap between successive pages is
// deduplicated via an in-memory hash set keyed on (time, client, question).
type DnsIngest struct {
	opts DnsIngestOpts

	mu         sync.Mutex
	highWater  time.Time
	recentHash map[uint64]time.Time
}

// NewDnsIngest constructs a DnsIngest with sane defaults.
func NewDnsIngest(opts DnsIngestOpts) *DnsIngest {
	if opts.PageSize <= 0 {
		opts.PageSize = 500
	}
	if opts.Overlap == 0 {
		opts.Overlap = 30 * time.Second
	}
	return &DnsIngest{
		opts:       opts,
		recentHash: make(map[uint64]time.Time),
	}
}

// Tick fetches one or more pages newest-first, stops when it crosses the
// watermark minus the overlap window, and invokes OnEntry for each new
// unique entry. Tick is safe to call concurrently but serializes itself
// so pagination state is consistent.
func (d *DnsIngest) Tick(ctx context.Context, _ time.Time) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	cutoff := d.highWater.Add(-d.opts.Overlap)
	newest := time.Time{}
	var older time.Time // zero == "newest page" in FetchQueryLogPage

	for {
		resp, err := d.opts.Adguard.FetchQueryLogPage(ctx, older, d.opts.PageSize)
		if err != nil {
			return err
		}

		var rows []rawIngestEntry
		if len(resp.Data) > 0 {
			if err := json.Unmarshal(resp.Data, &rows); err != nil {
				return fmt.Errorf("dns-ingest: decode page: %w", err)
			}
		}
		if len(rows) == 0 {
			break
		}

		shouldStop := false
		for _, row := range rows {
			entry, ok := normalizeIngest(row)
			if !ok {
				continue
			}
			if entry.Time.Before(cutoff) {
				shouldStop = true
				break
			}
			h := hashIngest(entry)
			if _, dup := d.recentHash[h]; dup {
				continue
			}
			d.recentHash[h] = entry.Time
			if entry.Time.After(newest) {
				newest = entry.Time
			}
			if d.opts.OnEntry != nil {
				d.opts.OnEntry(entry)
			}
		}

		if shouldStop || len(rows) < d.opts.PageSize {
			break
		}

		// Advance the paging cursor to the oldest entry in this page.
		oldestInPage, err := time.Parse(time.RFC3339Nano, resp.Oldest)
		if err != nil {
			// Can't advance safely — stop rather than loop forever.
			break
		}
		if oldestInPage.Before(cutoff) {
			break
		}
		older = oldestInPage
	}

	if newest.After(d.highWater) {
		d.highWater = newest
	}
	d.gc(cutoff)
	return nil
}

// gc drops hash entries older than the dedup window so the set stays
// bounded across ticks. The one-minute slack beyond cutoff absorbs any
// AdGuard clock skew.
func (d *DnsIngest) gc(cutoff time.Time) {
	horizon := cutoff.Add(-time.Minute)
	for h, t := range d.recentHash {
		if t.Before(horizon) {
			delete(d.recentHash, h)
		}
	}
}

// rawIngestEntry is a subset of AdGuard's per-entry JSON shape.
type rawIngestEntry struct {
	Time     string `json:"time"`
	Client   string `json:"client"`
	Question struct {
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"question"`
	Answer []struct {
		Value string `json:"value"`
		TTL   uint32 `json:"ttl"`
	} `json:"answer"`
	Reason string `json:"reason"`
}

// normalizeIngest converts a raw row into an IngestedEntry. Returns
// (_, false) for rows with unparseable time or client IP.
func normalizeIngest(row rawIngestEntry) (IngestedEntry, bool) {
	t, err := time.Parse(time.RFC3339Nano, row.Time)
	if err != nil {
		return IngestedEntry{}, false
	}
	client, err := netip.ParseAddr(row.Client)
	if err != nil {
		return IngestedEntry{}, false
	}
	var answers []IngestedAnswer
	if len(row.Answer) > 0 {
		answers = make([]IngestedAnswer, 0, len(row.Answer))
		for _, a := range row.Answer {
			ip, err := netip.ParseAddr(a.Value)
			if err != nil {
				continue
			}
			answers = append(answers, IngestedAnswer{IP: ip, TTL: a.TTL})
		}
	}
	return IngestedEntry{
		Time:     t,
		ClientIP: client,
		Question: row.Question.Name,
		QType:    row.Question.Type,
		Blocked:  strings.HasPrefix(row.Reason, "Filtered"),
		Answers:  answers,
	}, true
}

// hashIngest produces a stable 64-bit dedup key from (time, client IP,
// question, qtype). Collisions across distinct tuples are astronomically
// unlikely given a 64-bit truncated SHA-256. QType participates so that
// a dual-stack client issuing an A and AAAA for the same name within a
// single sub-second window is not folded into one dedup slot.
func hashIngest(e IngestedEntry) uint64 {
	h := sha256.New()
	_ = binary.Write(h, binary.LittleEndian, e.Time.UnixNano())
	h.Write([]byte(e.ClientIP.String()))
	h.Write([]byte("|"))
	h.Write([]byte(e.Question))
	h.Write([]byte("|"))
	h.Write([]byte(e.QType))
	sum := h.Sum(nil)
	return binary.LittleEndian.Uint64(sum[:8])
}
