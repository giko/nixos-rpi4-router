package server

import (
	"net/http"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/collector"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/envelope"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/state"
)

func handleSystem(st *state.State) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		data, updated := st.SnapshotSystem()
		stale := state.IsStale(updated, collector.Hot.Interval())
		envelope.WriteJSON(w, http.StatusOK, data, updated, stale)
	}
}
