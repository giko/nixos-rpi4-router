package model

type Pool struct {
	Name               string       `json:"name"`
	Members            []PoolMember `json:"members"`
	ClientIPs          []string     `json:"client_ips"`
	FailsafeDropActive bool         `json:"failsafe_drop_active"`
}

type PoolMember struct {
	Tunnel    string `json:"tunnel"`
	Fwmark    string `json:"fwmark"`
	Healthy   bool   `json:"healthy"`
	FlowCount int    `json:"flow_count"`
}
