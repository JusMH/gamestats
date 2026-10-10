package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"sync"
	"testing"

	"gamestats/go-ingestion/internal/events"
	"gamestats/go-ingestion/internal/matches"
)

func eventBody(gameID, matchID, id, kind string) string {
	event := events.Event{
		SchemaVersion: 1, EventID: id, Timestamp: "2026-10-10T18:00:00Z",
		GameID: gameID, MatchID: matchID, Type: kind,
		Data: json.RawMessage(`{"unknown":[true,null,{"large":9007199254740993}]}`),
	}
	body, err := json.Marshal(event)
	if err != nil {
		panic(err)
	} // This fixture contains only JSON-serializable values.
	return string(body)
}

func perform(t *testing.T, handler http.Handler, method, path, body string, status int) *httptest.ResponseRecorder {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(method, path, strings.NewReader(body)))
	if response.Code != status {
		t.Fatalf("%s %s: got %d, want %d; %s", method, path, response.Code, status, response.Body.String())
	}
	if response.Header().Get("Content-Type") != "application/json" {
		t.Fatal("response must be JSON")
	}
	return response
}

// Valid envelope cases from implementation 1 now need an existing match,
// except when the tested event itself starts the match.
func prepareMatch(t *testing.T, handler http.Handler, body string) {
	t.Helper()
	var event events.Event
	if err := json.Unmarshal([]byte(body), &event); err != nil {
		t.Fatal(err)
	}
	if event.Type != "MATCH_STARTED" {
		perform(t, handler, http.MethodPost, "/events",
			eventBody(event.GameID, event.MatchID, "_setup-start", "MATCH_STARTED"), http.StatusOK)
	}
}

func inspect(t *testing.T, handler http.Handler, gameID, matchID, state string, ids []string) {
	t.Helper()
	path := "/matches/" + url.PathEscape(gameID) + "/" + url.PathEscape(matchID)
	response := perform(t, handler, http.MethodGet, path, "", http.StatusOK)
	var snapshot matches.Snapshot
	if err := json.Unmarshal(response.Body.Bytes(), &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.GameID != gameID || snapshot.MatchID != matchID || snapshot.State != state ||
		snapshot.EventCount != len(ids) || !reflect.DeepEqual(snapshot.EventIDs, ids) {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
}

func TestHTTPMatchLifecycle(t *testing.T) {
	handler := NewHandler()
	cases := []struct {
		id, kind  string
		status    int
		errorText string
	}{
		{"early", "CUSTOM", 404, "MATCH_STARTED"},
		{"early-finish", "MATCH_FINISHED", 404, "MATCH_STARTED"},
		{"start", "MATCH_STARTED", 200, ""},
		{"start", "MATCH_STARTED", 409, "eventId"},
		{"start2", "MATCH_STARTED", 409, "already"},
		{"e1", "CUSTOM", 200, ""},
		{"e1", "OTHER", 409, "eventId"},
		{"e2", "OTHER", 200, ""},
		{"finish", "MATCH_FINISHED", 200, ""},
		{"late", "CUSTOM", 409, "closed"},
		{"new-start", "MATCH_STARTED", 409, "closed"},
		{"new-finish", "MATCH_FINISHED", 409, "closed"},
		{"finish", "MATCH_FINISHED", 409, "eventId"},
	}
	ids := []string{}
	state := "open"
	for _, tc := range cases {
		t.Run(tc.id+"-"+tc.kind, func(t *testing.T) {
			response := perform(t, handler, http.MethodPost, "/events", eventBody("unknown", "m1", tc.id, tc.kind), tc.status)
			var result map[string]string
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if tc.status == 200 {
				if result["status"] != "accepted" || result["eventId"] != tc.id {
					t.Fatalf("unexpected response: %v", result)
				}
				ids = append(ids, tc.id)
				if tc.kind == "MATCH_FINISHED" {
					state = "closed"
				}
			} else if result["status"] != "rejected" || !strings.Contains(result["error"], tc.errorText) {
				t.Fatalf("unexpected rejection: %v", result)
			}
			if len(ids) == 0 {
				perform(t, handler, http.MethodGet, "/matches/unknown/m1", "", 404)
			} else {
				inspect(t, handler, "unknown", "m1", state, ids)
			}
		})
	}
	// A validation failure also leaves stored state unchanged.
	perform(t, handler, http.MethodPost, "/events", "{", 400)
	inspect(t, handler, "unknown", "m1", "closed", ids)
}

func TestHTTPMatchIsolation(t *testing.T) {
	handler := NewHandler()
	for _, gameID := range []string{"combat", "racing", "new/game"} {
		perform(t, handler, http.MethodPost, "/events", eventBody(gameID, "shared", "start", "MATCH_STARTED"), 200)
		perform(t, handler, http.MethodPost, "/events", eventBody(gameID, "shared", "e1", "CUSTOM"), 200)
		inspect(t, handler, gameID, "shared", "open", []string{"start", "e1"})
	}
	perform(t, handler, http.MethodPost, "/events", eventBody("combat", "shared", "finish", "MATCH_FINISHED"), 200)
	inspect(t, handler, "racing", "shared", "open", []string{"start", "e1"})
	perform(t, handler, http.MethodGet, "/matches/missing/shared", "", 404)
	// Handlers own separate stores; there is no package-global mutable state.
	perform(t, NewHandler(), http.MethodGet, "/matches/combat/shared", "", 404)
}

func TestHTTPConcurrentMatches(t *testing.T) {
	server := httptest.NewServer(NewHandler())
	defer server.Close()
	client := server.Client()
	const workers, perMatch = 8, 20
	ready := make(chan struct{})
	var wg sync.WaitGroup
	send := func(body string) error {
		response, err := client.Post(server.URL+"/events", "application/json", strings.NewReader(body))
		if err != nil {
			return err
		}
		defer response.Body.Close()
		var result map[string]string
		if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
			return err
		}
		if response.StatusCode != 200 || result["status"] != "accepted" {
			return fmt.Errorf("unexpected ingestion: %d %v", response.StatusCode, result)
		}
		return nil
	}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-ready
			gameID := fmt.Sprintf("game-%d", i)
			if err := send(eventBody(gameID, "shared", "start", "MATCH_STARTED")); err != nil {
				t.Error(err)
				return
			}
			for n := 0; n < perMatch; n++ {
				if err := send(eventBody(gameID, "shared", fmt.Sprintf("e-%d", n), "CUSTOM")); err != nil {
					t.Error(err)
					return
				}
			}
			if err := send(eventBody(gameID, "shared", "finish", "MATCH_FINISHED")); err != nil {
				t.Error(err)
			}
		}(i)
	}
	close(ready)
	wg.Wait()
	for i := 0; i < workers; i++ {
		response, err := client.Get(fmt.Sprintf("%s/matches/game-%d/shared", server.URL, i))
		if err != nil {
			t.Fatal(err)
		}
		var snapshot matches.Snapshot
		err = json.NewDecoder(response.Body).Decode(&snapshot)
		response.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != 200 || snapshot.GameID != fmt.Sprintf("game-%d", i) || snapshot.MatchID != "shared" ||
			snapshot.State != "closed" || snapshot.EventCount != perMatch+2 {
			t.Fatalf("unexpected final state: %+v", snapshot)
		}
		ids := []string{"start"}
		for n := 0; n < perMatch; n++ {
			ids = append(ids, fmt.Sprintf("e-%d", n))
		}
		ids = append(ids, "finish")
		if !reflect.DeepEqual(snapshot.EventIDs, ids) {
			t.Fatalf("lost or mixed events: %+v", snapshot)
		}
	}
	// An actual HTTP HEAD response has a status and Allow but no body.
	request, err := http.NewRequest(http.MethodHead, server.URL+"/matches/game-0/shared", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 405 || response.Header.Get("Allow") != "GET" || len(body) != 0 {
		t.Fatal("unexpected HEAD response")
	}
}
