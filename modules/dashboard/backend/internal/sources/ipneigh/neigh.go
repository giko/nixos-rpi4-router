package ipneigh

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Runner executes a command and returns its stdout.
type Runner func(ctx context.Context, args ...string) (string, error)

// DefaultRunner exec-invokes the real ip binary.
func DefaultRunner(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "ip", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("ip %s: %w", strings.Join(args, " "), err)
	}
	return string(out), nil
}

// Collect runs `ip neigh show` and returns a map of IPv4 to MAC.
// FAILED and INCOMPLETE entries are filtered out.
func Collect(ctx context.Context, run Runner) (map[string]string, error) {
	out, err := run(ctx, "neigh", "show")
	if err != nil {
		return nil, fmt.Errorf("collect: %w", err)
	}
	return parseNeigh(out), nil
}

// parseNeigh parses raw `ip neigh show` output into a map of IP to MAC.
// FAILED and INCOMPLETE entries are skipped. MACs are lowercased.
func parseNeigh(raw string) map[string]string {
	result := make(map[string]string)
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

		// Find lladdr keyword followed by MAC.
		mac := ""
		for i, f := range fields {
			if f == "lladdr" && i+1 < len(fields) {
				mac = strings.ToLower(fields[i+1])
				break
			}
		}
		if mac == "" {
			continue
		}

		result[ip] = mac
	}
	return result
}
