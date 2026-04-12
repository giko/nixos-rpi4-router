package model

type Tunnel struct {
	Name                      string `json:"name"`
	Interface                 string `json:"interface"`
	Fwmark                    string `json:"fwmark"`
	RoutingTable              int    `json:"routing_table"`
	PublicKey                 string `json:"public_key"`
	Endpoint                  string `json:"endpoint"`
	LatestHandshakeSecondsAgo int64  `json:"latest_handshake_seconds_ago"`
	RXBytes                   uint64 `json:"rx_bytes"`
	TXBytes                   uint64 `json:"tx_bytes"`
	Healthy                   bool   `json:"healthy"`
	ConsecutiveFailures       int    `json:"consecutive_failures"`
}
