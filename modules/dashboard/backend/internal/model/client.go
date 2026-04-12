package model

import "time"

type Client struct {
	Hostname        string    `json:"hostname"`
	IP              string    `json:"ip"`
	MAC             string    `json:"mac"`
	LeaseType       string    `json:"lease_type"`
	LastSeen        time.Time `json:"last_seen"`
	Route           string    `json:"route"`
	AllowlistStatus string    `json:"allowlist_status"`
	// ConnCount is the total number of tracked connections for this client
	// across every destination (WAN, LAN, VPN). For pool-scoped counts,
	// use TunnelConns keyed by the pool's tunnel fwmarks.
	ConnCount int `json:"conn_count"`
	// TunnelConns is the per-tunnel connection breakdown, keyed by hex
	// fwmark. Only non-zero marks are included. A client whose
	// connections are round-robined across a pool has entries here for
	// every healthy tunnel.
	TunnelConns  map[string]int `json:"tunnel_conns"`
	DNSQueries1h int            `json:"dns_queries_1h"`
}

type ClientDetail struct {
	Client
	RecentQueries    []QueryLogEntry `json:"recent_queries"`
	BlockedQueries1h int             `json:"blocked_queries_1h"`
}
