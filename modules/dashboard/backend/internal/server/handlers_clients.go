package server

import (
	"net/http"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/collector"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/envelope"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/state"
)

func handleClients(st *state.State) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		data, updated := st.SnapshotClients()
		stale := state.IsStale(updated, collector.Medium.Interval())
		envelope.WriteJSON(w, http.StatusOK, map[string]any{"clients": data}, updated, stale)
	}
}

func handleClientDetail(st *state.State) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := r.PathValue("ip")
		client, updated, ok := st.SnapshotClient(ip)
		if !ok {
			envelope.WriteJSON(w, http.StatusNotFound, map[string]any{"error": "client not found", "ip": ip}, updated, false)
			return
		}
		stale := state.IsStale(updated, collector.Medium.Interval())
		envelope.WriteJSON(w, http.StatusOK, client, updated, stale)
	}
}
