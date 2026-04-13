package ipneigh

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Runner executes a command and returns its stdout.
type Runner func(ctx context.Context, args ...string) (string, error)

// Entry is a single parsed neighbour-table entry. Dev is the interface
// name from the `dev <iface>` field so callers can filter out neighbours
// learned on WAN or other non-LAN interfaces — otherwise the upstream
// gateway's ARP entry becomes a synthetic "neighbor" client.
type Entry struct {
	IP  string
	MAC string
	Dev string
}

// DefaultRunner exec-invokes the real ip binary.
func DefaultRunner(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "ip", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("ip %s: %w", strings.Join(args, " "), err)
	}
	return string(out), nil
}

// Collect runs `ip neigh show` and returns entries with IP, MAC, and dev
// (interface) preserved. FAILED and INCOMPLETE entries are filtered out.
// The result is keyed by IP; if the same IP appears twice the last entry
// wins, matching the historical behaviour of the map-returning API.
func Collect(ctx context.Context, run Runner) (map[string]Entry, error) {
	out, err := run(ctx, "neigh", "show")
	if err != nil {
		return nil, fmt.Errorf("collect: %w", err)
	}
	return parseNeigh(out), nil
}

// parseNeigh parses raw `ip neigh show` output. FAILED and INCOMPLETE
// entries are skipped. MACs are lowercased. Returned map is keyed by IP.
func parseNeigh(raw string) map[string]Entry {
	result := make(map[string]Entry)
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 1 {
			continue
		}

		ip := fields[0]
		// Skip IPv6.
		if strings.Contains(ip, ":") {
			continue
		}

		// Check last field for state.
		state := fields[len(fields)-1]
		if state == "FAILED" || state == "INCOMPLETE" {
			continue
		}

		// Extract optional dev <iface> and required lladdr <mac>.
		var mac, dev string
		for i, f := range fields {
			if f == "lladdr" && i+1 < len(fields) {
				mac = strings.ToLower(fields[i+1])
			}
			if f == "dev" && i+1 < len(fields) {
				dev = fields[i+1]
			}
		}
		if mac == "" {
			continue
		}

		result[ip] = Entry{IP: ip, MAC: mac, Dev: dev}
	}
	return result
}
