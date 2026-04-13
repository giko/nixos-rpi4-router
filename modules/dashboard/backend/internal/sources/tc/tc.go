// Package tc parses `tc -s qdisc show dev <iface>` output for the two
// shaping setups the router uses:
//   - CAKE on WAN egress (eth1)
//   - HTB outer + fq_codel inner on WAN ingress IFB (ifb4eth1)
//
// The output is human-readable text; we look for known keywords and
// pull the adjacent number. Keys we care about are documented at
// https://man7.org/linux/man-pages/man8/tc-cake.8.html and
// https://man7.org/linux/man-pages/man8/tc-fq_codel.8.html.
package tc

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// QdiscStats is the unified shape both parsers emit.
type QdiscStats struct {
	Kind         string `json:"kind"`          // "cake" | "htb+fq_codel"
	BandwidthBps int64  `json:"bandwidth_bps"` // CAKE: parsed from "bandwidth 100Mbit"
	SentBytes    int64  `json:"sent_bytes"`
	SentPackets  int64  `json:"sent_packets"`
	Dropped      int64  `json:"dropped"`
	Overlimits   int64  `json:"overlimits"`
	Requeues     int64  `json:"requeues"`
	BacklogBytes int64  `json:"backlog_bytes"`
	BacklogPkts  int64  `json:"backlog_pkts"`

	// CAKE-only.
	Tins []CAKETin `json:"tins,omitempty"`

	// HTB+fq_codel only.
	NewFlowCount  int64 `json:"new_flow_count,omitempty"`
	OldFlowsLen   int64 `json:"old_flows_len,omitempty"`
	NewFlowsLen   int64 `json:"new_flows_len,omitempty"`
	ECNMark       int64 `json:"ecn_mark,omitempty"`
	DropOverlimit int64 `json:"drop_overlimit,omitempty"`
}

// CAKETin is one CAKE traffic class.
type CAKETin struct {
	Name         string `json:"name"`
	ThreshKbit   int64  `json:"thresh_kbit"`
	TargetUs     int64  `json:"target_us"`
	IntervalUs   int64  `json:"interval_us"`
	PeakDelayUs  int64  `json:"peak_delay_us"`
	AvgDelayUs   int64  `json:"avg_delay_us"`
	BacklogBytes int64  `json:"backlog_bytes"`
	Packets      int64  `json:"packets"`
	Bytes        int64  `json:"bytes"`
	Drops        int64  `json:"drops"`
	Marks        int64  `json:"marks"`
}

// Runner runs `tc` with the given args. Tests inject a fake.
type Runner func(ctx context.Context, args ...string) ([]byte, error)

// DefaultRunner exec-invokes /sbin/tc (or whatever is in PATH).
func DefaultRunner(ctx context.Context, args ...string) ([]byte, error) {
	out, err := exec.CommandContext(ctx, "tc", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("tc %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

// CollectCAKE runs `tc -s qdisc show dev <iface>` and returns the
// CAKE-shaped stats from the first qdisc reported (must be the root).
func CollectCAKE(ctx context.Context, run Runner, iface string) (QdiscStats, error) {
	out, err := run(ctx, "-s", "qdisc", "show", "dev", iface)
	if err != nil {
		return QdiscStats{}, err
	}
	return ParseCAKE(string(out))
}

// CollectHTB runs `tc -s qdisc show dev <iface>` and returns the
// merged HTB+fq_codel stats expected on the ingress IFB.
func CollectHTB(ctx context.Context, run Runner, iface string) (QdiscStats, error) {
	out, err := run(ctx, "-s", "qdisc", "show", "dev", iface)
	if err != nil {
		return QdiscStats{}, err
	}
	return ParseHTB(string(out))
}

// ParseCAKE walks the text output and pulls counters + per-tin stats.
// Only the section belonging to the `qdisc cake` block is consumed —
// some interfaces (notably eth1 here, which doubles as the IFB
// redirect source) report a sibling `qdisc ingress` block whose
// counters would otherwise overwrite CAKE's.
func ParseCAKE(raw string) (QdiscStats, error) {
	q := QdiscStats{Kind: "cake"}
	all := splitLines(raw)
	cakeLines := sliceCakeBlock(all)
	if len(cakeLines) == 0 {
		return QdiscStats{}, fmt.Errorf("tc: no `qdisc cake` block found in CAKE output")
	}
	var sawSent bool
	for _, ln := range cakeLines {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "qdisc cake") {
			q.BandwidthBps = parseBandwidth(t)
			continue
		}
		if strings.HasPrefix(t, "Sent ") {
			q.SentBytes, q.SentPackets, q.Dropped, q.Overlimits, q.Requeues = parseSentLine(t)
			sawSent = true
		}
		if strings.HasPrefix(t, "backlog ") {
			q.BacklogBytes, q.BacklogPkts = parseBacklogLine(t)
		}
	}
	q.Tins = parseCakeTins(cakeLines)
	if !sawSent {
		return QdiscStats{}, fmt.Errorf("tc: no Sent line found in CAKE block")
	}
	return q, nil
}

// sliceCakeBlock returns the contiguous lines from the first
// `qdisc cake` line up to (but not including) the next line whose
// trimmed prefix is `qdisc ` — i.e. the next sibling qdisc. Returns
// the empty slice if no `qdisc cake` is present.
func sliceCakeBlock(lines []string) []string {
	start := -1
	for i, ln := range lines {
		if strings.HasPrefix(strings.TrimSpace(ln), "qdisc cake") {
			start = i
			break
		}
	}
	if start == -1 {
		return nil
	}
	end := len(lines)
	for j := start + 1; j < len(lines); j++ {
		if strings.HasPrefix(strings.TrimSpace(lines[j]), "qdisc ") {
			end = j
			break
		}
	}
	return lines[start:end]
}

// ParseHTB extracts HTB outer + fq_codel inner counters into one
// QdiscStats. Counters live on different qdiscs:
//
//   - bytes / packets / dropped come from the fq_codel LEAF Sent line
//     (it's the authoritative leaf-level delivery counter; HTB's own
//     Sent line counts the same packets but a future HTB direct-class
//     would diverge slightly).
//   - overlimits and requeues come from the HTB OUTER Sent line because
//     fq_codel doesn't rate-limit — its overlimits is structurally zero
//     even when the router is shaping aggressively.
//   - backlog comes from the fq_codel leaf (the leaf qdisc owns the
//     queue we want to surface).
//
// Bounded to the htb block (mirroring ParseCAKE's sliceCakeBlock). The
// IFB device only ships htb+fq_codel today, but a future config that
// adds a sibling qdisc on the same device would otherwise leak its
// counters into ours — same class of bug ParseCAKE was patched for.
func ParseHTB(raw string) (QdiscStats, error) {
	q := QdiscStats{Kind: "htb+fq_codel"}
	all := splitLines(raw)
	htbLines := sliceHTBBlock(all)
	if len(htbLines) == 0 {
		return QdiscStats{}, fmt.Errorf("tc: no `qdisc htb` block found in HTB output")
	}

	var firstSent, lastSent string
	for _, ln := range htbLines {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "Sent ") {
			if firstSent == "" {
				firstSent = t
			}
			lastSent = t
		}
	}
	if lastSent == "" {
		return QdiscStats{}, fmt.Errorf("tc: no Sent line found in HTB block")
	}

	// Leaf-level totals (bytes, packets, dropped) — from fq_codel.
	q.SentBytes, q.SentPackets, q.Dropped, _, _ = parseSentLine(lastSent)
	// Rate-limit signal (overlimits, requeues) — from htb outer if
	// distinct from the leaf, else fall back to whatever lastSent reported.
	if firstSent != "" && firstSent != lastSent {
		_, _, _, q.Overlimits, q.Requeues = parseSentLine(firstSent)
	} else {
		_, _, _, q.Overlimits, q.Requeues = parseSentLine(lastSent)
	}

	for _, ln := range htbLines {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "backlog ") {
			q.BacklogBytes, q.BacklogPkts = parseBacklogLine(t)
		}
		if strings.Contains(t, "new_flow_count") || strings.Contains(t, "ecn_mark") {
			q.NewFlowCount = parseKey(t, "new_flow_count")
			q.ECNMark = parseKey(t, "ecn_mark")
			q.DropOverlimit = parseKey(t, "drop_overlimit")
		}
		if strings.Contains(t, "new_flows_len") || strings.Contains(t, "old_flows_len") {
			q.NewFlowsLen = parseKey(t, "new_flows_len")
			q.OldFlowsLen = parseKey(t, "old_flows_len")
		}
	}
	return q, nil
}

// sliceHTBBlock returns the contiguous lines from the first
// `qdisc htb` line up to (but not including) the next sibling root
// qdisc line. Nested qdiscs (e.g. the fq_codel inside htb) are kept
// because their declaration includes `parent <handle>` rather than
// `root`.
func sliceHTBBlock(lines []string) []string {
	start := -1
	for i, ln := range lines {
		if strings.HasPrefix(strings.TrimSpace(ln), "qdisc htb") {
			start = i
			break
		}
	}
	if start == -1 {
		return nil
	}
	end := len(lines)
	for j := start + 1; j < len(lines); j++ {
		t := strings.TrimSpace(lines[j])
		if strings.HasPrefix(t, "qdisc ") && !strings.Contains(t, " parent ") {
			end = j
			break
		}
	}
	return lines[start:end]
}

// --- helpers ---

func splitLines(s string) []string {
	var out []string
	sc := bufio.NewScanner(strings.NewReader(s))
	for sc.Scan() {
		out = append(out, sc.Text())
	}
	return out
}

// parseSentLine extracts the standard tc summary:
//
//	Sent 42167731517 bytes 89707798 pkt (dropped 5613, overlimits 99439519 requeues 20020)
func parseSentLine(line string) (sentB, sentP, dropped, overlimits, requeues int64) {
	fields := strings.Fields(line)
	for i := 0; i < len(fields); i++ {
		switch fields[i] {
		case "Sent":
			if i+1 < len(fields) {
				sentB, _ = strconv.ParseInt(fields[i+1], 10, 64)
			}
		case "bytes":
			if i+1 < len(fields) {
				sentP, _ = strconv.ParseInt(fields[i+1], 10, 64)
			}
		case "(dropped":
			if i+1 < len(fields) {
				dropped, _ = strconv.ParseInt(strings.TrimSuffix(fields[i+1], ","), 10, 64)
			}
		case "overlimits":
			if i+1 < len(fields) {
				overlimits, _ = strconv.ParseInt(fields[i+1], 10, 64)
			}
		case "requeues":
			if i+1 < len(fields) {
				requeues, _ = strconv.ParseInt(strings.TrimSuffix(fields[i+1], ")"), 10, 64)
			}
		}
	}
	return
}

func parseBacklogLine(line string) (bytes, pkts int64) {
	// backlog 0b 0p requeues 0
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return
	}
	bytes, _ = strconv.ParseInt(strings.TrimSuffix(fields[1], "b"), 10, 64)
	pkts, _ = strconv.ParseInt(strings.TrimSuffix(fields[2], "p"), 10, 64)
	return
}

// parseBandwidth pulls "bandwidth NNNMbit" / "NNNKbit" / "NNNGbit" from
// CAKE's qdisc line and returns bits per second.
func parseBandwidth(line string) int64 {
	fields := strings.Fields(line)
	for i, f := range fields {
		if f != "bandwidth" || i+1 >= len(fields) {
			continue
		}
		v := fields[i+1]
		mul := int64(1)
		switch {
		case strings.HasSuffix(v, "Gbit"):
			mul = 1_000_000_000
			v = strings.TrimSuffix(v, "Gbit")
		case strings.HasSuffix(v, "Mbit"):
			mul = 1_000_000
			v = strings.TrimSuffix(v, "Mbit")
		case strings.HasSuffix(v, "Kbit"):
			mul = 1_000
			v = strings.TrimSuffix(v, "Kbit")
		case strings.HasSuffix(v, "bit"):
			v = strings.TrimSuffix(v, "bit")
		}
		n, _ := strconv.ParseInt(v, 10, 64)
		return n * mul
	}
	return 0
}

// parseKey returns the int64 that follows the given key on a tc stat
// line like "  maxpacket 68130 drop_overlimit 0 new_flow_count 9664721 ecn_mark 831".
func parseKey(line, key string) int64 {
	fields := strings.Fields(line)
	for i, f := range fields {
		if f == key && i+1 < len(fields) {
			n, _ := strconv.ParseInt(fields[i+1], 10, 64)
			return n
		}
	}
	return 0
}

// parseCakeTins finds the CAKE per-tin column block. The header row
// names the tins (typically "Bulk Best Effort Voice" for diffserv3),
// then each subsequent row is one stat across the tins. We walk down,
// grabbing the rows we care about.
func parseCakeTins(lines []string) []CAKETin {
	header := -1
	for i, ln := range lines {
		if strings.Contains(ln, "Bulk") && strings.Contains(ln, "Best Effort") {
			header = i
			break
		}
	}
	if header == -1 {
		return nil
	}
	names := strings.Fields(lines[header])
	// "Bulk Best Effort Voice" — note "Best Effort" is 2 fields, so we
	// rejoin: the line has exactly 4 tokens with diffserv3.
	// TODO: this rejoin only handles CAKE diffserv3; diffserv4 (4 tins:
	// Bulk / Best Effort / Video / Voice) would need a different rejoin.
	if len(names) == 4 && names[1] == "Best" && names[2] == "Effort" {
		names = []string{"Bulk", "Best Effort", "Voice"}
	}
	tins := make([]CAKETin, len(names))
	for i := range tins {
		tins[i].Name = names[i]
	}
	for j := header + 1; j < len(lines); j++ {
		row := strings.Fields(lines[j])
		if len(row) == 0 {
			break
		}
		key := row[0]
		vals := row[1:]
		if len(vals) != len(tins) {
			continue
		}
		for i, v := range vals {
			n := parseTinValue(v)
			switch key {
			case "thresh":
				tins[i].ThreshKbit = n
			case "target":
				tins[i].TargetUs = n
			case "interval":
				tins[i].IntervalUs = n
			case "pk_delay":
				tins[i].PeakDelayUs = n
			case "av_delay":
				tins[i].AvgDelayUs = n
			case "backlog":
				tins[i].BacklogBytes = n
			case "pkts":
				tins[i].Packets = n
			case "bytes":
				tins[i].Bytes = n
			case "drops":
				tins[i].Drops = n
			case "marks":
				tins[i].Marks = n
			}
		}
	}
	return tins
}

// parseTinValue accepts numbers with units like "5ms", "100ms", "6250Kbit",
// "5002752b", "0", "20.7ms" and returns a microsecond/byte/kbit/raw value
// depending on the suffix; the caller knows which key it asked for.
// We normalise:
//
//	*ms      -> microseconds (for delay/interval/target rows)
//	*us      -> microseconds
//	*Kbit    -> kbit
//	*Mbit    -> kbit (×1000)
//	*Gbit    -> kbit (×1_000_000)
//	*b       -> bytes
//	bare num -> int (packets/drops/marks)
func parseTinValue(v string) int64 {
	if v == "0" {
		return 0
	}
	switch {
	case strings.HasSuffix(v, "us"):
		n, _ := strconv.ParseFloat(strings.TrimSuffix(v, "us"), 64)
		return int64(n)
	case strings.HasSuffix(v, "ms"):
		n, _ := strconv.ParseFloat(strings.TrimSuffix(v, "ms"), 64)
		return int64(n * 1000)
	case strings.HasSuffix(v, "Gbit"):
		n, _ := strconv.ParseFloat(strings.TrimSuffix(v, "Gbit"), 64)
		return int64(n * 1_000_000)
	case strings.HasSuffix(v, "Mbit"):
		n, _ := strconv.ParseFloat(strings.TrimSuffix(v, "Mbit"), 64)
		return int64(n * 1_000)
	case strings.HasSuffix(v, "Kbit"):
		n, _ := strconv.ParseInt(strings.TrimSuffix(v, "Kbit"), 10, 64)
		return n
	case strings.HasSuffix(v, "b"):
		n, _ := strconv.ParseInt(strings.TrimSuffix(v, "b"), 10, 64)
		return n
	}
	n, _ := strconv.ParseInt(v, 10, 64)
	return n
}
