package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"gamestats/go-ingestion/internal/analysis"
	"gamestats/go-ingestion/internal/matches"
)

func TestResultsLifecycle(t *testing.T) {
	var snapshots []matches.EventSnapshot
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var s matches.EventSnapshot
		if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
			t.Error(err)
		}
		snapshots = append(snapshots, s)
		json.NewEncoder(w).Encode(analysis.Result{GameID: s.GameID, MatchID: s.MatchID, State: s.State, Metrics: json.RawMessage(`[{"from":"scala"}]`), Patterns: json.RawMessage(`[]`)})
	}))
	defer server.Close()
	client, _ := analysis.NewClient(server.URL, 0)
	handler := NewHandlerWithAnalyzer(client)
	perform(t, handler, "GET", "/results/g/m", "", 404)
	for i, kind := range []string{"MATCH_STARTED", "CUSTOM", "MATCH_FINISHED"} {
		perform(t, handler, "POST", "/events", eventBody("g", "m", kind, kind), 200)
		response := perform(t, handler, "GET", "/results/g/m", "", 200)
		var result analysis.Result
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		state := "open"
		if i == 2 {
			state = "closed"
		}
		if result.State != state || string(result.Metrics) != `[{"from":"scala"}]` || len(snapshots[i].Events) != i+1 {
			t.Fatalf("result: %+v", result)
		}
	}
	for _, s := range snapshots {
		for _, e := range s.Events {
			if string(e.Data) != `{"unknown":[true,null,{"large":9007199254740993}]}` {
				t.Fatal("data changed")
			}
		}
	}
}

func TestResultsFailures(t *testing.T) {
	for _, mode := range []string{"unconfigured", "unavailable", "timeout", "invalid", "HTTP error", "redirect"} {
		t.Run(mode, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch mode {
				case "timeout":
					io.Copy(io.Discard, r.Body)
					<-r.Context().Done()
				case "HTTP error":
					http.Error(w, "sensitive details", 500)
				case "redirect":
					http.Redirect(w, r, "/elsewhere", 307)
				default:
					io.WriteString(w, "invalid sensitive details")
				}
			}))
			defer server.Close()
			url := server.URL
			if mode == "unavailable" {
				server.Close()
			}
			client, _ := analysis.NewClient(url, 100*time.Millisecond)
			var analyzer Analyzer = client
			status := 502
			if mode == "unconfigured" {
				analyzer = nil
				status = 503
			}
			if mode == "timeout" {
				status = 504
			}
			handler := NewHandlerWithAnalyzer(analyzer)
			perform(t, handler, "POST", "/events", eventBody("g", "m", "start", "MATCH_STARTED"), 200)
			response := perform(t, handler, "GET", "/results/g/m", "", status)
			if strings.Contains(response.Body.String(), "sensitive") || strings.Contains(response.Body.String(), url) {
				t.Fatal("internal details exposed")
			}
			perform(t, handler, "GET", "/results/g/missing", "", 404)
			inspect(t, handler, "g", "m", "open", []string{"start"})
		})
	}
}

func TestResultsConcurrentIngestion(t *testing.T) {
	entered, release := make(chan struct{}, 8), make(chan struct{})
	// release must close before server shutdown.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var s matches.EventSnapshot
		if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
			t.Error(err)
		}
		entered <- struct{}{}
		<-release
		if s.State != "open" || len(s.Events) != 1 || s.Events[0].EventID != "start" {
			t.Errorf("inconsistent snapshot: %+v", s)
		}
		json.NewEncoder(w).Encode(analysis.Result{GameID: s.GameID, MatchID: s.MatchID, State: s.State, Metrics: json.RawMessage(`[]`), Patterns: json.RawMessage(`[]`)})
	}))
	defer server.Close()
	defer close(release)
	client, _ := analysis.NewClient(server.URL, 3*time.Second)
	handler := NewHandlerWithAnalyzer(client)
	perform(t, handler, "POST", "/events", eventBody("g", "m", "start", "MATCH_STARTED"), 200)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest("GET", "/results/g/m", nil))
			if response.Code != 200 {
				t.Errorf("results: %d", response.Code)
			}
		}()
	}
	for i := 0; i < 8; i++ {
		select {
		case <-entered:
		case <-time.After(5 * time.Second):
			t.Fatal("analysis did not start")
		}
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 30; i++ {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest("POST", "/events", strings.NewReader(eventBody("g", "m", fmt.Sprint(i), "CUSTOM"))))
			if response.Code != 200 {
				t.Errorf("ingestion: %d", response.Code)
			}
		}
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("HTTP analysis holds match mutex")
	}
	perform(t, handler, "POST", "/events", eventBody("g", "m", "finish", "MATCH_FINISHED"), 200)
	for i := 0; i < 8; i++ {
		release <- struct{}{}
	}
	wg.Wait()
	response := perform(t, handler, "GET", "/matches/g/m", "", 200)
	var summary matches.Snapshot
	json.Unmarshal(response.Body.Bytes(), &summary)
	if summary.State != "closed" || summary.EventCount != 32 {
		t.Fatalf("lost events: %+v", summary)
	}
}
