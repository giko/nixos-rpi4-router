package server

import (
	"net/http"

	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/collector"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/envelope"
	"github.com/giko/nixos-rpi4-router/modules/dashboard/backend/internal/state"
)

func handleSystem(st *state.State) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		// Evaluate staleness per tier so a 5 s medium-tier cycle doesn't
		// drag the hot-tier freshness badge stale near the end of every
		// healthy window. The envelope's updated_at still reflects the
		// hot tier (the fields clients poll at 2 s), but `stale` is OR'd
		// across both tiers so a genuinely stuck medium collector still
		// surfaces.
		data, hotUpdated, medUpdated := st.SnapshotSystemTiers()
		stale := state.IsStale(hotUpdated, collector.Hot.Interval()) ||
			state.IsStale(medUpdated, collector.Medium.Interval())
		envelope.WriteJSON(w, http.StatusOK, data, hotUpdated, stale)
	}
}
