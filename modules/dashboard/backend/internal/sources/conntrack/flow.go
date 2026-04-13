// Package conntrack reads /proc/net/nf_conntrack and exposes per-flow
// records with NAT direction attribution for downstream collectors.
package conntrack

import (
	"net/netip"
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
	RouteTag      string     // "WAN" | "wg_sw" | "wg_us" | "wg_tr" — from conntrack mark
	NATPublicIP   netip.Addr // zero unless inbound DNAT / UPnP
	NATPublicPort uint16
	LocalPort     uint16 // client-side port (derived from Direction)
	RemoteIP      netip.Addr
	RemotePort    uint16
	Age           time.Duration
	State         string // "ESTABLISHED", "TIME_WAIT", etc.
}
