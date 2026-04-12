package conntrack

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Runner executes a conntrack command and returns its stdout.
type Runner func(ctx context.Context, args ...string) (string, error)

// DefaultRunner exec-invokes the real conntrack binary.
func DefaultRunner(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "conntrack", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("conntrack %s: %w", strings.Join(args, " "), err)
	}
	return string(out), nil
}

// CountByFwmark counts active flows matching the given fwmark.
// It runs `conntrack -L -m <fwmark>` and counts non-empty lines in stdout.
func CountByFwmark(ctx context.Context, run Runner, fwmark string) (int, error) {
	out, err := run(ctx, "-L", "-m", fwmark)
	if err != nil {
		return 0, fmt.Errorf("countbyfwmark: %w", err)
	}

	if strings.TrimSpace(out) == "" {
		return 0, nil
	}

	n := 0
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) != "" {
			n++
		}
	}
	return n, nil
}
