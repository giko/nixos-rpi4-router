package server

import (
	"net/http"
	"net/netip"
	"time"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/collector"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/envelope"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/model"
)

const topDestinationsWindowSeconds = 3600

func NewClientTopDestinationsHandler(lookup clientLookup, td *collector.TopDestinations) http.HandlerFunc {
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
		body := model.ClientTopDestinations{
			ClientIP:      ip.String(),
			LeaseStatus:   string(status),
			WindowSeconds: topDestinationsWindowSeconds,
		}
		if status == collector.LeaseStatusNonDynamic {
			envelope.WriteJSON(w, http.StatusOK, body, now, false)
			return
		}
		snap := td.Snapshot(ip)
		dests := make([]model.ClientTopDestination, 0, len(snap))
		for _, d := range snap {
			dests = append(dests, model.ClientTopDestination{
				Domain:   d.Domain,
				Queries:  d.Queries,
				Blocked:  d.Blocked,
				Bytes:    d.Bytes,
				Flows:    d.Flows,
				LastSeen: d.LastSeen,
			})
		}
		body.Destinations = dests
		body.Count = len(dests)
		envelope.WriteJSON(w, http.StatusOK, body, now, status == collector.LeaseStatusExpired)
	}
}
