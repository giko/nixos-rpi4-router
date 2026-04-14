package server

import (
	"net/http"
	"net/netip"
	"time"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/collector"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/envelope"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/model"
)

// NewClientTrafficHandler returns the GET /api/clients/{ip}/traffic
// handler. It honours the lease lifecycle: dynamic clients get the
// full ring; non-dynamic (static / neighbor) clients get a body with
// nil samples; expired (tombstoned) clients get the cached ring with
// stale=true; everything else is 404.
func NewClientTrafficHandler(lookup clientLookup, traffic *collector.ClientTraffic) http.HandlerFunc {
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
		if status == collector.LeaseStatusNonDynamic {
			envelope.WriteJSON(w, http.StatusOK, model.ClientTraffic{
				ClientIP:    ip.String(),
				LeaseStatus: string(status),
				TickSeconds: 10,
			}, time.Now().UTC(), false)
			return
		}
		samples, stats, ok := traffic.Snapshot(ip)
		if !ok {
			http.NotFound(w, r)
			return
		}
		envelope.WriteJSON(w, http.StatusOK, model.ClientTraffic{
			ClientIP:        ip.String(),
			LeaseStatus:     string(status),
			Samples:         toTrafficSamples(samples),
			CurrentRxBps:    stats.CurrentRx,
			CurrentTxBps:    stats.CurrentTx,
			PeakRxBps10m:    stats.PeakRx,
			PeakTxBps10m:    stats.PeakTx,
			TotalRxBytes10m: stats.TotalRxBytes,
			TotalTxBytes10m: stats.TotalTxBytes,
			TickSeconds:     10,
		}, time.Now().UTC(), status == collector.LeaseStatusExpired)
	}
}

func toTrafficSamples(s []collector.TrafficSample) []model.TrafficSample {
	if len(s) == 0 {
		return nil
	}
	out := make([]model.TrafficSample, 0, len(s))
	for _, v := range s {
		out = append(out, model.TrafficSample{T: v.T, RxBps: v.RxBps, TxBps: v.TxBps})
	}
	return out
}
