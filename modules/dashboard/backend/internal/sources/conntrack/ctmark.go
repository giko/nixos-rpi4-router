package conntrack

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

// ClientFwmarks runs `conntrack -L` and returns a map of src_ip to fwmark (hex).
// Flows with mark=0 (WAN / unmarked) are excluded. The first-seen mark wins
// per IP, so the map reflects the earliest flow for each client.
func ClientFwmarks(ctx context.Context, run Runner) (map[string]string, error) {
	out, err := run(ctx, "-L")
	if err != nil {
		return nil, fmt.Errorf("clientfwmarks: %w", err)
	}

	result := make(map[string]string)
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}

		src := extractField(line, "src")
		markStr := extractField(line, "mark")
		if src == "" || markStr == "" {
			continue
		}

		mark, err := strconv.ParseUint(markStr, 10, 64)
		if err != nil || mark == 0 {
			continue
		}

		hex := fmt.Sprintf("0x%x", mark)
		if _, exists := result[src]; !exists {
			result[src] = hex
		}
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
