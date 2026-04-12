package model

import "time"

type SystemStats struct {
	CPU           CPUStats       `json:"cpu"`
	Memory        MemoryStats    `json:"memory"`
	TemperatureC  float64        `json:"temperature_c"`
	Throttled     string         `json:"throttled"`
	ThrottledFlag bool           `json:"throttled_flag"`
	UptimeSeconds float64        `json:"uptime_seconds"`
	LoadAverage   LoadAverage    `json:"load_average"`
	BootTime      time.Time      `json:"boot_time"`
	Services      []ServiceState `json:"services"`
}

type CPUStats struct {
	PercentUser   float64 `json:"percent_user"`
	PercentSystem float64 `json:"percent_system"`
	PercentIdle   float64 `json:"percent_idle"`
	PercentIOWait float64 `json:"percent_iowait"`
}

type MemoryStats struct {
	TotalBytes     uint64  `json:"total_bytes"`
	AvailableBytes uint64  `json:"available_bytes"`
	UsedBytes      uint64  `json:"used_bytes"`
	PercentUsed    float64 `json:"percent_used"`
}

type LoadAverage struct {
	One     float64 `json:"one"`
	Five    float64 `json:"five"`
	Fifteen float64 `json:"fifteen"`
}

type ServiceState struct {
	Name     string `json:"name"`
	Active   bool   `json:"active"`
	RawState string `json:"raw_state"`
}
