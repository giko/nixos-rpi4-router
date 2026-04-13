package collector

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/adguard"
)

// TestDnsIngestDedupsOverlap walks the three fixture pages and verifies
// that every timestamp reaches OnEntry exactly once.
//
// Fixture layout (see testdata/generate.go):
//
//	page 1: t=1000..801  (200 entries)
//	page 2: t= 830..631  (200 entries, 30s overlap with page 1)
//	page 3: t= 660..511  (150 entries, 30s overlap with page 2)
//
// Total raw rows served: 550. Unique timestamps (== expected OnEntry
// calls): 1000-511+1 = 490. Duplicates to be suppressed: 60.
func TestDnsIngestDedupsOverlap(t *testing.T) {
	var (
		mu       sync.Mutex
		requests int
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requests++
		mu.Unlock()

		older := r.URL.Query().Get("older_than")
		switch {
		case older == "":
			http.ServeFile(w, r, "testdata/querylog_page1.json")
		case older > "2026-04-13T14:12:00Z":
			// Page 1's oldest is 14:13:21Z -> serves page 2.
			http.ServeFile(w, r, "testdata/querylog_page2.json")
		default:
			// Page 2's oldest is 14:10:31Z -> serves page 3.
			http.ServeFile(w, r, "testdata/querylog_page3.json")
		}
	}))
	defer srv.Close()

	seen := make(map[string]int)
	ingested := 0
	onEntry := func(e IngestedEntry) {
		key := e.Time.UTC().Format(time.RFC3339Nano) + "|" +
			e.ClientIP.String() + "|" + e.Question
		seen[key]++
		ingested++
	}

	col := NewDnsIngest(DnsIngestOpts{
		Adguard:  adguard.NewClient(srv.URL, srv.Client()),
		OnEntry:  onEntry,
		PageSize: 200,
	})
	if err := col.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	const wantUnique = 490
	if ingested != wantUnique {
		t.Errorf("ingested %d entries, want %d", ingested, wantUnique)
	}
	if len(seen) != wantUnique {
		t.Errorf("unique keys = %d, want %d", len(seen), wantUnique)
	}
	for key, count := range seen {
		if count != 1 {
			t.Errorf("duplicate ingested: key=%s count=%d", key, count)
		}
	}
	if requests != 3 {
		t.Errorf("HTTP requests = %d, want 3", requests)
	}
}

// TestDnsIngestDedupKeyIncludesQType asserts that two entries sharing
// (time, client, question) but differing on QType both survive dedup.
// A dual-stack client issuing A and AAAA for the same name within a
// single sub-second window must not collapse into one delivered entry.
func TestDnsIngestDedupKeyIncludesQType(t *testing.T) {
	now := time.Date(2026, 4, 13, 15, 0, 0, 0, time.UTC)
	entries := []map[string]interface{}{
		{
			"time":     now.Format(time.RFC3339Nano),
			"client":   "192.168.1.42",
			"question": map[string]interface{}{"name": "example.com", "type": "A"},
			"answer":   []map[string]interface{}{},
			"reason":   "NotFiltered",
		},
		{
			"time":     now.Format(time.RFC3339Nano),
			"client":   "192.168.1.42",
			"question": map[string]interface{}{"name": "example.com", "type": "AAAA"},
			"answer":   []map[string]interface{}{},
			"reason":   "NotFiltered",
		},
	}
	raw, _ := json.Marshal(entries)
	fake := &fakeAdguardClient{responses: []adguard.QueryLogResponse{{Data: raw, Oldest: ""}}}

	var gotTypes []string
	col := NewDnsIngest(DnsIngestOpts{
		Adguard: fake,
		OnEntry: func(e IngestedEntry) { gotTypes = append(gotTypes, e.QType) },
	})
	if err := col.Tick(context.Background(), now); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if len(gotTypes) != 2 {
		t.Fatalf("expected both A and AAAA, got %v", gotTypes)
	}
	// Order is whatever the fixture returns; just check the set.
	saw := map[string]bool{}
	for _, q := range gotTypes {
		saw[q] = true
	}
	if !saw["A"] || !saw["AAAA"] {
		t.Errorf("saw = %v, want both A and AAAA", saw)
	}
}

// TestDnsIngestSecondTickSkipsWatermark confirms that a subsequent Tick
// with a stable fixture set does not re-deliver entries already behind
// the watermark minus overlap.
func TestDnsIngestSecondTickSkipsWatermark(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		older := r.URL.Query().Get("older_than")
		switch {
		case older == "":
			http.ServeFile(w, r, "testdata/querylog_page1.json")
		case older > "2026-04-13T14:12:00Z":
			http.ServeFile(w, r, "testdata/querylog_page2.json")
		default:
			http.ServeFile(w, r, "testdata/querylog_page3.json")
		}
	}))
	defer srv.Close()

	var firstCount, secondCount int
	counter := &firstCount
	onEntry := func(IngestedEntry) { *counter++ }

	col := NewDnsIngest(DnsIngestOpts{
		Adguard:  adguard.NewClient(srv.URL, srv.Client()),
		OnEntry:  onEntry,
		PageSize: 200,
	})

	if err := col.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick #1: %v", err)
	}
	if firstCount != 490 {
		t.Fatalf("first Tick ingested %d, want 490", firstCount)
	}

	counter = &secondCount
	if err := col.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick #2: %v", err)
	}
	if secondCount != 0 {
		t.Errorf("second Tick ingested %d, want 0 (watermark+dedup should suppress everything)", secondCount)
	}
}

// fakeAdguardClient lets us drive DnsIngest without an HTTP server.
type fakeAdguardClient struct {
	responses []adguard.QueryLogResponse
	calls     int
	lastOlder []time.Time
	err       error
}

func (f *fakeAdguardClient) FetchQueryLogPage(_ context.Context, olderThan time.Time, _ int) (adguard.QueryLogResponse, error) {
	if f.err != nil {
		return adguard.QueryLogResponse{}, f.err
	}
	f.lastOlder = append(f.lastOlder, olderThan)
	if f.calls >= len(f.responses) {
		return adguard.QueryLogResponse{}, nil
	}
	resp := f.responses[f.calls]
	f.calls++
	return resp, nil
}

// TestDnsIngestStopsOnEmptyPage confirms the loop terminates when the
// upstream returns an empty Data array (no rows).
func TestDnsIngestStopsOnEmptyPage(t *testing.T) {
	fake := &fakeAdguardClient{
		responses: []adguard.QueryLogResponse{
			{Oldest: "", Data: nil},
		},
	}
	col := NewDnsIngest(DnsIngestOpts{
		Adguard:  fake,
		OnEntry:  func(IngestedEntry) { t.Fatal("OnEntry called on empty page") },
		PageSize: 100,
	})
	if err := col.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if fake.calls != 1 {
		t.Errorf("calls = %d, want 1 (empty page should terminate immediately)", fake.calls)
	}
}

// TestDnsIngestStopsOnShortPage confirms the loop terminates when a page
// contains fewer rows than PageSize even with no explicit cutoff break.
func TestDnsIngestStopsOnShortPage(t *testing.T) {
	body := []byte(`[{"time":"2026-04-13T14:00:00Z","client":"192.168.1.1","question":{"name":"a.example","type":"A"},"answer":[],"reason":"NotFiltered"}]`)
	fake := &fakeAdguardClient{
		responses: []adguard.QueryLogResponse{
			{Oldest: "2026-04-13T14:00:00Z", Data: body},
		},
	}
	var got int
	col := NewDnsIngest(DnsIngestOpts{
		Adguard:  fake,
		OnEntry:  func(IngestedEntry) { got++ },
		PageSize: 100,
	})
	if err := col.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if got != 1 {
		t.Errorf("ingested = %d, want 1", got)
	}
	if fake.calls != 1 {
		t.Errorf("calls = %d, want 1 (short page should end pagination)", fake.calls)
	}
}

// TestDnsIngestBlockedClassification exhaustively walks AdGuard's
// Reason taxonomy to pin the Blocked predicate: every reason that
// actually results in a filter action starts with "Filtered", while
// NotFiltered* and Rewrite* reasons are allowed passes. FilteredSafeSearch
// is a rewrite (to the safe-search variant) rather than a hard drop, but
// the frontend AdGuard page classifies it as blocked via the same
// startsWith("Filtered") predicate, so the backend mirrors that behavior.
func TestDnsIngestBlockedClassification(t *testing.T) {
	now := time.Date(2026, 4, 13, 15, 0, 0, 0, time.UTC)
	cases := []struct {
		reason  string
		blocked bool
	}{
		{"NotFiltered", false},
		{"NotFilteredNotFound", false},
		{"NotFilteredWhiteList", false},
		{"ReasonRewrite", false},
		{"RewriteEtcHosts", false},
		{"RewriteRule", false},
		{"FilteredBlackList", true},
		{"FilteredSafeBrowsing", true},
		{"FilteredParental", true},
		{"FilteredBlockedService", true},
		{"FilteredSafeSearch", true},
		{"FilteredInvalid", true},
	}
	for _, c := range cases {
		t.Run(c.reason, func(t *testing.T) {
			row := map[string]interface{}{
				"time":     now.Format(time.RFC3339Nano),
				"client":   "192.168.1.42",
				"question": map[string]interface{}{"name": "example.com", "type": "A"},
				"answer":   []map[string]interface{}{},
				"reason":   c.reason,
			}
			raw, _ := json.Marshal([]interface{}{row})
			fake := &fakeAdguardClient{responses: []adguard.QueryLogResponse{{Data: raw, Oldest: ""}}}
			var got bool
			col := NewDnsIngest(DnsIngestOpts{
				Adguard: fake,
				OnEntry: func(e IngestedEntry) { got = e.Blocked },
			})
			if err := col.Tick(context.Background(), now); err != nil {
				t.Fatalf("Tick: %v", err)
			}
			if got != c.blocked {
				t.Errorf("reason=%s got Blocked=%v, want %v", c.reason, got, c.blocked)
			}
		})
	}
}

// TestDnsIngestMarksBlocked asserts the Blocked flag follows AdGuard's
// Reason field: anything prefixed with "Filtered" is blocked; everything
// else (including empty and NotFiltered) is allowed.
func TestDnsIngestMarksBlocked(t *testing.T) {
	body := []byte(`[
		{"time":"2026-04-13T14:00:02Z","client":"192.168.1.1","question":{"name":"ok.example","type":"A"},"answer":[],"reason":"NotFiltered"},
		{"time":"2026-04-13T14:00:01Z","client":"192.168.1.1","question":{"name":"blocked.example","type":"A"},"answer":[],"reason":"FilteredBlackList"},
		{"time":"2026-04-13T14:00:00Z","client":"192.168.1.1","question":{"name":"plain.example","type":"A"},"answer":[],"reason":""}
	]`)
	fake := &fakeAdguardClient{
		responses: []adguard.QueryLogResponse{
			{Oldest: "2026-04-13T14:00:00Z", Data: body},
		},
	}
	var entries []IngestedEntry
	col := NewDnsIngest(DnsIngestOpts{
		Adguard:  fake,
		OnEntry:  func(e IngestedEntry) { entries = append(entries, e) },
		PageSize: 100,
	})
	if err := col.Tick(context.Background(), time.Now()); err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("entries = %d, want 3", len(entries))
	}
	wantBlocked := map[string]bool{
		"ok.example":      false,
		"blocked.example": true,
		"plain.example":   false,
	}
	for _, e := range entries {
		want, ok := wantBlocked[e.Question]
		if !ok {
			t.Errorf("unexpected question %q", e.Question)
			continue
		}
		if e.Blocked != want {
			t.Errorf("%s: Blocked=%v want %v", e.Question, e.Blocked, want)
		}
	}
}
