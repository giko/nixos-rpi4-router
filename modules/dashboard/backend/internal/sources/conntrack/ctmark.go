package conntrack

import (
	"context"
	"fmt"
	"strconv"
	"strings"
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

		src := extractField(line, "src")
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
