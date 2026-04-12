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

type Response struct {
	Data      any        `json:"data"`
	UpdatedAt *time.Time `json:"updated_at"`
	Stale     bool       `json:"stale"`
}

// WriteJSON writes a JSON-encoded envelope to w with the given HTTP status.
// If updated is the zero time, updated_at is rendered as JSON null.
func WriteJSON(w http.ResponseWriter, status int, data any, updated time.Time, stale bool) {
	env := Response{
		Data:  data,
		Stale: stale,
	}
	if !updated.IsZero() {
		u := updated.UTC()
		env.UpdatedAt = &u
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(env)
}
