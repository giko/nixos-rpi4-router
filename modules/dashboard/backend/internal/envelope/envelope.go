// Package envelope wraps every JSON API response in a standard shape:
//
//	{ "data": ..., "updated_at": "...", "stale": bool }
//
// A zero updated_at renders as JSON null.
package envelope

import (
	"encoding/json"
	"net/http"
	"time"
)

type response struct {
	Data      any     `json:"data"`
	UpdatedAt *string `json:"updated_at"`
	Stale     bool    `json:"stale"`
}

// WriteJSON writes a JSON-encoded envelope to w with the given HTTP status.
// If updated is the zero time, updated_at is rendered as JSON null.
func WriteJSON(w http.ResponseWriter, status int, data any, updated time.Time, stale bool) {
	env := response{
		Data:  data,
		Stale: stale,
	}
	if !updated.IsZero() {
		s := updated.UTC().Format(time.RFC3339)
		env.UpdatedAt = &s
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(env)
}
