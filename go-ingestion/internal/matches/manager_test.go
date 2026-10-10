package matches

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"gamestats/go-ingestion/internal/events"
)

func testEvent(gameID, matchID, id, kind string) events.Event {
	return events.Event{
		SchemaVersion: 1, EventID: id, Timestamp: "2026-10-10T18:00:00Z",
		GameID: gameID, MatchID: matchID, Type: kind,
		Data: []byte(`{"unknown":[null,{"large":9007199254740993}]}`),
	}
}

func requireAccept(t *testing.T, manager *Manager, event events.Event) {
	t.Helper()
	if err := manager.Accept(event); err != nil {
		t.Fatal(err)
	}
}

func requireSnapshot(t *testing.T, manager *Manager, gameID, matchID, state string, ids []string) {
	t.Helper()
	snapshot, err := manager.Get(gameID, matchID)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.GameID != gameID || snapshot.MatchID != matchID || snapshot.State != state ||
		snapshot.EventCount != len(ids) || !reflect.DeepEqual(snapshot.EventIDs, ids) {
		t.Fatalf("unexpected snapshot: %+v; want %s %v", snapshot, state, ids)
	}
}

func TestLifecycle(t *testing.T) {
	manager := NewManager()
	if _, err := manager.Get("unknown", "m1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get: %v", err)
	}
	for _, kind := range []string{"CUSTOM", "MATCH_FINISHED"} {
		if err := manager.Accept(testEvent("unknown", "m1", "early", kind)); !errors.Is(err, ErrNotFound) {
			t.Fatalf("Accept before start: %v", err)
		}
	}
	requireAccept(t, manager, testEvent("unknown", "m1", "start", "MATCH_STARTED"))
	requireSnapshot(t, manager, "unknown", "m1", "open", []string{"start"})
	if err := manager.Accept(testEvent("unknown", "m1", "start2", "MATCH_STARTED")); !errors.Is(err, ErrAlreadyStarted) {
		t.Fatalf("second start: %v", err)
	}
	requireAccept(t, manager, testEvent("unknown", "m1", "e1", "CUSTOM"))
	requireAccept(t, manager, testEvent("unknown", "m1", "e2", "ANY_TYPE"))
	requireSnapshot(t, manager, "unknown", "m1", "open", []string{"start", "e1", "e2"})

	for _, differentContent := range []bool{false, true} {
		duplicate := testEvent("unknown", "m1", "e1", "CUSTOM")
		if differentContent {
			duplicate.Data = []byte(`{"changed":true}`)
		}
		if err := manager.Accept(duplicate); !errors.Is(err, ErrDuplicate) {
			t.Fatalf("duplicate: %v", err)
		}
	}
	requireSnapshot(t, manager, "unknown", "m1", "open", []string{"start", "e1", "e2"})
	requireAccept(t, manager, testEvent("unknown", "m1", "finish", "MATCH_FINISHED"))
	for _, kind := range []string{"CUSTOM", "MATCH_STARTED", "MATCH_FINISHED"} {
		if err := manager.Accept(testEvent("unknown", "m1", "late", kind)); !errors.Is(err, ErrClosed) {
			t.Fatalf("event after close: %v", err)
		}
	}
	if err := manager.Accept(testEvent("unknown", "m1", "finish", "MATCH_FINISHED")); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("duplicate after close: %v", err)
	}
	requireSnapshot(t, manager, "unknown", "m1", "closed", []string{"start", "e1", "e2", "finish"})
}

func TestKeyIsolation(t *testing.T) {
	manager := NewManager()
	// Neither concatenated keys nor identical IDs may cause cross-match conflicts.
	for _, pair := range [][2]string{{"combat", "shared"}, {"racing", "shared"}, {"a/b", "c"}, {"a", "b/c"}, {"combat", "other"}} {
		requireAccept(t, manager, testEvent(pair[0], pair[1], "start", "MATCH_STARTED"))
		requireAccept(t, manager, testEvent(pair[0], pair[1], "e1", "CUSTOM"))
		requireSnapshot(t, manager, pair[0], pair[1], "open", []string{"start", "e1"})
	}
}

func TestOwnsStoredDataAndSnapshots(t *testing.T) {
	manager := NewManager()
	first := testEvent("unknown", "m1", "start", "MATCH_STARTED")
	original := string(first.Data)
	requireAccept(t, manager, first)
	first.Data[0] = 'x'
	next := testEvent("unknown", "m1", "e1", "CUSTOM")
	requireAccept(t, manager, next)
	next.Data[0] = 'x'
	snapshot, err := manager.Get("unknown", "m1")
	if err != nil {
		t.Fatal(err)
	}
	snapshot.EventIDs[0] = "corrupted"
	requireSnapshot(t, manager, "unknown", "m1", "open", []string{"start", "e1"})

	current := manager.matches[key{"unknown", "m1"}]
	current.mu.RLock()
	defer current.mu.RUnlock()
	for _, event := range current.events {
		if string(event.Data) != original {
			t.Fatalf("stored data was mutated: %s", event.Data)
		}
	}
}

// Release workers together to exercise actual overlap without sleeps.
func concurrently(workers int, work func(int)) {
	var wg sync.WaitGroup
	ready := make(chan struct{})
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) { defer wg.Done(); <-ready; work(i) }(i)
	}
	close(ready)
	wg.Wait()
}

func TestConcurrentDifferentMatches(t *testing.T) {
	manager := NewManager()
	const workers, perMatch = 16, 50
	concurrently(workers, func(i int) {
		gameID := fmt.Sprintf("game-%d", i%2)
		matchID := fmt.Sprintf("match-%d", i)
		if err := manager.Accept(testEvent(gameID, matchID, "start", "MATCH_STARTED")); err != nil {
			t.Error(err)
			return
		}
		for n := 0; n < perMatch; n++ {
			if err := manager.Accept(testEvent(gameID, matchID, fmt.Sprintf("e-%d", n), "CUSTOM")); err != nil {
				t.Error(err)
				return
			}
			snapshot, err := manager.Get(gameID, matchID)
			if err != nil || snapshot.EventCount != n+2 {
				t.Errorf("inconsistent snapshot: %+v, %v", snapshot, err)
				return
			}
		}
		if err := manager.Accept(testEvent(gameID, matchID, "finish", "MATCH_FINISHED")); err != nil {
			t.Error(err)
		}
	})
	for i := 0; i < workers; i++ {
		ids := []string{"start"}
		for n := 0; n < perMatch; n++ {
			ids = append(ids, fmt.Sprintf("e-%d", n))
		}
		ids = append(ids, "finish")
		requireSnapshot(t, manager, fmt.Sprintf("game-%d", i%2), fmt.Sprintf("match-%d", i), "closed", ids)
	}
}

func TestDifferentMatchDoesNotWaitForLockedMatch(t *testing.T) {
	manager := NewManager()
	requireAccept(t, manager, testEvent("game", "a", "start", "MATCH_STARTED"))
	requireAccept(t, manager, testEvent("game", "b", "start", "MATCH_STARTED"))
	current := manager.matches[key{"game", "a"}]
	current.mu.Lock()
	defer current.mu.Unlock()
	done := make(chan error, 1)
	go func() { done <- manager.Accept(testEvent("game", "b", "e1", "CUSTOM")) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("match b was blocked by match a")
	}
}

func TestConcurrentStartAndDuplicates(t *testing.T) {
	manager := NewManager()
	const workers = 32
	results := make(chan error, workers)
	concurrently(workers, func(i int) {
		results <- manager.Accept(testEvent("game", "m1", fmt.Sprintf("start-%d", i), "MATCH_STARTED"))
	})
	successes := 0
	for i := 0; i < workers; i++ {
		err := <-results
		if err == nil {
			successes++
		} else if !errors.Is(err, ErrAlreadyStarted) {
			t.Fatalf("unexpected start error: %v", err)
		}
	}
	if successes != 1 {
		t.Fatalf("accepted %d starts", successes)
	}
	concurrently(workers, func(i int) { results <- manager.Accept(testEvent("game", "m1", "shared", "CUSTOM")) })
	successes = 0
	for i := 0; i < workers; i++ {
		err := <-results
		if err == nil {
			successes++
		} else if !errors.Is(err, ErrDuplicate) {
			t.Fatalf("unexpected duplicate error: %v", err)
		}
	}
	snapshot, err := manager.Get("game", "m1")
	if err != nil {
		t.Fatal(err)
	}
	if successes != 1 || snapshot.EventCount != 2 {
		t.Fatalf("successes = %d, snapshot = %+v", successes, snapshot)
	}
}

func TestConcurrentWritesReadsAndFinish(t *testing.T) {
	manager := NewManager()
	requireAccept(t, manager, testEvent("game", "m1", "start", "MATCH_STARTED"))
	const writers = 100
	results := make(chan error, writers)
	concurrently(writers+2, func(i int) {
		switch {
		case i < writers:
			results <- manager.Accept(testEvent("game", "m1", fmt.Sprintf("e-%d", i), "CUSTOM"))
		case i == writers:
			if err := manager.Accept(testEvent("game", "m1", "finish", "MATCH_FINISHED")); err != nil {
				t.Error(err)
			}
		default:
			previousCount := 0
			for n := 0; n < writers; n++ {
				snapshot, err := manager.Get("game", "m1")
				if err != nil {
					t.Error(err)
					return
				}
				if snapshot.EventCount != len(snapshot.EventIDs) || snapshot.EventCount < previousCount ||
					snapshot.EventIDs[0] != "start" || (snapshot.State == "closed" && snapshot.EventIDs[len(snapshot.EventIDs)-1] != "finish") {
					t.Errorf("inconsistent concurrent snapshot: %+v", snapshot)
					return
				}
				previousCount = snapshot.EventCount
			}
		}
	})
	accepted := 0
	for i := 0; i < writers; i++ {
		err := <-results
		if err == nil {
			accepted++
		} else if !errors.Is(err, ErrClosed) {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	snapshot, err := manager.Get("game", "m1")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.State != "closed" || snapshot.EventCount != accepted+2 || snapshot.EventIDs[len(snapshot.EventIDs)-1] != "finish" {
		t.Fatalf("unexpected final state: %+v; accepted = %d", snapshot, accepted)
	}
	seen := make(map[string]bool)
	for _, id := range snapshot.EventIDs {
		if seen[id] {
			t.Fatalf("repeated stored ID: %s", id)
		}
		seen[id] = true
	}
	// Every retained event must have a deduplication entry and intact payload.
	current := manager.matches[key{"game", "m1"}]
	current.mu.RLock()
	defer current.mu.RUnlock()
	if len(current.ids) != len(current.events) {
		t.Fatal("event index and storage differ")
	}
	for _, event := range current.events {
		if !seen[event.EventID] || string(event.Data) != string(testEvent("", "", "", "").Data) {
			t.Fatalf("corrupt event: %+v", event)
		}
	}
}

func TestConcurrentSameMatchKeepsEveryEvent(t *testing.T) {
	manager := NewManager()
	requireAccept(t, manager, testEvent("game", "m1", "start", "MATCH_STARTED"))
	const writers = 100
	concurrently(writers, func(i int) {
		if err := manager.Accept(testEvent("game", "m1", fmt.Sprintf("e-%d", i), "CUSTOM")); err != nil {
			t.Error(err)
		}
	})
	requireAccept(t, manager, testEvent("game", "m1", "finish", "MATCH_FINISHED"))
	snapshot, err := manager.Get("game", "m1")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.EventCount != writers+2 {
		t.Fatalf("lost events: %+v", snapshot)
	}
	seen := make(map[string]bool)
	for _, id := range snapshot.EventIDs {
		seen[id] = true
	}
	for i := 0; i < writers; i++ {
		if !seen[fmt.Sprintf("e-%d", i)] {
			t.Fatalf("missing event %d", i)
		}
	}
}
