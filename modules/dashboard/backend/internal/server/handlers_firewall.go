package server

import (
	"net/http"
	"time"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/envelope"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/state"
)

// Stale-window constants derived from spec §8.4 frontend poll cadence × 2.
// Using per-endpoint windows (rather than one tier-wide value) keeps the
// stale badge honest for data that barely changes (rules) vs data that
// ticks fast (counters).
const (
	firewallRulesStaleAfter    = 60 * time.Second // spec: 30 s poll × 2
	firewallCountersStaleAfter = 10 * time.Second // spec:  5 s poll × 2
	upnpStaleAfter             = 30 * time.Second // spec: 15 s poll × 2
	qosStaleAfter              = 10 * time.Second // spec:  5 s poll × 2
)

// handleFirewallRules serves the static-ish Firewall projection:
// port forwards, PBR rules, allowed MACs, and the rolled-up 1h
// forward-drop count. Spec §7.4: "{port_forwards, pbr, allowed_macs,
// blocked_forward_count_1h}".
func handleFirewallRules(st *state.State) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		fw, updated := st.SnapshotFirewall()
		stale := state.IsStale(updated, firewallRulesStaleAfter/2)
		body := map[string]any{
			"port_forwards":            fw.PortForwards,
			"pbr":                      fw.PBR,
			"allowed_macs":             fw.AllowedMACs,
			"blocked_forward_count_1h": fw.BlockedForwardCount1h,
		}
		envelope.WriteJSON(w, http.StatusOK, body, updated, stale)
	}
}

// handleFirewallCounters serves the dynamic counters view —
// {chains: [ {family, table, name, hook, policy, counters: [{handle, comment, packets, bytes}]} ]}.
// Spec §7.4: "{chains: [...]}".
func handleFirewallCounters(st *state.State) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		fw, updated := st.SnapshotFirewall()
		stale := state.IsStale(updated, firewallCountersStaleAfter/2)
		body := struct {
			Chains any `json:"chains"`
		}{Chains: fw.Chains}
		envelope.WriteJSON(w, http.StatusOK, body, updated, stale)
	}
}

// handleUPnP serves active UPnP mappings extracted from the
// inet/miniupnpd table. Spec §7.4: "{leases: [...]}".
func handleUPnP(st *state.State) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		fw, updated := st.SnapshotFirewall()
		stale := state.IsStale(updated, upnpStaleAfter/2)
		body := struct {
			Leases any `json:"leases"`
		}{Leases: fw.UPnPLeases}
		envelope.WriteJSON(w, http.StatusOK, body, updated, stale)
	}
}
