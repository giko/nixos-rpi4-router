package server

import (
	"net/http"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/envelope"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/state"
)

func handleQoS(st *state.State) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		data, updated := st.SnapshotQoS()
		stale := state.IsStale(updated, qosStaleAfter/2)
		envelope.WriteJSON(w, http.StatusOK, data, updated, stale)
	}
}
