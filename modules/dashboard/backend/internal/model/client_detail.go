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
