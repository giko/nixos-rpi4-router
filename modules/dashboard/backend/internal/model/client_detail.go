package model

import "time"

// TrafficSample is one ring entry of per-client throughput. T is the
// tick boundary at which the deltas were computed.
type TrafficSample struct {
	T     time.Time `json:"t"`
	RxBps uint64    `json:"rx_bps"`
	TxBps uint64    `json:"tx_bps"`
}

// ClientTraffic is the body of /api/clients/{ip}/traffic. Samples is
// nil for non-dynamic clients (no per-flow accounting available).
// TickSeconds tells the frontend how to label the X axis.
type ClientTraffic struct {
	ClientIP        string          `json:"client_ip"`
	LeaseStatus     string          `json:"lease_status"`
	Samples         []TrafficSample `json:"samples"`
	CurrentRxBps    uint64          `json:"current_rx_bps"`
	CurrentTxBps    uint64          `json:"current_tx_bps"`
	PeakRxBps10m    uint64          `json:"peak_rx_bps_10m"`
	PeakTxBps10m    uint64          `json:"peak_tx_bps_10m"`
	TotalRxBytes10m uint64          `json:"total_rx_bytes_10m"`
	TotalTxBytes10m uint64          `json:"total_tx_bytes_10m"`
	TickSeconds     int             `json:"tick_seconds"`
}

type ClientDnsQuery struct {
	Time         time.Time `json:"time"`
	Question     string    `json:"question"`
	QuestionType string    `json:"question_type"`
	Upstream     string    `json:"upstream"`
	Reason       string    `json:"reason"`
	ElapsedMs    float64   `json:"elapsed_ms"`
	Blocked      bool      `json:"blocked"`
}

type ClientDns struct {
	ClientIP string           `json:"client_ip"`
	Recent   []ClientDnsQuery `json:"recent"`
	Count    int              `json:"count"`
	Limit    int              `json:"limit"`
}

// TODO: add age_seconds once a first-seen tracker is wired into the
// conntrack collector. /proc/net/nf_conntrack reports the remaining
// timeout, not the age; computing a real age requires a stateful
// per-FlowKey map that is out of scope for this task.

// ClientFlow describes one open conntrack flow attributed to a client.
// LocalIP/Port and RemoteIP/Port are oriented from the client's POV
// regardless of conntrack's original/reply tuple. NATPublicIP/Port are
// non-zero for inbound DNAT'd flows.
type ClientFlow struct {
	Proto         string `json:"proto"`
	Direction     string `json:"direction"`
	LocalIP       string `json:"local_ip"`
	LocalPort     uint16 `json:"local_port"`
	RemoteIP      string `json:"remote_ip"`
	RemotePort    uint16 `json:"remote_port"`
	NATPublicIP   string `json:"nat_public_ip,omitempty"`
	NATPublicPort uint16 `json:"nat_public_port,omitempty"`
	Domain        string `json:"domain"`
	RouteTag      string `json:"route_tag"`
	ClientRxBytes uint64 `json:"client_rx_bytes"`
	ClientTxBytes uint64 `json:"client_tx_bytes"`
	State         string `json:"state"`
}

type ClientConnections struct {
	ClientIP    string       `json:"client_ip"`
	LeaseStatus string       `json:"lease_status"`
	Flows       []ClientFlow `json:"flows"`
	Count       int          `json:"count"`
}

type ClientTopDestination struct {
	Domain   string    `json:"domain"`
	Queries  uint64    `json:"queries"`
	Blocked  uint64    `json:"blocked"`
	Bytes    uint64    `json:"bytes"`
	Flows    uint32    `json:"flows"`
	LastSeen time.Time `json:"last_seen"`
}

type ClientTopDestinations struct {
	ClientIP      string                 `json:"client_ip"`
	LeaseStatus   string                 `json:"lease_status"`
	WindowSeconds int                    `json:"window_seconds"`
	Destinations  []ClientTopDestination `json:"destinations"`
	Count         int                    `json:"count"`
}

type DnsQpsSample struct {
	T                time.Time `json:"t"`
	QueriesPerWindow uint32    `json:"queries_per_window"`
}

type FlowCountSample struct {
	T         time.Time `json:"t"`
	OpenFlows uint32    `json:"open_flows"`
}

// ClientSparklines bundles the three short rings the activity panel
// renders. All three slices are nil for non-dynamic clients (no
// per-client accounting). TickSeconds applies to all three rings —
// they share the hot-tier 10-second tick boundary.
type ClientSparklines struct {
	ClientIP    string            `json:"client_ip"`
	LeaseStatus string            `json:"lease_status"`
	TickSeconds int               `json:"tick_seconds"`
	Traffic     []TrafficSample   `json:"traffic"`
	DnsQps      []DnsQpsSample    `json:"dns_qps"`
	FlowCount   []FlowCountSample `json:"flow_count"`
}
