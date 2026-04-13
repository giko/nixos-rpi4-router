package collector

import (
	"fmt"
	"net/netip"
	"testing"
	"time"
)

func TestDomainEnrichStoresAndLooksUp(t *testing.T) {
	e := NewDomainEnricher(EnricherOpts{PerClientCap: 2000, GlobalCap: 40000})
	client := netip.MustParseAddr("192.168.1.42")
	rip := netip.MustParseAddr("52.84.17.12")
	now := time.Now()
	e.Record(client, rip, "cdn.netflix.com", 60*time.Second, now)

	got, ok := e.Lookup(client, rip, now.Add(10*time.Second))
	if !ok || got != "cdn.netflix.com" {
		t.Fatalf("Lookup = %q, %v; want cdn.netflix.com, true", got, ok)
	}
}

func TestDomainEnrichExpiresAfterTTLPlusGrace(t *testing.T) {
	e := NewDomainEnricher(EnricherOpts{PerClientCap: 2000, Grace: 30 * time.Minute})
	client := netip.MustParseAddr("192.168.1.42")
	rip := netip.MustParseAddr("52.84.17.12")
	now := time.Now()
	e.Record(client, rip, "cdn.netflix.com", 60*time.Second, now)

	if _, ok := e.Lookup(client, rip, now.Add(29*time.Minute)); !ok {
		t.Error("within grace: should still be present")
	}
	if _, ok := e.Lookup(client, rip, now.Add(31*time.Minute+time.Second)); ok {
		t.Error("past grace: should be expired")
	}
}

func TestDomainEnrichMostRecentWins(t *testing.T) {
	e := NewDomainEnricher(EnricherOpts{PerClientCap: 2000})
	client := netip.MustParseAddr("192.168.1.42")
	rip := netip.MustParseAddr("1.2.3.4")
	now := time.Now()
	e.Record(client, rip, "older.example", 60*time.Second, now)
	e.Record(client, rip, "newer.example", 60*time.Second, now.Add(1*time.Second))

	got, _ := e.Lookup(client, rip, now.Add(2*time.Second))
	if got != "newer.example" {
		t.Errorf("got %q, want newer.example", got)
	}
}

func TestDomainEnrichEvictsAtPerClientCap(t *testing.T) {
	e := NewDomainEnricher(EnricherOpts{PerClientCap: 3})
	client := netip.MustParseAddr("192.168.1.42")
	now := time.Now()
	for i := 0; i < 5; i++ {
		rip := netip.MustParseAddr(fmt.Sprintf("10.0.0.%d", i))
		e.Record(client, rip, fmt.Sprintf("d%d.example", i), 60*time.Second, now.Add(time.Duration(i)*time.Second))
	}
	// First two should be evicted (least-recent).
	if _, ok := e.Lookup(client, netip.MustParseAddr("10.0.0.0"), now.Add(10*time.Second)); ok {
		t.Error("d0 should have been evicted")
	}
	if _, ok := e.Lookup(client, netip.MustParseAddr("10.0.0.4"), now.Add(10*time.Second)); !ok {
		t.Error("d4 should still be present")
	}
}
