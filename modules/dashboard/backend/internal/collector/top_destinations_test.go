package collector

import (
	"net/netip"
	"testing"
	"time"
)

func TestTopDestDecaysAfter60Minutes(t *testing.T) {
	td := NewTopDestinations(TopDestOpts{})
	client := netip.MustParseAddr("192.168.1.42")
	td.Track(client)
	t0 := time.Date(2026, 4, 13, 14, 0, 0, 0, time.UTC)
	td.RecordQuery(client, "netflix.com", false, t0)
	td.RecordBytes(client, "netflix.com", 100_000_000, t0)
	td.Advance(t0.Add(59 * time.Minute))
	if dests := td.Snapshot(client); len(dests) != 1 || dests[0].Queries != 1 {
		t.Fatalf("after 59min: %+v", dests)
	}
	td.Advance(t0.Add(61 * time.Minute))
	if dests := td.Snapshot(client); len(dests) != 0 {
		t.Fatalf("after 61min: expected decayed, got %+v", dests)
	}
}

func TestTopDestGroupsBySecondLevel(t *testing.T) {
	td := NewTopDestinations(TopDestOpts{})
	client := netip.MustParseAddr("192.168.1.42")
	td.Track(client)
	now := time.Date(2026, 4, 13, 14, 0, 0, 0, time.UTC)
	td.RecordQuery(client, "cdn.netflix.com", false, now)
	td.RecordQuery(client, "api.netflix.com", false, now)
	td.RecordQuery(client, "unrelated.example.com", false, now)
	dests := td.Snapshot(client)
	byName := map[string]uint64{}
	for _, d := range dests {
		byName[d.Domain] = d.Queries
	}
	if byName["netflix.com"] != 2 {
		t.Errorf("netflix.com queries = %d, want 2", byName["netflix.com"])
	}
	if byName["example.com"] != 1 {
		t.Errorf("example.com queries = %d, want 1", byName["example.com"])
	}
}

func TestTopDestBlockedWithoutBytesStillListed(t *testing.T) {
	td := NewTopDestinations(TopDestOpts{})
	client := netip.MustParseAddr("192.168.1.42")
	td.Track(client)
	now := time.Date(2026, 4, 13, 14, 0, 0, 0, time.UTC)
	for i := 0; i < 47; i++ {
		td.RecordQuery(client, "doubleclick.net", true, now)
	}
	dests := td.Snapshot(client)
	if len(dests) != 1 {
		t.Fatalf("want 1, got %d", len(dests))
	}
	if dests[0].Blocked != 47 || dests[0].Bytes != 0 {
		t.Errorf("got %+v", dests[0])
	}
}

func TestTopDestSortByBytesThenBlockedThenQueries(t *testing.T) {
	td := NewTopDestinations(TopDestOpts{})
	client := netip.MustParseAddr("192.168.1.42")
	td.Track(client)
	now := time.Date(2026, 4, 13, 14, 0, 0, 0, time.UTC)
	td.RecordBytes(client, "heavy.com", 1_000_000, now)
	td.RecordQuery(client, "heavy.com", false, now)
	td.RecordQuery(client, "spammy.com", true, now)
	td.RecordQuery(client, "spammy.com", true, now)
	td.RecordQuery(client, "chatty.com", false, now)
	dests := td.Snapshot(client)
	if len(dests) != 3 {
		t.Fatalf("want 3 domains, got %+v", dests)
	}
	if dests[0].Domain != "heavy.com" {
		t.Errorf("first = %s, want heavy.com (bytes desc)", dests[0].Domain)
	}
	if dests[1].Domain != "spammy.com" {
		t.Errorf("second = %s, want spammy.com (blocked desc after bytes tie at 0)", dests[1].Domain)
	}
}
