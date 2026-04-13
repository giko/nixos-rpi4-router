package conntrack

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/sources/iputil"
)

// ClientConnInfo holds per-client connection data derived from conntrack.
type ClientConnInfo struct {
	// TotalConns is the total number of tracked connections for this IP
	// (including mark=0 / WAN connections).
	TotalConns int
	// TunnelConns maps tunnel fwmark (hex) → connection count. Only
	// non-zero marks are included. In a round-robin pool, a single
	// client's connections are spread across all healthy tunnels.
	TunnelConns map[string]int
}

// ClientConnections runs `conntrack -L` and returns per-source-IP
// connection info. Every connection is counted (including WAN/mark=0).
// Tunnel-specific counts are keyed by hex fwmark.
func ClientConnections(ctx context.Context, run Runner) (map[string]ClientConnInfo, error) {
	out, err := run(ctx, "-L")
	if err != nil {
		return nil, fmt.Errorf("clientconns: %w", err)
	}

	result := make(map[string]ClientConnInfo)
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}

		src := attributeSrc(line)
		if src == "" {
			continue
		}

		info := result[src]
		info.TotalConns++

		markStr := extractField(line, "mark")
		if markStr != "" {
			mark, err := strconv.ParseUint(markStr, 10, 64)
			if err == nil && mark != 0 {
				if info.TunnelConns == nil {
					info.TunnelConns = make(map[string]int)
				}
				info.TunnelConns[fmt.Sprintf("0x%x", mark)]++
			}
		}
		result[src] = info
	}

	return result, nil
}

// attributeSrc picks the LAN endpoint of a conntrack line. `conntrack -L`
// prints the original tuple (src/dst) followed by the reply tuple
// (src/dst). For outbound traffic the original src is the LAN host.
// For inbound DNAT (port-forward) sessions, the original src is the
// remote peer's public IP and the reply src is the LAN host that
// actually holds the connection — so attributing to the original src
// leaves the LAN host with conn_count=0.
//
// Rule: if the original src is public and the reply src is RFC1918, use
// the reply src. Otherwise use the original src (the common case).
func attributeSrc(line string) string {
	origSrc, replySrc := extractOrigAndReplySrc(line)
	if origSrc == "" {
		return ""
	}
	if !iputil.IsRFC1918(origSrc) && replySrc != "" && iputil.IsRFC1918(replySrc) {
		return replySrc
	}
	return origSrc
}

// extractOrigAndReplySrc returns the first (original) and second (reply)
// src=VALUE fields on a conntrack line. Either may be empty.
func extractOrigAndReplySrc(line string) (string, string) {
	var first, second string
	const prefix = "src="
	for _, field := range strings.Fields(line) {
		if !strings.HasPrefix(field, prefix) {
			continue
		}
		v := field[len(prefix):]
		if first == "" {
			first = v
			continue
		}
		second = v
		break
	}
	return first, second
}

// extractField finds the first occurrence of "key=value" in a whitespace-
// separated line and returns the value. Returns "" if the key is not found.
func extractField(line, key string) string {
	prefix := key + "="
	for _, field := range strings.Fields(line) {
		if strings.HasPrefix(field, prefix) {
			return field[len(prefix):]
		}
	}
	return ""
}
