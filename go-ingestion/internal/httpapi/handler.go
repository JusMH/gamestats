// Package httpapi exposes health, ingestion and match inspection endpoints.
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"gamestats/go-ingestion/internal/analysis"
	"log"
	"net"
	"net/http"

	"gamestats/go-ingestion/internal/events"
	"gamestats/go-ingestion/internal/matches"
)

const maxBodyBytes = 1 << 20 // 1 MiB per event.

// Analyzer separates request handling from the external transport.
type Analyzer interface {
	Analyze(context.Context, matches.EventSnapshot) (analysis.Result, error)
}

type api struct {
	matches  *matches.Manager
	analyzer Analyzer
}

// NewHandler creates an independent in-memory store shared by its requests.
func NewHandler() http.Handler {
	return NewHandlerWithAnalyzer(nil)
}

// NewHandlerWithAnalyzer enables on-demand analysis; nil means unconfigured.
func NewHandlerWithAnalyzer(analyzer Analyzer) http.Handler {
	service := &api{matches: matches.NewManager(), analyzer: analyzer}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", health)
	mux.HandleFunc("/events", service.acceptEvent)
	mux.HandleFunc("/matches/{gameId}/{matchId}", service.getMatch)
	mux.HandleFunc("/results/{gameId}/{matchId}", service.getResults)
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

func (a *api) getResults(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	snapshot, err := a.matches.GetEvents(r.PathValue("gameId"), r.PathValue("matchId"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "match not found"})
		return
	}
	if a.analyzer == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "analysis service is not configured"})
		return
	}
	result, err := a.analyzer.Analyze(r.Context(), snapshot)
	if err != nil {
		// A disconnected caller no longer needs a response. The context has already
		// canceled the upstream request; cancellation is not a gateway failure.
		if r.Context().Err() == context.Canceled {
			return
		}
		status, message := http.StatusBadGateway, "analysis service failed"
		var networkError net.Error
		if errors.Is(err, context.DeadlineExceeded) || (errors.As(err, &networkError) && networkError.Timeout()) {
			status, message = http.StatusGatewayTimeout, "analysis service timed out"
		}
		writeJSON(w, status, map[string]string{"error": message})
		return
	}
	writeJSON(w, http.StatusOK, result)
}
