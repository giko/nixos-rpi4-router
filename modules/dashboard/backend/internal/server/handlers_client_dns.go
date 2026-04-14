package server

import (
	"context"
	"net/http"
	"net/netip"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/envelope"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/model"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/adguard"
)

// AdguardQueryLogClient is the small AdGuard surface the per-client DNS
// handler needs; tests inject a fake.
type AdguardQueryLogClient interface {
	FetchQueryLogForClient(ctx context.Context, clientIP string, limit int) ([]adguard.QueryLogClientRow, error)
}

const (
	clientDnsLimit = 100
	clientDnsTTL   = 3 * time.Second
)

// clientDnsEntry holds a cached AdGuard response. `rows` is the slice
// returned by FetchQueryLogForClient — we hold the decoded value, not
// the raw JSON, because the handler iterates it to project out model
// fields per request.
type clientDnsEntry struct {
	rows      []adguard.QueryLogClientRow
	expiresAt time.Time
}

// clientDnsCache coalesces concurrent per-client DNS fetches behind a
// 3-second TTL and a singleflight.Group keyed on the client IP. Mirrors
// queryLogCache (global /api/adguard/querylog) so two open client-detail
// tabs polling every 3 seconds don't produce two AdGuard round-trips
// per tick per viewer.
type clientDnsCache struct {
	client AdguardQueryLogClient
	group  singleflight.Group
	mu     sync.Mutex
	cache  map[string]clientDnsEntry
}

func newClientDnsCache(ag AdguardQueryLogClient) *clientDnsCache {
	return &clientDnsCache{client: ag, cache: make(map[string]clientDnsEntry)}
}

// fetch returns cached rows when the TTL has not expired, or enters
// singleflight to coalesce concurrent upstream calls. Uses
// context.Background() inside Do so a disconnected first caller does
// not cancel the shared fetch for all waiters.
func (c *clientDnsCache) fetch(_ context.Context, ip string) ([]adguard.QueryLogClientRow, error) {
	c.mu.Lock()
	if ent, ok := c.cache[ip]; ok && time.Now().Before(ent.expiresAt) {
		rows := ent.rows
		c.mu.Unlock()
		return rows, nil
	}
	c.mu.Unlock()

	result, err, _ := c.group.Do(ip, func() (any, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		rows, err := c.client.FetchQueryLogForClient(ctx, ip, clientDnsLimit)
		if err != nil {
			return nil, err
		}
		c.mu.Lock()
		c.cache[ip] = clientDnsEntry{rows: rows, expiresAt: time.Now().Add(clientDnsTTL)}
		// Evict stale entries so long-lived caches don't grow unbounded
		// as clients come and go.
		for k, v := range c.cache {
			if time.Now().After(v.expiresAt) {
				delete(c.cache, k)
			}
		}
		c.mu.Unlock()
		return rows, nil
	})
	if err != nil {
		return nil, err
	}
	return result.([]adguard.QueryLogClientRow), nil
}

func NewClientDnsHandler(cache *clientDnsCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip, err := netip.ParseAddr(r.PathValue("ip"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		rows, err := cache.fetch(r.Context(), ip.String())
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
