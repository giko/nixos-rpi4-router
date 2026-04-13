package model

type Traffic struct {
	Interfaces []Interface `json:"interfaces"`
}

type Interface struct {
	Name string `json:"name"`
	// Role is one of "lan", "wan", "tunnel", or "" (unknown). Derived from
	// topology at collection time so the frontend can locate interfaces
	// by role without hardcoding names.
	Role         string            `json:"role"`
	Operstate    string            `json:"operstate"` // "up", "down", "unknown" — from /sys/class/net/<name>/operstate
	RXBps        uint64            `json:"rx_bps"`
	TXBps        uint64            `json:"tx_bps"`
	RXBytesTotal uint64            `json:"rx_bytes_total"`
	TXBytesTotal uint64            `json:"tx_bytes_total"`
	Samples60s   []InterfaceSample `json:"samples_60s"`
}

type InterfaceSample struct {
	RXBps uint64 `json:"rx_bps"`
	TXBps uint64 `json:"tx_bps"`
}
