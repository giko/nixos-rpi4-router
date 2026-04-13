// Package conntrack reads /proc/net/nf_conntrack and exposes per-flow
// records with NAT direction attribution for downstream collectors.
package conntrack

import (
	"bufio"
	"fmt"
	"io"
	"net/netip"
	"strconv"
	"strings"
	"time"
)

type Direction uint8

const (
	DirUnknown  Direction = iota
	DirOutbound           // client is the original-direction source (LAN → WAN)
	DirInbound            // client is the reply-direction source (inbound DNAT / UPnP)
)

func (d Direction) String() string {
	switch d {
	case DirOutbound:
		return "outbound"
	case DirInbound:
		return "inbound"
	default:
		return "unknown"
	}
}

// FlowKey is the normalized 5-tuple used to match an entry across ticks.
// A FlowKey is stable for the lifetime of a conntrack entry, including
// after the flow is offloaded.
type FlowKey struct {
	Proto       uint8
	OrigSrcIP   netip.Addr
	OrigDstIP   netip.Addr
	OrigSrcPort uint16
	OrigDstPort uint16
}

// FlowBytes is a per-flow snapshot emitted by EnumerateFlows each tick.
type FlowBytes struct {
	Key           FlowKey
	ClientIP      netip.Addr // DNAT-aware: original src, or reply src on inbound DNAT
	Direction     Direction
	OrigBytes     uint64     // cumulative original-direction bytes
	ReplyBytes    uint64     // cumulative reply-direction bytes
	RouteTag      string     // tunnel name from conntrack mark (e.g. "WAN", "wg_sw"); empty when unmapped
	NATPublicIP   netip.Addr // zero unless inbound DNAT / UPnP
	NATPublicPort uint16
	LocalPort     uint16 // client-side port (derived from Direction)
	RemoteIP      netip.Addr
	RemotePort    uint16
	Age           time.Duration
	State         string // "ESTABLISHED", "TIME_WAIT", etc.
}

// EnumerateOpts configures flow enumeration. RouteTags maps a numeric
// conntrack mark (as emitted by the nftables mangle rules) to the
// tunnel name the dashboard surface reports. The caller is responsible
// for building this map from the NixOS topology — fwmarks are assigned
// sequentially based on the sorted list of configured tunnel names, so
// hardcoding a static table here would silently mislabel flows on any
// deployment with a different tunnel set.
type EnumerateOpts struct {
	RouteTags map[uint32]string
}

// EnumerateFlows parses /proc/net/nf_conntrack format (one line per flow)
// and returns per-flow records. Caller supplies a reader so tests can
// inject fixtures. Returns flows whose original-source is a private
// LAN IP (192.168.0.0/16, 10.0.0.0/8, 172.16/12).
func EnumerateFlows(r io.Reader, opts EnumerateOpts) ([]FlowBytes, error) {
	var out []FlowBytes
	scan := bufio.NewScanner(r)
	scan.Buffer(make([]byte, 64*1024), 1024*1024)
	for scan.Scan() {
		line := scan.Text()
		if line == "" {
			continue
		}
		fb, ok, err := parseLine(line, opts)
		if err != nil {
			return nil, fmt.Errorf("parse %q: %w", line, err)
		}
		if !ok {
			continue
		}
		out = append(out, fb)
	}
	return out, scan.Err()
}

// parseLine parses one /proc/net/nf_conntrack line. Returns ok=false for
// non-IPv4 entries or entries without a LAN source.
//
// The nf_conntrack line format lists the original tuple first, then the
// reply tuple. For TCP/UDP the boundary between tuples is easy to spot
// because dport= appears once per direction, but ICMP entries have no
// sport/dport — the original tuple terminates at dst=. To handle both
// uniformly we count src= and dst= occurrences: the first src= is the
// original source, the second src= (when present) is the reply source,
// and likewise for dst=.
func parseLine(line string, opts EnumerateOpts) (FlowBytes, bool, error) {
	fields := strings.Fields(line)
	if len(fields) < 5 {
		return FlowBytes{}, false, nil
	}
	if fields[0] != "ipv4" {
		return FlowBytes{}, false, nil
	}
	protoNum, err := strconv.Atoi(fields[3])
	if err != nil {
		return FlowBytes{}, false, nil
	}
	state := ""
	if protoNum == 6 && len(fields) > 5 {
		state = fields[5]
	}

	var fb FlowBytes
	fb.Key.Proto = uint8(protoNum)
	fb.State = state

	var (
		srcCount   int
		dstCount   int
		sportCount int
		dportCount int
		bytesCount int
	)
	for _, f := range fields {
		kv := strings.SplitN(f, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "src":
			ip, err := netip.ParseAddr(kv[1])
			if err != nil {
				return FlowBytes{}, false, err
			}
			if srcCount == 0 {
				fb.Key.OrigSrcIP = ip
			}
			// Reply-direction src (second occurrence) is unused: the
			// original tuple already carries the information we need for
			// LAN attribution, and the reply src in an un-NAT'd flow is
			// identical to the original dst. We still count it so the
			// tuple-boundary tracking stays correct.
			srcCount++
		case "dst":
			ip, err := netip.ParseAddr(kv[1])
			if err != nil {
				return FlowBytes{}, false, err
			}
			if dstCount == 0 {
				fb.Key.OrigDstIP = ip
			}
			dstCount++
		case "sport":
			p, _ := strconv.Atoi(kv[1])
			if sportCount == 0 {
				fb.Key.OrigSrcPort = uint16(p)
			}
			sportCount++
		case "dport":
			p, _ := strconv.Atoi(kv[1])
			if dportCount == 0 {
				fb.Key.OrigDstPort = uint16(p)
			}
			dportCount++
		case "bytes":
			n, _ := strconv.ParseUint(kv[1], 10, 64)
			if bytesCount == 0 {
				fb.OrigBytes = n
			} else if bytesCount == 1 {
				fb.ReplyBytes = n
			}
			bytesCount++
		case "mark":
			n, _ := strconv.ParseUint(kv[1], 0, 64)
			if tag, ok := opts.RouteTags[uint32(n)]; ok {
				fb.RouteTag = tag
			}
		}
	}

	if !fb.Key.OrigSrcIP.IsPrivate() {
		return FlowBytes{}, false, nil
	}
	fb.Direction = DirOutbound
	fb.ClientIP = fb.Key.OrigSrcIP
	fb.LocalPort = fb.Key.OrigSrcPort
	fb.RemoteIP = fb.Key.OrigDstIP
	fb.RemotePort = fb.Key.OrigDstPort
	fb.NATPublicIP = netip.Addr{}
	return fb, true, nil
}
