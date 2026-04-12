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
	ConnCount       int       `json:"conn_count"` // total tracked connections from conntrack
	DNSQueries1h    int       `json:"dns_queries_1h"`
}

type ClientDetail struct {
	Client
	RecentQueries    []QueryLogEntry `json:"recent_queries"`
	BlockedQueries1h int             `json:"blocked_queries_1h"`
}
