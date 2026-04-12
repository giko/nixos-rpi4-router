// Package ratelimit provides per-IP and global token-bucket rate limiting
// as an http middleware.
package ratelimit

import (
	"encoding/json"
	"net"
	"net/http"
	"sync"

	"golang.org/x/time/rate"
)

// Options configures the two token buckets.
type Options struct {
	PerIPPerSec  float64
	PerIPBurst   int
	GlobalPerSec float64
	GlobalBurst  int
}

// Default returns production-grade defaults: 20 req/s per IP (burst 40),
// 200 req/s global (burst 400).
func Default() Options {
	return Options{PerIPPerSec: 20, PerIPBurst: 40, GlobalPerSec: 200, GlobalBurst: 400}
}

// New returns a middleware that enforces two token-bucket limits.
// /api/health is exempt so monitoring probes are never throttled.
//
// Per-IP is checked BEFORE global so a noisy client does not drain
// the shared bucket.
func New(opts Options) func(http.Handler) http.Handler {
	global := rate.NewLimiter(rate.Limit(opts.GlobalPerSec), opts.GlobalBurst)

	var mu sync.Mutex
	perIP := map[string]*rate.Limiter{}

	getLimiter := func(ip string) *rate.Limiter {
		mu.Lock()
		defer mu.Unlock()
		if l, ok := perIP[ip]; ok {
			return l
		}
		l := rate.NewLimiter(rate.Limit(opts.PerIPPerSec), opts.PerIPBurst)
		perIP[ip] = l
		return l
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/api/health" {
				next.ServeHTTP(w, r)
				return
			}

			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				ip = r.RemoteAddr
			}

			// Per-IP first, then global.
			if !getLimiter(ip).Allow() {
				tooMany(w)
				return
			}
			if !global.Allow() {
				tooMany(w)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func tooMany(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Retry-After", "1")
	w.WriteHeader(http.StatusTooManyRequests)
	json.NewEncoder(w).Encode(map[string]any{
		"error":               "rate limit exceeded",
		"retry_after_seconds": 1,
	})
}
