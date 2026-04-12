package vcgencmd

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Collect runs `vcgencmd get_throttled` and returns the raw hex value and
// whether any throttling bits are set.
func Collect(ctx context.Context) (string, bool, error) {
	cmd := exec.CommandContext(ctx, "vcgencmd", "get_throttled")
	out, err := cmd.Output()
	if err != nil {
		return "", false, fmt.Errorf("vcgencmd get_throttled: %w", err)
	}

	raw, flag := parseThrottled(strings.TrimSpace(string(out)))
	return raw, flag, nil
}

// parseThrottled splits a "throttled=0x..." line and returns the hex value
// and whether it indicates any throttling (non-zero).
func parseThrottled(line string) (string, bool) {
	_, value, found := strings.Cut(line, "=")
	if !found {
		return line, false
	}
	value = strings.TrimSpace(value)
	return value, value != "0x0"
}
