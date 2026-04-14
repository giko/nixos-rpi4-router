package server

import (
	"net/http"
	"net/netip"
	"time"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/collector"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/envelope"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/model"
)

// NewClientSparklinesHandler returns the GET /api/clients/{ip}/sparklines
// handler. It bundles three per-client rings (traffic, DNS qps, open
// flow count) in a single payload so the activity panel can render with
// one fetch. All three rings share the hot-tier 10-second tick boundary.
//
// Lease lifecycle:
//   - dynamic/expired: returns the rings (expired flips stale=true).
//   - non-dynamic:     returns an OK body with all rings nil.
//   - unknown:         404.
//
// Any of the collectors may be nil — the handler simply leaves the
// corresponding ring nil in that case, so main.go can wire up whichever
// subset is available.
func NewClientSparklinesHandler(
	lookup clientLookup,
	traffic *collector.ClientTraffic,
	dnsRate *collector.DnsRate,
	flowCount *collector.FlowCount,
) http.HandlerFunc {
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
		body := model.ClientSparklines{
			ClientIP:    ip.String(),
			LeaseStatus: string(status),
			TickSeconds: 10,
		}
		if status == collector.LeaseStatusNonDynamic {
			envelope.WriteJSON(w, http.StatusOK, body, now, false)
			return
		}
		if traffic != nil {
			if samples, _, ok := traffic.Snapshot(ip); ok {
				body.Traffic = toTrafficSamples(samples)
			}
		}
		if dnsRate != nil {
			if samples, ok := dnsRate.Snapshot(ip); ok {
				body.DnsQps = toDnsQpsSamples(samples)
			}
		}
		if flowCount != nil {
			if samples, ok := flowCount.Snapshot(ip); ok {
				body.FlowCount = toFlowCountSamples(samples)
			}
		}
		envelope.WriteJSON(w, http.StatusOK, body, now, status == collector.LeaseStatusExpired)
	}
}

func toDnsQpsSamples(s []collector.DnsRateSample) []model.DnsQpsSample {
	if len(s) == 0 {
		return nil
	}
	out := make([]model.DnsQpsSample, 0, len(s))
	for _, v := range s {
		out = append(out, model.DnsQpsSample{T: v.T, QueriesPerWindow: v.QueriesPerWindow})
	}
	return out
}

func toFlowCountSamples(s []collector.FlowCountSample) []model.FlowCountSample {
	if len(s) == 0 {
		return nil
	}
	out := make([]model.FlowCountSample, 0, len(s))
	for _, v := range s {
		out = append(out, model.FlowCountSample{T: v.T, OpenFlows: v.OpenFlows})
	}
	return out
}
