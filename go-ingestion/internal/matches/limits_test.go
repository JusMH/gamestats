package matches

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"gamestats/go-ingestion/internal/events"
)

func TestEventLimitConcurrentAndClosure(t *testing.T) {
	m := NewManager()
	requireAccept(t, m, testEvent("game", "limit", "start", "MATCH_STARTED"))
	for i := 1; i < MaxEvents-2; i++ {
		requireAccept(t, m, testEvent("game", "limit", fmt.Sprint(i), "CUSTOM"))
	}
	results := make(chan error, 32)
	concurrently(32, func(i int) { results <- m.Accept(testEvent("game", "limit", fmt.Sprintf("race-%d", i), "CUSTOM")) })
	accepted := 0
	for i := 0; i < 32; i++ {
		err := <-results
		if err == nil {
			accepted++
		} else if !errors.Is(err, ErrLimit) {
			t.Fatal(err)
		}
	}
	if accepted != 1 {
		t.Fatalf("accepted %d events into one remaining slot", accepted)
	}
	if err := m.Accept(testEvent("game", "limit", "start", "CUSTOM")); !errors.Is(err, ErrDuplicate) {
		t.Fatal(err)
	}
	s, _ := m.Get("game", "limit")
	if s.EventCount != MaxEvents-1 || s.State != "open" {
		t.Fatalf("%+v", s)
	}
	requireAccept(t, m, testEvent("game", "limit", "finish", "MATCH_FINISHED"))
	s, _ = m.Get("game", "limit")
	if s.EventCount != MaxEvents || s.State != "closed" {
		t.Fatalf("%+v", s)
	}
	// Capacity belongs to a match, not to every match or game together.
	requireAccept(t, m, testEvent("another", "limit", "start", "MATCH_STARTED"))
}

// Each string remains below Scala's 65536-character bound, even in a large
// event. ASCII padding lets us select the exact size of the wire encoding.
func sizedEvent(t *testing.T, id, kind string, target int) events.Event {
	t.Helper()
	e := testEvent("bytes<&é", "limit", id, kind)
	e.Data = []byte(`{"parts":[]}`)
	base, err := encodedSize(e)
	if err != nil {
		t.Fatal(err)
	}
	parts := []string{}
	remaining := target - base
	for remaining > 0 {
		overhead := 2
		if len(parts) > 0 {
			overhead++
		}
		n := remaining - overhead
		if n < 0 {
			t.Fatal("unrepresentable padding")
		}
		if n > 60000 {
			n = 60000
		}
		// Leave enough room for the next quoted string and comma.
		if tail := remaining - overhead - n; tail > 0 && tail < 3 {
			n -= 3 - tail
		}
		parts = append(parts, strings.Repeat("x", n))
		remaining -= overhead + n
	}
	e.Data, err = json.Marshal(map[string]any{"parts": parts})
	if err != nil {
		t.Fatal(err)
	}
	actual, _ := encodedSize(e)
	if actual != target {
		t.Fatalf("size %d, want %d", actual, target)
	}
	return e
}

func TestSnapshotByteBoundaryAndRejectedEventCanBeRetried(t *testing.T) {
	m := NewManager()
	requireAccept(t, m, testEvent("bytes<&é", "limit", "start", "MATCH_STARTED"))
	current := m.matches[key{"bytes<&é", "limit"}]
	for i := 0; i < 7; i++ {
		// Fill up to precisely 8 MiB minus the closing event and separator.
		available := MaxSnapshotBytes - MaxFinishBytes - 1 - current.budget.envelopeBytes - current.budget.eventBytes - len(current.events)
		if available > 900000 {
			available = 900000
		}
		requireAccept(t, m, sizedEvent(t, fmt.Sprint(i), "CUSTOM", available))
	}
	available := MaxSnapshotBytes - MaxFinishBytes - 1 - current.budget.envelopeBytes - current.budget.eventBytes - len(current.events)
	requireAccept(t, m, sizedEvent(t, "last", "CUSTOM", available))
	before, _ := m.GetEvents("bytes<&é", "limit")
	beforeBytes, _ := json.Marshal(before)
	if len(beforeBytes) != MaxSnapshotBytes-MaxFinishBytes-3 {
		t.Fatalf("open size %d", len(beforeBytes))
	}
	if err := m.Accept(testEvent("bytes<&é", "limit", "retry", "CUSTOM")); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
	// A closing event one byte over its reserved budget is also rejected.
	if err := m.Accept(sizedEvent(t, "retry", "MATCH_FINISHED", MaxFinishBytes+1)); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
	after, _ := m.GetEvents("bytes<&é", "limit")
	afterBytes, _ := json.Marshal(after)
	if string(afterBytes) != string(beforeBytes) || len(current.ids) != len(current.events) {
		t.Fatal("rejection changed stored data")
	}
	// Same ID: rejected events never enter the duplicate index.
	requireAccept(t, m, sizedEvent(t, "retry", "MATCH_FINISHED", MaxFinishBytes))
	closed, _ := m.GetEvents("bytes<&é", "limit")
	encoded, err := json.Marshal(closed)
	if err != nil || len(encoded) != MaxSnapshotBytes || closed.State != "closed" {
		t.Fatalf("closed bytes %d, err %v", len(encoded), err)
	}
}

func TestOversizedStartDoesNotCreateMatch(t *testing.T) {
	m := NewManager()
	e := testEvent("game", "limit", "start", "MATCH_STARTED")
	// Escaping expands this small raw input well beyond the snapshot budget.
	e.Data = []byte(`{"text":"` + strings.Repeat("<", MaxSnapshotBytes/6) + `"}`)
	if err := m.Accept(e); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
	if _, err := m.Get("game", "limit"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	e.Data = []byte(`{}`)
	requireAccept(t, m, e)
	// Identifier sizes must also permit a minimal close below the POST limit.
	e = testEvent(strings.Repeat("<", MaxFinishBytes/6), "limit", "start", "MATCH_STARTED")
	if err := m.Accept(e); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
}

func TestSnapshotAccountingIncludesJSONEscapingAndConcurrentByteBudget(t *testing.T) {
	m := NewManager()
	requireAccept(t, m, testEvent("bytes<&é", "limit", "start", "MATCH_STARTED"))
	// Raw data contains whitespace and characters expanded by encoding/json.
	e := testEvent("bytes<&é", "limit", "<é\n", "CUSTOM")
	e.Data = []byte("{ \"text\" : \"<>&é\\u2028\" }")
	requireAccept(t, m, e)
	current := m.matches[key{"bytes<&é", "limit"}]
	s, _ := m.GetEvents("bytes<&é", "limit")
	b, _ := json.Marshal(s)
	if len(b)+2 != current.budget.envelopeBytes+current.budget.eventBytes+len(s.Events)-1 {
		t.Fatal("escaped bytes not counted accurately")
	}
	// Competing large events cannot jointly cross the byte limit.
	results := make(chan error, 16)
	concurrently(16, func(i int) { results <- m.Accept(sizedEvent(t, fmt.Sprint(i), "CUSTOM", 900000)) })
	accepted := 0
	for i := 0; i < 16; i++ {
		err := <-results
		if err == nil {
			accepted++
		} else if !errors.Is(err, ErrLimit) {
			t.Fatal(err)
		}
	}
	if accepted != 8 {
		t.Fatalf("accepted %d large events", accepted)
	}
	requireAccept(t, m, testEvent("bytes<&é", "limit", "finish", "MATCH_FINISHED"))
	s, _ = m.GetEvents("bytes<&é", "limit")
	b, _ = json.Marshal(s)
	if len(b) > MaxSnapshotBytes {
		t.Fatalf("oversized snapshot: %d", len(b))
	}
}
