package httpapi

import (
	"encoding/json"
	"gamestats/go-ingestion/internal/analysis"
	"gamestats/go-ingestion/internal/matches"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDashboardDelegatesAllSnapshotsToScala(t *testing.T) {
	var observed []matches.EventSnapshot
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/analyze-summary" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		var input struct {
			Matches []matches.EventSnapshot `json:"matches"`
		}
		if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
			t.Error(err)
		}
		observed = input.Matches
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"source":"GameStats / Scala","totals":{"started":2},"games":[],"matches":[]}`))
	}))
	defer upstream.Close()
	client, _ := analysis.NewClient(upstream.URL, 0)
	handler := NewHandlerWithAnalyzer(client)
	perform(t, handler, "POST", "/events", eventBody("snake", "a", "s", "MATCH_STARTED"), 200)
	perform(t, handler, "POST", "/events", eventBody("racing", "b", "s", "MATCH_STARTED"), 200)
	perform(t, handler, "GET", "/dashboard", "", 200)
	if len(observed) != 2 {
		t.Fatalf("expected 2 snapshots, got %d", len(observed))
	}
	perform(t, handler, "POST", "/dashboard", "{}", 405)
	perform(t, NewHandler(), "GET", "/dashboard", "", 503)
}
