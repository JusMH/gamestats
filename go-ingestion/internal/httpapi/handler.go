// Package httpapi exposes health, ingestion and match inspection endpoints.
package httpapi

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"

	"gamestats/go-ingestion/internal/events"
	"gamestats/go-ingestion/internal/matches"
)

const maxBodyBytes = 1 << 20 // 1 MiB per event.

type api struct{ matches *matches.Manager }

// NewHandler creates an independent in-memory store shared by its requests.
func NewHandler() http.Handler {
	service := &api{matches: matches.NewManager()}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", health)
	mux.HandleFunc("/events", service.acceptEvent)
	mux.HandleFunc("/matches/{gameId}/{matchId}", service.getMatch)
	return mux
}

func health(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (a *api) acceptEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	defer r.Body.Close()
	event, err := events.Decode(r.Body)
	if err != nil {
		rejectEvent(w, http.StatusBadRequest, err)
		return
	}
	if err := a.matches.Accept(event); err != nil {
		status := http.StatusConflict
		if errors.Is(err, matches.ErrNotFound) {
			status = http.StatusNotFound
		}
		rejectEvent(w, status, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "accepted", "eventId": event.EventID})
}

func (a *api) getMatch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	snapshot, err := a.matches.Get(r.PathValue("gameId"), r.PathValue("matchId"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, snapshot)
}

func rejectEvent(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"status": "rejected", "error": err.Error()})
}

func methodNotAllowed(w http.ResponseWriter, allowed string) {
	w.Header().Set("Allow", allowed)
	writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed; use " + allowed})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("writing HTTP response: %v", err)
	}
}
