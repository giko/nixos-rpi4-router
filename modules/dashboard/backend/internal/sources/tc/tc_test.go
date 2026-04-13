package tc

import (
	"os"
	"testing"
)

func TestParseCakeFromFixture(t *testing.T) {
	raw, err := os.ReadFile("testdata/cake_egress.txt")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	q, err := ParseCAKE(string(raw))
	if err != nil {
		t.Fatalf("ParseCAKE: %v", err)
	}
	if q.Kind != "cake" {
		t.Errorf("Kind = %q, want cake", q.Kind)
	}
	if q.SentBytes <= 0 {
		t.Errorf("SentBytes = %d, want > 0", q.SentBytes)
	}
	if q.SentPackets <= 0 {
		t.Errorf("SentPackets = %d, want > 0", q.SentPackets)
	}
	// The cake fixture also contains a sibling `qdisc ingress` whose
	// Sent counters are much higher (it sees all packets the IFB
	// redirected). If our SentBytes lands above 100 GB we know we
	// accidentally consumed the ingress block too.
	if q.SentBytes >= 100_000_000_000 {
		t.Errorf("SentBytes = %d; looks like ingress block leaked into CAKE parse (cake root should be ~42 GB)", q.SentBytes)
	}
	if q.BandwidthBps != 100_000_000 {
		t.Errorf("BandwidthBps = %d, want 100000000 (100 Mbit from fixture)", q.BandwidthBps)
	}
	if len(q.Tins) != 3 {
		t.Errorf("Tin count = %d, want 3 (Bulk/Best Effort/Voice)", len(q.Tins))
	}
	for _, tin := range q.Tins {
		if tin.Name == "" {
			t.Errorf("tin missing name: %+v", tin)
		}
	}
}

func TestParseHTBFromFixture(t *testing.T) {
	raw, err := os.ReadFile("testdata/htb_ingress.txt")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	q, err := ParseHTB(string(raw))
	if err != nil {
		t.Fatalf("ParseHTB: %v", err)
	}
	if q.Kind != "htb+fq_codel" {
		t.Errorf("Kind = %q, want htb+fq_codel", q.Kind)
	}
	if q.SentBytes <= 0 {
		t.Errorf("SentBytes = %d, want > 0", q.SentBytes)
	}
	// The HTB fixture's fq_codel block should populate ECN-mark + new-flow stats.
	if q.NewFlowCount == 0 {
		t.Errorf("NewFlowCount = 0; expected populated from fq_codel")
	}
}
