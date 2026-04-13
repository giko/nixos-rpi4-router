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
	Identifier  uint32 // ICMP echo id (proto 1/58). Zero for TCP/UDP/SCTP etc.
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
	State         string // "ESTABLISHED", "TIME_WAIT", etc.
}

// EnumerateOpts configures flow enumeration. RouteTags maps a numeric
// conntrack mark (as emitted by the nftables mangle rules) to the
// tunnel name the dashboard surface reports. The caller is responsible
// for building this map from the NixOS topology — fwmarks are assigned
// sequentially based on the sorted list of configured tunnel names, so
// hardcoding a static table here would silently mislabel flows on any
// deployment with a different tunnel set.
//
// LANPrefixes lists the subnets that should be considered "our LAN" when
// attributing flow direction. Callers should pass the router's actual
// client subnets (e.g. 192.168.1.0/24, 192.168.20.0/24) so that router
// sessions terminating on an RFC1918 / CGNAT WAN address are correctly
// skipped instead of being misclassified as inbound DNAT, and so that
// private-space peers on the far side of a site-to-site VPN aren't
// mistaken for local clients. When LANPrefixes is empty, parseLine falls
// back to netip.Addr.IsPrivate() — this is retained for legacy callers
// and tests that don't need precise LAN membership.
type EnumerateOpts struct {
	RouteTags   map[uint32]string
	LANPrefixes []netip.Prefix
}

// isLAN returns true if ip falls within any of the supplied prefixes.
// When prefixes is empty, falls back to ip.IsPrivate() — useful for tests
// that don't care about precise LAN membership but require the pre-Task-1.3
// private-space heuristic.
func isLAN(ip netip.Addr, prefixes []netip.Prefix) bool {
	if len(prefixes) == 0 {
		return ip.IsPrivate()
	}
	for _, p := range prefixes {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

// EnumerateFlows parses /proc/net/nf_conntrack format (one line per flow)
// and returns per-flow records. Caller supplies a reader so tests can
// inject fixtures. Returns flows whose original source, or reply source
// (for inbound DNAT), falls within opts.LANPrefixes. When LANPrefixes is
// empty the check falls back to netip.Addr.IsPrivate() — see EnumerateOpts
// for the reasoning behind requiring explicit prefixes in production.
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

	// Reply-direction tuple fields (second occurrence of src/dst/sport/dport)
	// are captured so we can attribute inbound DNAT flows: when the original
	// source is a public peer but the reply source is a LAN host (meaning
	// traffic was DNAT'd into the LAN), the reply tuple is the only place
	// where the LAN endpoint appears.
	var (
		replySrc   netip.Addr
		replyDst   netip.Addr
		replySport uint16
		replyDport uint16
	)
	var (
		srcCount   int
		dstCount   int
		sportCount int
		dportCount int
		bytesCount int
		idCount    int
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
			} else if srcCount == 1 {
				replySrc = ip
			}
			srcCount++
		case "dst":
			ip, err := netip.ParseAddr(kv[1])
			if err != nil {
				return FlowBytes{}, false, err
			}
			if dstCount == 0 {
				fb.Key.OrigDstIP = ip
			} else if dstCount == 1 {
				replyDst = ip
			}
			dstCount++
		case "sport":
			p, _ := strconv.Atoi(kv[1])
			if sportCount == 0 {
				fb.Key.OrigSrcPort = uint16(p)
			} else if sportCount == 1 {
				replySport = uint16(p)
			}
			sportCount++
		case "dport":
			p, _ := strconv.Atoi(kv[1])
			if dportCount == 0 {
				fb.Key.OrigDstPort = uint16(p)
			} else if dportCount == 1 {
				replyDport = uint16(p)
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
		case "id":
			if idCount == 0 {
				n, _ := strconv.ParseUint(kv[1], 10, 32)
				fb.Key.Identifier = uint32(n)
			}
			idCount++
		case "mark":
			n, _ := strconv.ParseUint(kv[1], 0, 64)
			if tag, ok := opts.RouteTags[uint32(n)]; ok {
				fb.RouteTag = tag
			}
		}
	}

	// replyDst and replyDport are captured for completeness (they carry the
	// pre-NAT / post-NAT peer endpoint depending on direction) but the
	// attribution block below doesn't need them yet: outbound flows derive
	// the remote endpoint from the original destination and inbound DNAT
	// flows derive it from the original source.
	_ = replyDst
	_ = replyDport

	switch {
	case isLAN(fb.Key.OrigSrcIP, opts.LANPrefixes):
		// Outbound: client is orig src.
		fb.Direction = DirOutbound
		fb.ClientIP = fb.Key.OrigSrcIP
		fb.LocalPort = fb.Key.OrigSrcPort
		fb.RemoteIP = fb.Key.OrigDstIP
		fb.RemotePort = fb.Key.OrigDstPort
		fb.NATPublicIP = netip.Addr{}
		fb.NATPublicPort = 0
	case replySrc.IsValid() && isLAN(replySrc, opts.LANPrefixes) &&
		!isLAN(fb.Key.OrigSrcIP, opts.LANPrefixes) &&
		!isLAN(fb.Key.OrigDstIP, opts.LANPrefixes):
		// Inbound DNAT: client is reply src. The original destination is
		// the public (pre-DNAT) address the peer contacted; the original
		// source is the remote peer. We also require that orig src is NOT
		// a LAN host — otherwise LAN→LAN traffic between two local subnets
		// would be double-classified. Finally, we require that orig dst is
		// NOT a LAN host: if the original packet was already addressed to a
		// LAN host (e.g. a site-to-site VPN peer reaching a LAN host
		// directly), no DNAT rewrite happened — this is a routed session,
		// not a port forward, and should not be labelled with NAT metadata.
		fb.Direction = DirInbound
		fb.ClientIP = replySrc
		fb.LocalPort = replySport
		fb.RemoteIP = fb.Key.OrigSrcIP
		fb.RemotePort = fb.Key.OrigSrcPort
		fb.NATPublicIP = fb.Key.OrigDstIP
		fb.NATPublicPort = fb.Key.OrigDstPort
		// The prerouting mangle chain in modules/nftables.nix returns early
		// for iifname != lanIf, so inbound packets on the WAN interface
		// never hit the fwmark set-rules — the conntrack row carries
		// mark=0, which leaves RouteTag empty after the mark lookup above.
		// But these flows are deterministically WAN-routed (we only reach
		// this branch when the reply src is a LAN host and neither orig
		// src nor orig dst is a LAN host, i.e. a peer came in over WAN
		// and was DNAT'd to a local client). Default the tag to "WAN" so
		// downstream grouping-by-RouteTag doesn't silently lose inbound
		// DNAT flows.
		if fb.RouteTag == "" {
			fb.RouteTag = "WAN"
		}
	default:
		// Neither side is LAN — not our flow (e.g. router-originated on a
		// double-NAT WAN, site-to-site with private peers, or plain
		// transit).
		return FlowBytes{}, false, nil
	}
	return fb, true, nil
}
