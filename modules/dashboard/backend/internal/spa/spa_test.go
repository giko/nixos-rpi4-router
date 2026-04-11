package spa

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerServesIndex(t *testing.T) {
	h := Handler()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	if !strings.Contains(string(body), "SENTINEL OS") {
		t.Errorf("body missing SENTINEL OS; got %q", body)
	}
}

func TestHandlerFallsBackToIndexForNavigationRequest(t *testing.T) {
	h := Handler()
	// A non-existent path like /vpn/pools should still serve index.html
	// so the React client-side router can pick it up — but only when the
	// request looks like a browser navigation (Accept: text/html).
	req := httptest.NewRequest(http.MethodGet, "/vpn/pools", nil)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	if !strings.Contains(string(body), "SENTINEL OS") {
		t.Errorf("SPA fallback did not serve index.html; got %q", body)
	}
}

func TestHandlerReturns404ForMissingAsset(t *testing.T) {
	h := Handler()
	// A stale hashed asset URL (e.g. after a frontend rebuild) should return
	// 404, NOT the HTML shell. Otherwise the browser gets 200 + <!doctype html>
	// and errors with "Unexpected token '<'" when it tries to parse it as JS.
	req := httptest.NewRequest(http.MethodGet, "/assets/index-stale.js", nil)
	req.Header.Set("Accept", "*/*")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	if strings.Contains(string(body), "SENTINEL OS") {
		t.Errorf("SPA shell leaked to non-navigation request: %q", body)
	}
}
