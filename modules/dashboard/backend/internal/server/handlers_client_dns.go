package server

import (
	"context"
	"net/http"
	"net/netip"
	"time"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/envelope"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/model"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/adguard"
)

// AdguardQueryLogClient is the small AdGuard surface the per-client DNS
// handler needs; tests inject a fake.
type AdguardQueryLogClient interface {
	FetchQueryLogForClient(ctx context.Context, clientIP string, limit int) ([]adguard.QueryLogClientRow, error)
}

const clientDnsLimit = 100

func NewClientDnsHandler(ag AdguardQueryLogClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip, err := netip.ParseAddr(r.PathValue("ip"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		rows, err := ag.FetchQueryLogForClient(ctx, ip.String(), clientDnsLimit)
		if err != nil {
			envelope.WriteJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()}, time.Now().UTC(), true)
			return
		}
		out := make([]model.ClientDnsQuery, 0, len(rows))
		for _, r := range rows {
			out = append(out, model.ClientDnsQuery{
				Time:         r.Time,
				Question:     r.Question,
				QuestionType: r.QuestionType,
				Upstream:     r.Upstream,
				Reason:       r.Reason,
				ElapsedMs:    r.ElapsedMs,
				Blocked:      r.Blocked,
			})
		}
		envelope.WriteJSON(w, http.StatusOK, model.ClientDns{
			ClientIP: ip.String(),
			Recent:   out,
			Count:    len(out),
			Limit:    clientDnsLimit,
		}, time.Now().UTC(), false)
	}
}
