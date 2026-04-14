package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/envelope"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/model"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/adguard"
)

type fakeAdguardLog struct {
	mu    sync.Mutex
	rows  []adguard.QueryLogClientRow
	err   error
	last  string
	calls int32
	// delay, when non-zero, blocks FetchQueryLogForClient for the
	// configured duration. Used by the singleflight test to verify
	// concurrent callers coalesce into a single upstream request.
	delay time.Duration
}

func (f *fakeAdguardLog) FetchQueryLogForClient(_ context.Context, ip string, _ int) ([]adguard.QueryLogClientRow, error) {
	atomic.AddInt32(&f.calls, 1)
	f.mu.Lock()
	f.last = ip
	err := f.err
	rows := f.rows
	delay := f.delay
	f.mu.Unlock()
	if delay > 0 {
		time.Sleep(delay)
	}
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func newDnsSrv(t *testing.T, ag AdguardQueryLogClient) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/clients/{ip}/dns", NewClientDnsHandler(newClientDnsCache(ag)))
	return httptest.NewServer(mux)
}

func TestClientDnsReturnsRows(t *testing.T) {
	now := time.Now().UTC()
	fake := &fakeAdguardLog{rows: []adguard.QueryLogClientRow{
		{Time: now, Question: "example.com", QuestionType: "A", Upstream: "tls://1.1.1.1", Reason: "NotFiltered", ElapsedMs: 1.2, Blocked: false},
		{Time: now.Add(-time.Second), Question: "ads.doubleclick.net", QuestionType: "A", Reason: "FilteredBlackList", Blocked: true},
	}}
	srv := newDnsSrv(t, fake)
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/clients/192.168.1.42/dns")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	var env envelope.Response
	_ = json.NewDecoder(resp.Body).Decode(&env)
	raw, _ := json.Marshal(env.Data)
	var body model.ClientDns
	_ = json.Unmarshal(raw, &body)
	if body.ClientIP != "192.168.1.42" {
		t.Errorf("ClientIP = %q", body.ClientIP)
	}
	if body.Limit != 100 {
		t.Errorf("Limit = %d", body.Limit)
	}
	if body.Count != 2 {
		t.Errorf("Count = %d", body.Count)
	}
	if len(body.Recent) != 2 || !body.Recent[1].Blocked {
		t.Errorf("Recent unexpected: %+v", body.Recent)
	}
	if fake.last != "192.168.1.42" {
		t.Errorf("downstream not filtered to ip; got %q", fake.last)
	}
}

func TestClientDnsBadGatewayOnAdguardError(t *testing.T) {
	srv := newDnsSrv(t, &fakeAdguardLog{err: errors.New("boom")})
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/clients/192.168.1.42/dns")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestClientDnsBadIPReturns404(t *testing.T) {
	srv := newDnsSrv(t, &fakeAdguardLog{})
	defer srv.Close()
	resp, err := http.Get(srv.URL + "/api/clients/not-an-ip/dns")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

// TestClientDnsCoalescesConcurrentFetches fires several concurrent
// requests for the same client IP against a slow AdGuard backend; the
// singleflight cache must turn them into a single upstream call. Without
// this guarantee, two open client-detail tabs re-introduce the hammer
// that queryLogCache eliminated for the global querylog endpoint.
func TestClientDnsCoalescesConcurrentFetches(t *testing.T) {
	fake := &fakeAdguardLog{
		rows:  []adguard.QueryLogClientRow{{Question: "example.com"}},
		delay: 100 * time.Millisecond,
	}
	srv := newDnsSrv(t, fake)
	defer srv.Close()

	const parallel = 5
	var wg sync.WaitGroup
	wg.Add(parallel)
	errs := make(chan error, parallel)
	for i := 0; i < parallel; i++ {
		go func() {
			defer wg.Done()
			resp, err := http.Get(srv.URL + "/api/clients/192.168.1.42/dns")
			if err != nil {
				errs <- err
				return
			}
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				errs <- errors.New("non-200")
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("request failed: %v", err)
	}
	if got := atomic.LoadInt32(&fake.calls); got != 1 {
		t.Errorf("FetchQueryLogForClient called %d times; want 1 (singleflight coalesce)", got)
	}
}
