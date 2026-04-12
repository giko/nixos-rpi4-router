package poolhealth

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// State holds the parsed pool-health state from the JSON file written by the
// wg-pool-health service.
type State struct {
	UpdatedAt string                `json:"updated_at"`
	Tunnels   map[string]TunnelInfo `json:"tunnels"`
}

// TunnelInfo holds health status for a single WireGuard tunnel.
type TunnelInfo struct {
	Healthy             bool `json:"healthy"`
	ConsecutiveFailures int  `json:"consecutive_failures"`
}

// ReadState reads and parses the pool-health JSON state file at path.
//
// If the file does not exist, an empty State with an initialized (empty) map
// is returned without error. Malformed JSON returns an error.
func ReadState(path string) (State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return State{Tunnels: make(map[string]TunnelInfo)}, nil
		}
		return State{}, fmt.Errorf("readstate: %w", err)
	}

	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return State{}, fmt.Errorf("readstate: %w", err)
	}

	if s.Tunnels == nil {
		s.Tunnels = make(map[string]TunnelInfo)
	}

	return s, nil
}
