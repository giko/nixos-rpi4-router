// Package iputil holds small IP classification helpers shared across
// sources. It exists so the conntrack and clients collectors agree on
// what "LAN" means without one package importing the other.
package iputil

import "net"

// IsRFC1918 reports whether the given IPv4 address string belongs to any
// of the RFC1918 private ranges: 10.0.0.0/8, 172.16.0.0/12, or
// 192.168.0.0/16. Any non-IPv4 input returns false. This is a
// deliberately conservative superset rather than a tight match against
// the router's configured LAN subnets — callers that need the narrower
// definition can layer their own check on top.
func IsRFC1918(ip string) bool {
	addr := net.ParseIP(ip)
	if addr == nil {
		return false
	}
	v4 := addr.To4()
	if v4 == nil {
		return false
	}
	if v4[0] == 10 {
		return true
	}
	if v4[0] == 172 && v4[1] >= 16 && v4[1] <= 31 {
		return true
	}
	if v4[0] == 192 && v4[1] == 168 {
		return true
	}
	return false
}
