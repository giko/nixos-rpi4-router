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
	// The live fixture's htb-outer line reports overlimits ≈ 32M while
	// the fq_codel leaf shows 0. If we read 0 here we know we accidentally
	// pulled overlimits from the leaf instead of the outer.
	if q.Overlimits == 0 {
		t.Errorf("Overlimits = 0; expected populated from htb outer Sent line, not fq_codel leaf")
	}
}

func TestParseCakeMissingCakeBlock(t *testing.T) {
	// eth0-style output (mq + fq_codel, no cake) must produce a clear
	// error so the QoS collector can log it instead of silently emitting
	// zeros.
	raw := `qdisc mq 0: root
qdisc fq_codel 0: parent :1 limit 10240p flows 1024
 Sent 100 bytes 5 pkt (dropped 0, overlimits 0 requeues 0)
 backlog 0b 0p requeues 0
`
	if _, err := ParseCAKE(raw); err == nil {
		t.Fatal("ParseCAKE on non-cake output should error")
	}
}

func TestParseCakeEmpty(t *testing.T) {
	if _, err := ParseCAKE(""); err == nil {
		t.Fatal("ParseCAKE on empty input should error")
	}
}

func TestParseHTBMissingHTBBlock(t *testing.T) {
	raw := `qdisc fq_codel 0: parent :1 limit 10240p flows 1024
 Sent 100 bytes 5 pkt (dropped 0, overlimits 0 requeues 0)
`
	if _, err := ParseHTB(raw); err == nil {
		t.Fatal("ParseHTB on non-htb output should error")
	}
}

func TestParseHTBEmpty(t *testing.T) {
	if _, err := ParseHTB(""); err == nil {
		t.Fatal("ParseHTB on empty input should error")
	}
}

func TestParseCakeZeroTrafficStillReturnsStats(t *testing.T) {
	// Idle/freshly-booted router: cake reports `Sent 0 bytes 0 pkt`.
	// This must NOT be treated as a missing-Sent error.
	raw := `qdisc cake 8003: root refcnt 2 bandwidth 100Mbit diffserv3 triple-isolate
 Sent 0 bytes 0 pkt (dropped 0, overlimits 0 requeues 0)
 backlog 0b 0p requeues 0
`
	q, err := ParseCAKE(raw)
	if err != nil {
		t.Fatalf("ParseCAKE on zero-traffic output errored: %v", err)
	}
	if q.Kind != "cake" {
		t.Errorf("Kind = %q, want cake", q.Kind)
	}
	if q.SentBytes != 0 || q.SentPackets != 0 {
		t.Errorf("SentBytes/SentPackets = %d/%d, want 0/0", q.SentBytes, q.SentPackets)
	}
	if q.BandwidthBps != 100_000_000 {
		t.Errorf("BandwidthBps = %d, want 100000000", q.BandwidthBps)
	}
}

func TestParseHTBOverlimitsFromOuterQdisc(t *testing.T) {
	// HTB-outer Sent line carries the rate-limit signal (overlimits);
	// the fq_codel leaf line always reports overlimits=0 because
	// fq_codel doesn't shape. Verify ParseHTB pulls overlimits from htb
	// while keeping bytes/packets/dropped from the leaf.
	raw := `qdisc htb 1: root refcnt 2 r2q 10 default 0x1
 Sent 999 bytes 9 pkt (dropped 1, overlimits 12345 requeues 7)
 backlog 0b 0p requeues 0
qdisc fq_codel 8004: parent 1:1 limit 10240p flows 1024 quantum 1514
 Sent 999 bytes 9 pkt (dropped 1, overlimits 0 requeues 0)
 backlog 0b 0p requeues 0
  maxpacket 0 drop_overlimit 0 new_flow_count 3 ecn_mark 0
  new_flows_len 0 old_flows_len 0
`
	q, err := ParseHTB(raw)
	if err != nil {
		t.Fatalf("ParseHTB: %v", err)
	}
	if q.SentBytes != 999 || q.SentPackets != 9 || q.Dropped != 1 {
		t.Errorf("leaf totals: bytes=%d packets=%d dropped=%d, want 999/9/1", q.SentBytes, q.SentPackets, q.Dropped)
	}
	if q.Overlimits != 12345 {
		t.Errorf("Overlimits = %d, want 12345 (from htb outer)", q.Overlimits)
	}
	if q.Requeues != 7 {
		t.Errorf("Requeues = %d, want 7 (from htb outer)", q.Requeues)
	}
}

func TestParseHTBMissingFqCodelLeaf(t *testing.T) {
	// htb-only output (fq_codel leaf failed to attach) — must error,
	// not silently return zeroed leaf stats and mask a broken shaper.
	raw := `qdisc htb 1: root refcnt 2 r2q 10 default 0x1
 Sent 999 bytes 9 pkt (dropped 0, overlimits 0 requeues 0)
 backlog 0b 0p requeues 0
`
	if _, err := ParseHTB(raw); err == nil {
		t.Fatal("ParseHTB on htb-without-fq_codel input should error")
	}
}

func TestParseHTBHealthyWithIdenticalSentLines(t *testing.T) {
	// Healthy ingress with no rate-limiting — htb outer and fq_codel
	// leaf legitimately report identical Sent text. Leaf presence must
	// be detected from the qdisc structure, not the Sent string.
	raw := `qdisc htb 1: root refcnt 2 r2q 10 default 0x1
 Sent 100 bytes 1 pkt (dropped 0, overlimits 0 requeues 0)
 backlog 0b 0p requeues 0
qdisc fq_codel 8004: parent 1:1 limit 10240p flows 1024
 Sent 100 bytes 1 pkt (dropped 0, overlimits 0 requeues 0)
 backlog 0b 0p requeues 0
  maxpacket 0 drop_overlimit 0 new_flow_count 1 ecn_mark 0
  new_flows_len 0 old_flows_len 0
`
	q, err := ParseHTB(raw)
	if err != nil {
		t.Fatalf("ParseHTB on healthy identical-Sent output errored: %v", err)
	}
	if q.SentBytes != 100 || q.SentPackets != 1 {
		t.Errorf("Sent totals = %d/%d, want 100/1", q.SentBytes, q.SentPackets)
	}
	if q.NewFlowCount != 1 {
		t.Errorf("NewFlowCount = %d, want 1 (proves we read the leaf, not just the outer)", q.NewFlowCount)
	}
}

func TestParseBacklogScaledUnits(t *testing.T) {
	cases := []struct {
		name        string
		line        string
		wantBytes   int64
		wantPackets int64
	}{
		{"raw bytes", "backlog 1500b 1p requeues 0", 1500, 1},
		{"kibibytes", "backlog 12Kb 8p requeues 0", 12 * 1024, 8},
		{"mebibytes", "backlog 3Mb 2000p requeues 0", 3 * 1024 * 1024, 2000},
		{"zero", "backlog 0b 0p requeues 0", 0, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b, p := parseBacklogLine(c.line)
			if b != c.wantBytes || p != c.wantPackets {
				t.Errorf("parseBacklogLine(%q) = (%d, %d), want (%d, %d)", c.line, b, p, c.wantBytes, c.wantPackets)
			}
		})
	}
}

func TestParseTinValueScaledBytes(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"0", 0},
		{"1500b", 1500},
		{"12Kb", 12 * 1024},
		{"3Mb", 3 * 1024 * 1024},
		{"1Gb", 1024 * 1024 * 1024},
	}
	for _, c := range cases {
		if got := parseTinValue(c.in); got != c.want {
			t.Errorf("parseTinValue(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}
