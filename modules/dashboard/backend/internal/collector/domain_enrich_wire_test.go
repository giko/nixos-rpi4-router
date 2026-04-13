package collector

import (
	"context"
	"encoding/json"
	"net/netip"
	"testing"
	"time"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/adguard"
)

// TestEnricherFedByIngest wires a DnsIngest collector to a DomainEnricher
// via the OnEntry callback and verifies that an answer record flowing
// through ingest becomes resolvable by (client, remote) lookup on the
// enricher. This is the end-to-end integration check for Mega-Phase B.
func TestEnricherFedByIngest(t *testing.T) {
	now := time.Date(2026, 4, 13, 15, 0, 0, 0, time.UTC)
	entryJSON, err := json.Marshal([]map[string]interface{}{{
		"time":   now.Format(time.RFC3339Nano),
		"client": "192.168.1.42",
		"question": map[string]interface{}{
			"name": "cdn.netflix.com",
			"type": "A",
		},
		"answer": []map[string]interface{}{{
			"value": "52.84.17.12",
			"ttl":   float64(60),
		}},
		"reason": "NotFiltered",
	}})
	if err != nil {
		t.Fatalf("marshal fixture: %v", err)
	}

	fake := &fakeAdguardClient{
		responses: []adguard.QueryLogResponse{{
			Data:   entryJSON,
			Oldest: "",
		}},
	}

	enr := NewDomainEnricher(EnricherOpts{})
	ingest := NewDnsIngest(DnsIngestOpts{
		Adguard: fake,
		OnEntry: func(ie IngestedEntry) {
			for _, a := range ie.Answers {
				enr.Record(ie.ClientIP, a.IP, ie.Question,
					time.Duration(a.TTL)*time.Second, ie.Time)
			}
		},
	})
	if err := ingest.Tick(context.Background(), now); err != nil {
		t.Fatalf("Tick: %v", err)
	}

	got, ok := enr.Lookup(
		netip.MustParseAddr("192.168.1.42"),
		netip.MustParseAddr("52.84.17.12"),
		now,
	)
	if !ok || got != "cdn.netflix.com" {
		t.Fatalf("Lookup = %q, %v; want cdn.netflix.com, true", got, ok)
	}
}
