package server

import (
	"net/http"
	"net/netip"
	"strconv"
	"time"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/collector"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/envelope"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/model"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/conntrack"
)

// FlowSource hands the connections handler the current per-flow
// conntrack snapshot. Production wires a thread-safe cache populated
// by the hot-tier collector (Task 9.1).
type FlowSource interface {
	Snapshot() []conntrack.FlowBytes
}

// DomainLookup matches *collector.DomainEnricher.Lookup so handlers and
// tests can inject a fake.
type DomainLookup interface {
	Lookup(client, remote netip.Addr, now time.Time) (string, bool)
}

func NewClientConnectionsHandler(lookup clientLookup, flows FlowSource, domains DomainLookup) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip, err := netip.ParseAddr(r.PathValue("ip"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		status := resolveLeaseStatus(lookup, ip)
		if status == collector.LeaseStatusUnknown {
			http.NotFound(w, r)
			return
		}

		now := time.Now().UTC()
		all := flows.Snapshot()
		out := make([]model.ClientFlow, 0, 8)
		for _, fb := range all {
			if fb.ClientIP != ip {
				continue
			}
			cf := buildClientFlow(fb, ip, domains, now)
			out = append(out, cf)
		}

		envelope.WriteJSON(w, http.StatusOK, model.ClientConnections{
			ClientIP:    ip.String(),
			LeaseStatus: string(status),
			Flows:       out,
			Count:       len(out),
		}, now, status == collector.LeaseStatusExpired)
	}
}

func buildClientFlow(fb conntrack.FlowBytes, clientIP netip.Addr, domains DomainLookup, now time.Time) model.ClientFlow {
	cf := model.ClientFlow{
		Proto:      protoString(fb.Key.Proto),
		Direction:  fb.Direction.String(),
		LocalIP:    clientIP.String(),
		LocalPort:  fb.LocalPort,
		RemoteIP:   addrString(fb.RemoteIP),
		RemotePort: fb.RemotePort,
		RouteTag:   fb.RouteTag,
		State:      fb.State,
	}
	if fb.NATPublicIP.IsValid() {
		cf.NATPublicIP = fb.NATPublicIP.String()
		cf.NATPublicPort = fb.NATPublicPort
	}
	switch fb.Direction {
	case conntrack.DirOutbound:
		cf.ClientTxBytes = fb.OrigBytes
		cf.ClientRxBytes = fb.ReplyBytes
	case conntrack.DirInbound:
		cf.ClientRxBytes = fb.OrigBytes
		cf.ClientTxBytes = fb.ReplyBytes
	}
	if domains != nil && fb.RemoteIP.IsValid() {
		if d, ok := domains.Lookup(clientIP, fb.RemoteIP, now); ok {
			cf.Domain = d
		}
	}
	return cf
}

func protoString(p uint8) string {
	switch p {
	case 6:
		return "tcp"
	case 17:
		return "udp"
	case 1:
		return "icmp"
	case 58:
		return "icmpv6"
	case 132:
		return "sctp"
	default:
		return strconv.Itoa(int(p))
	}
}

func addrString(a netip.Addr) string {
	if !a.IsValid() {
		return ""
	}
	return a.String()
}
