package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"gamestats/go-ingestion/internal/events"
	"gamestats/go-ingestion/internal/matches"
)

func TestCapacityHTTPRejectsBeforeMutationAndStillAllowsClosure(t *testing.T) {
	m := matches.NewManager()
	e := events.Event{SchemaVersion: 1, EventID: "start", Timestamp: "2026-10-18T12:00:00Z", GameID: "generic", MatchID: "capacity", Type: "MATCH_STARTED", Data: json.RawMessage(`{}`)}
	for i := 0; i < matches.MaxEvents-1; i++ {
		if i > 0 {
			e.EventID = fmt.Sprint(i)
			e.Type = "CUSTOM"
		}
		if err := m.Accept(e); err != nil {
			t.Fatal(err)
		}
	}
	a := &api{matches: m}
	for _, tc := range []struct {
		kind   string
		status int
	}{{"CUSTOM", 413}, {"MATCH_FINISHED", 200}} {
		e.EventID = "retry"
		e.Type = tc.kind
		b, _ := json.Marshal(e)
		w := httptest.NewRecorder()
		a.acceptEvent(w, httptest.NewRequest(http.MethodPost, "/events", bytes.NewReader(b)))
		if w.Code != tc.status {
			t.Fatalf("%s: status %d, %s", tc.kind, w.Code, w.Body.String())
		}
		if tc.status == 413 {
			var body map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || body["status"] != "rejected" || body["error"] == "" {
				t.Fatal(w.Body.String())
			}
		}
	}
	s, _ := m.Get("generic", "capacity")
	if s.EventCount != matches.MaxEvents || s.State != "closed" {
		t.Fatalf("%+v", s)
	}
}
