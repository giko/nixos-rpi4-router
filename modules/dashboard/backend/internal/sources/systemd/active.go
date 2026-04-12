package systemd

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/model"
)

// Collect runs `systemctl is-active` for the given units and returns their
// states. Exit code 3 ("some unit inactive") is expected and still parsed.
// An empty stdout with any exit code is treated as a real error.
func Collect(ctx context.Context, units []string) ([]model.ServiceState, error) {
	if len(units) == 0 {
		return nil, nil
	}

	args := append([]string{"is-active"}, units...)
	cmd := exec.CommandContext(ctx, "systemctl", args...)
	out, err := cmd.Output()

	stdout := string(out)
	if stdout == "" {
		if err != nil {
			return nil, fmt.Errorf("systemctl is-active: %w", err)
		}
		return nil, fmt.Errorf("systemctl is-active: empty output")
	}

	// Exit code 3 means "some unit is inactive" -- not a real error.
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 3 {
			// expected, parse the output
		} else {
			return nil, fmt.Errorf("systemctl is-active: %w", err)
		}
	}

	return parseIsActive(units, stdout), nil
}

// parseIsActive maps positional one-line-per-unit output from
// `systemctl is-active` to ServiceState values.
func parseIsActive(units []string, stdout string) []model.ServiceState {
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	result := make([]model.ServiceState, 0, len(units))
	for i, name := range units {
		var rawState string
		if i < len(lines) {
			rawState = strings.TrimSpace(lines[i])
		}
		result = append(result, model.ServiceState{
			Name:     name,
			Active:   rawState == "active",
			RawState: rawState,
		})
	}
	return result
}
