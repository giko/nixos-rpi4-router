package server

import (
	"net/http"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/collector"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/envelope"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/state"
)

func handleTunnels(st *state.State) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		data, updated := st.SnapshotTunnels()
		stale := state.IsStale(updated, collector.Hot.Interval())
		envelope.WriteJSON(w, http.StatusOK, map[string]any{"tunnels": data}, updated, stale)
	}
}
