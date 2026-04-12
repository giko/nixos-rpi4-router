package ratelimit_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/ratelimit"
)

// ok200 is a trivial handler that always returns 200.
var ok200 = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
})

func TestPerIPLimit(t *testing.T) {
	mw := ratelimit.New(ratelimit.Options{
		PerIPPerSec:  2,
		PerIPBurst:   2,
		GlobalPerSec: 1000,
		GlobalBurst:  1000,
	})
	h := mw(ok200)

	req := func() *http.Response {
		r := httptest.NewRequest("GET", "/api/test", nil)
		r.RemoteAddr = "10.0.0.1:12345"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		return w.Result()
	}

	// First two requests consume the burst.
	if s := req().StatusCode; s != 200 {
		t.Fatalf("request 1: want 200, got %d", s)
	}
	if s := req().StatusCode; s != 200 {
		t.Fatalf("request 2: want 200, got %d", s)
	}
	// Third should be rate-limited.
	resp := req()
	if resp.StatusCode != 429 {
		t.Fatalf("request 3: want 429, got %d", resp.StatusCode)
	}
	if ra := resp.Header.Get("Retry-After"); ra != "1" {
		t.Fatalf("Retry-After: want %q, got %q", "1", ra)
	}
}

func TestGlobalLimit(t *testing.T) {
	mw := ratelimit.New(ratelimit.Options{
		PerIPPerSec:  1000,
		PerIPBurst:   1000,
		GlobalPerSec: 1,
		GlobalBurst:  1,
	})
	h := mw(ok200)

	// First request from IP-A passes.
	r1 := httptest.NewRequest("GET", "/api/test", nil)
	r1.RemoteAddr = "10.0.0.1:1111"
	w1 := httptest.NewRecorder()
	h.ServeHTTP(w1, r1)
	if w1.Code != 200 {
		t.Fatalf("request 1: want 200, got %d", w1.Code)
	}

	// Second request from a different IP-B hits the global limit.
	r2 := httptest.NewRequest("GET", "/api/test", nil)
	r2.RemoteAddr = "10.0.0.2:2222"
	w2 := httptest.NewRecorder()
	h.ServeHTTP(w2, r2)
	if w2.Code != 429 {
		t.Fatalf("request 2: want 429, got %d", w2.Code)
	}
}

func TestHealthEndpointExempt(t *testing.T) {
	mw := ratelimit.New(ratelimit.Options{
		PerIPPerSec:  1,
		PerIPBurst:   1,
		GlobalPerSec: 1,
		GlobalBurst:  1,
	})
	h := mw(ok200)

	// Exhaust both buckets with a non-health request.
	r0 := httptest.NewRequest("GET", "/api/test", nil)
	r0.RemoteAddr = "10.0.0.1:1111"
	w0 := httptest.NewRecorder()
	h.ServeHTTP(w0, r0)

	// /api/health must still succeed, even 100 times in a row.
	for i := range 100 {
		r := httptest.NewRequest("GET", "/api/health", nil)
		r.RemoteAddr = "10.0.0.1:1111"
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 200 {
			t.Fatalf("health iteration %d: want 200, got %d", i, w.Code)
		}
	}
}
