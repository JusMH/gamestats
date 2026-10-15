package analysis

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"gamestats/go-ingestion/internal/events"
	"gamestats/go-ingestion/internal/matches"
)

func TestClientRequestAndResult(t *testing.T) {
	snapshot := matches.EventSnapshot{GameID: "snake", MatchID: "m1", State: "open", Events: []events.Event{{SchemaVersion: 1, EventID: "e1", Timestamp: "2026-10-10T18:00:00Z", GameID: "snake", MatchID: "m1", GameVersion: "v1", PlayerID: "p1", Type: "CUSTOM", Data: json.RawMessage(`{"nested":[null,true,{"large":9007199254740993}]}`)}}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/analyze" || r.Header.Get("Content-Type") != "application/json" {
			t.Error("invalid HTTP request")
		}
		var got matches.EventSnapshot
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Error(err)
		}
		if !reflect.DeepEqual(got, snapshot) {
			t.Errorf("payload changed: %+v", got)
		}
		io.WriteString(w, `{"gameId":"snake","matchId":"m1","state":"open","metrics":[{"value":9007199254740993}],"patterns":[]}`)
	}))
	defer server.Close()
	client, err := NewClient(server.URL+"/", 0)
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Analyze(context.Background(), snapshot)
	if err != nil || string(result.Metrics) != `[{"value":9007199254740993}]` {
		t.Fatalf("result: %+v %v", result, err)
	}
}

func TestClientRejectsInvalidResponses(t *testing.T) {
	for _, body := range []string{
		`{`, `null`, `[]`, `{}`, `{"gameId":"g","matchId":"m","state":"open","metrics":null,"patterns":[]}`,
		`{"gameId":"g","matchId":"m","state":"open","metrics":[],"patterns":{}}`,
		`{"gameId":"other","matchId":"m","state":"open","metrics":[],"patterns":[]}`,
		`{"gameId":"g","matchId":"other","state":"open","metrics":[],"patterns":[]}`,
		`{"gameId":"g","matchId":"m","state":"closed","metrics":[],"patterns":[]}`,
		`{"gameId":"g","matchId":"m","state":"open","metrics":[],"patterns":[]} {}`,
		strings.Repeat(" ", maxResponseBytes+1),
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, body) }))
		client, _ := NewClient(server.URL, 0)
		_, err := client.Analyze(context.Background(), matches.EventSnapshot{GameID: "g", MatchID: "m", State: "open"})
		server.Close()
		if !errors.Is(err, ErrInvalidResponse) {
			t.Fatalf("accepted invalid response (%d bytes): %v", len(body), err)
		}
	}
}

func TestClientCancellation(t *testing.T) {
	started := make(chan struct{})
	canceled := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		close(started)
		<-r.Context().Done()
		close(canceled)
	}))
	defer server.Close()
	client, _ := NewClient(server.URL, time.Second)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { _, err := client.Analyze(ctx, matches.EventSnapshot{}); done <- err }()
	select {
	case <-started:
	case err := <-done:
		t.Fatalf("request failed before reaching mock: %v", err)
	case <-time.After(3 * time.Second):
		t.Fatal("mock did not receive request")
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	select {
	case <-canceled:
	case <-time.After(3 * time.Second):
		t.Fatal("upstream was not canceled")
	}
}

func TestClientConfiguration(t *testing.T) {
	for _, address := range []string{"", "localhost:9000", "ftp://host", "http://user:password@host", "http://host?token=x", "http://host/#fragment"} {
		if _, err := NewClient(address, 0); err == nil {
			t.Errorf("accepted %q", address)
		}
	}
	if _, err := NewClient("http://localhost", -time.Second); err == nil {
		t.Fatal("negative timeout")
	}
}
