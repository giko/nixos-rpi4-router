package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/collector"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/envelope"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/adguard"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/state"
)

func handleAdguardStats(st *state.State) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		data, updated := st.SnapshotAdguard()
		stale := state.IsStale(updated, collector.Medium.Interval())
		envelope.WriteJSON(w, http.StatusOK, data, updated, stale)
	}
}

// --- querylog cache with singleflight deduplication ---

const queryLogTTL = 3 * time.Second

type queryLogEntry struct {
	body      json.RawMessage
	expiresAt time.Time
}

type queryLogCache struct {
	client *adguard.Client
	group  singleflight.Group
	mu     sync.Mutex
	cache  map[string]queryLogEntry
}

func newQueryLogCache(client *adguard.Client) *queryLogCache {
	return &queryLogCache{client: client, cache: make(map[string]queryLogEntry)}
}

func (q *queryLogCache) fetch(r *http.Request) (json.RawMessage, error) {
	key := r.URL.RawQuery

	// Check cache under lock.
	q.mu.Lock()
	if ent, ok := q.cache[key]; ok && time.Now().Before(ent.expiresAt) {
		q.mu.Unlock()
		return ent.body, nil
	}
	q.mu.Unlock()

	// Build options from query params.
	opts := adguard.QueryLogOptions{Limit: 200}
	qv := r.URL.Query()
	if v := qv.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			opts.Limit = n
		}
	}
	opts.Client = qv.Get("client")
	opts.Domain = qv.Get("domain")

	// Use context.Background() inside Do — NOT r.Context().
	// This prevents a disconnected first caller from cancelling the
	// shared fetch for all waiters.
	result, err, _ := q.group.Do(key, func() (any, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		raw, err := q.client.FetchQueryLog(ctx, opts)
		if err != nil {
			return nil, err
		}
		// Cache the result.
		q.mu.Lock()
		q.cache[key] = queryLogEntry{body: raw, expiresAt: time.Now().Add(queryLogTTL)}
		// Evict stale entries.
		for k, v := range q.cache {
			if time.Now().After(v.expiresAt) {
				delete(q.cache, k)
			}
		}
		q.mu.Unlock()
		return raw, nil
	})
	if err != nil {
		return nil, err
	}
	return result.(json.RawMessage), nil
}

func handleAdguardQueryLog(cache *queryLogCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		raw, err := cache.fetch(r)
		if err != nil {
			envelope.WriteJSON(w, http.StatusBadGateway, map[string]any{"error": err.Error()}, time.Now().UTC(), true)
			return
		}
		// raw is the normalised entries array from FetchQueryLog.
		// Wrap as {"queries": <array>} per spec.
		envelope.WriteJSON(w, http.StatusOK, map[string]json.RawMessage{"queries": raw}, time.Now().UTC(), false)
	}
}
