// Package adguard wraps the AdGuard Home REST API.
package adguard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Client talks to the AdGuard Home REST API.
type Client struct {
	base *url.URL
	http *http.Client
}

// NewClient creates an AdGuard API client. If httpClient is nil,
// http.DefaultClient is used.
func NewClient(base string, httpClient *http.Client) *Client {
	u, err := url.Parse(strings.TrimRight(base, "/"))
	if err != nil {
		// Programming error — panic is appropriate for an unparseable URL
		// supplied at startup.
		panic(fmt.Sprintf("adguard: invalid base URL %q: %v", base, err))
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{base: u, http: httpClient}
}

// Stats represents the subset of /control/stats that the dashboard uses.
type Stats struct {
	NumDNSQueries int          `json:"num_dns_queries"`
	NumBlocked    int          `json:"num_blocked_filtering"`
	TopBlocked    []TopDomain  `json:"top_blocked"`
	TopQueried    []TopDomain  `json:"top_queried"`
	TopClients    []TopClient  `json:"top_clients"`
	Density       []DensityBin `json:"density"`
}

// TopDomain is a flattened domain + hit count.
type TopDomain struct {
	Domain string `json:"domain"`
	Count  int    `json:"count"`
}

// TopClient is a flattened client IP + query count.
type TopClient struct {
	IP    string `json:"ip"`
	Count int    `json:"count"`
}

// DensityBin is one hourly bucket of queries and blocked counts.
type DensityBin struct {
	StartHour int `json:"start_hour"`
	Queries   int `json:"queries"`
	Blocked   int `json:"blocked"`
}

// FetchStats calls GET /control/stats and returns normalised Stats.
//
// AdGuard's top_blocked_domains / top_queried_domains / top_clients
// arrays are encoded as [{"domain.com": 100}, ...] — a slice of
// single-key maps. We flatten them into typed slices.
func (c *Client) FetchStats(ctx context.Context) (Stats, error) {
	u := c.base.JoinPath("/control/stats").String()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return Stats{}, fmt.Errorf("adguard: build request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return Stats{}, fmt.Errorf("adguard: GET %s: %w", u, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Stats{}, fmt.Errorf("adguard: GET %s: status %d", u, resp.StatusCode)
	}

	var raw rawStats
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return Stats{}, fmt.Errorf("adguard: decode stats: %w", err)
	}

	return raw.flatten(), nil
}

// QueryLogOptions controls the /control/querylog request.
type QueryLogOptions struct {
	Limit  int
	Client string
	Domain string
}

// FetchQueryLog calls GET /control/querylog and returns the bare entries
// array as raw JSON. AdGuard returns {"data": [...], ...} — we extract
// "data" so the handler can wrap it however it likes.
func (c *Client) FetchQueryLog(ctx context.Context, opts QueryLogOptions) (json.RawMessage, error) {
	u := c.base.JoinPath("/control/querylog").String()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("adguard: build request: %w", err)
	}

	q := req.URL.Query()
	if opts.Limit > 0 {
		q.Set("limit", fmt.Sprintf("%d", opts.Limit))
	}

	search := buildSearch(opts.Client, opts.Domain)
	if search != "" {
		q.Set("search", search)
	}
	req.URL.RawQuery = q.Encode()

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("adguard: GET querylog: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("adguard: GET querylog: status %d", resp.StatusCode)
	}

	var envelope struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("adguard: decode querylog: %w", err)
	}

	return envelope.Data, nil
}

// QueryLogResponse is the decoded shape of /control/querylog. "data" is
// kept as raw JSON so consumers can decode into whatever per-entry shape
// they need (the background dns_ingest collector maps into its own row
// type; the /api/adguard/querylog handler forwards the array verbatim).
type QueryLogResponse struct {
	Oldest string          `json:"oldest"`
	Data   json.RawMessage `json:"data"`
}

// FetchQueryLogPage pulls one page of the global query log filtered by
// older_than. Used by the background dns_ingest collector with its
// watermark cursor. Does NOT filter by client — consumers filter
// locally after ingestion.
func (c *Client) FetchQueryLogPage(ctx context.Context, olderThan time.Time, limit int) (QueryLogResponse, error) {
	u := c.base.JoinPath("/control/querylog")

	q := url.Values{}
	if !olderThan.IsZero() {
		q.Set("older_than", olderThan.UTC().Format("2006-01-02T15:04:05.000000000Z"))
	}
	q.Set("limit", strconv.Itoa(limit))
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return QueryLogResponse{}, fmt.Errorf("adguard: build request: %w", err)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return QueryLogResponse{}, fmt.Errorf("adguard: GET querylog page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return QueryLogResponse{}, fmt.Errorf("adguard: GET querylog page: status %d", resp.StatusCode)
	}

	var body QueryLogResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return QueryLogResponse{}, fmt.Errorf("adguard: decode querylog page: %w", err)
	}
	return body, nil
}

// buildSearch concatenates client and domain with a space when both are set.
func buildSearch(client, domain string) string {
	parts := make([]string, 0, 2)
	if client != "" {
		parts = append(parts, client)
	}
	if domain != "" {
		parts = append(parts, domain)
	}
	return strings.Join(parts, " ")
}

// --- raw AdGuard JSON shapes ---

type rawStats struct {
	NumDNSQueries      int                `json:"num_dns_queries"`
	NumBlockedFiltered int                `json:"num_blocked_filtering"`
	TopBlockedDomains  []map[string]int   `json:"top_blocked_domains"`
	TopQueriedDomains  []map[string]int   `json:"top_queried_domains"`
	TopClients         []map[string]int   `json:"top_clients"`
	DNSQueries         []int              `json:"dns_queries"`
	BlockedFiltering   []int              `json:"blocked_filtering"`
}

func (r rawStats) flatten() Stats {
	s := Stats{
		NumDNSQueries: r.NumDNSQueries,
		NumBlocked:    r.NumBlockedFiltered,
	}

	s.TopBlocked = flattenDomains(r.TopBlockedDomains)
	s.TopQueried = flattenDomains(r.TopQueriedDomains)
	s.TopClients = flattenClients(r.TopClients)
	s.Density = buildDensity(r.DNSQueries, r.BlockedFiltering)

	return s
}

func flattenDomains(src []map[string]int) []TopDomain {
	out := make([]TopDomain, 0, len(src))
	for _, m := range src {
		for domain, count := range m {
			out = append(out, TopDomain{Domain: domain, Count: count})
		}
	}
	return out
}

func flattenClients(src []map[string]int) []TopClient {
	out := make([]TopClient, 0, len(src))
	for _, m := range src {
		for ip, count := range m {
			out = append(out, TopClient{IP: ip, Count: count})
		}
	}
	return out
}

func buildDensity(queries, blocked []int) []DensityBin {
	n := len(queries)
	if len(blocked) > n {
		n = len(blocked)
	}
	if n == 0 {
		return nil
	}
	out := make([]DensityBin, n)
	for i := 0; i < n; i++ {
		out[i].StartHour = i
		if i < len(queries) {
			out[i].Queries = queries[i]
		}
		if i < len(blocked) {
			out[i].Blocked = blocked[i]
		}
	}
	return out
}
