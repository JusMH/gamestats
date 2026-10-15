package matches

import (
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func TestEventSnapshotLifecycleAndOwnership(t *testing.T) {
	m := NewManager()
	if _, err := m.GetEvents("g", "m"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	want := []string{"MATCH_STARTED", "CUSTOM", "MATCH_FINISHED"}
	for i, kind := range want {
		event := testEvent("g", "m", kind, kind)
		event.PlayerID, event.GameVersion = "p1", "1.0"
		requireAccept(t, m, event)
		snapshot, err := m.GetEvents("g", "m")
		if err != nil {
			t.Fatal(err)
		}
		state := "open"
		if i == 2 {
			state = "closed"
		}
		if snapshot.GameID != "g" || snapshot.MatchID != "m" || snapshot.State != state || len(snapshot.Events) != i+1 {
			t.Fatalf("snapshot: %+v", snapshot)
		}
		for n, stored := range snapshot.Events {
			expected := testEvent("g", "m", want[n], want[n])
			expected.PlayerID, expected.GameVersion = "p1", "1.0"
			if !reflect.DeepEqual(stored, expected) {
				t.Fatalf("event changed: %+v", stored)
			}
		}
		snapshot.Events[0].Data[0] = 'x'
		snapshot.Events[0].EventID = "changed"
		snapshot.Events = append(snapshot.Events, event)
		again, _ := m.GetEvents("g", "m")
		if again.Events[0].EventID != want[0] || again.Events[0].Data[0] != '{' || len(again.Events) != i+1 {
			t.Fatal("mutable alias")
		}
	}
}

func TestEventSnapshotsWhileAccepting(t *testing.T) {
	m := NewManager()
	requireAccept(t, m, testEvent("g", "m", "start", "MATCH_STARTED"))
	concurrently(8, func(worker int) {
		for n := 0; n < 50; n++ {
			if worker < 4 {
				if err := m.Accept(testEvent("g", "m", fmt.Sprintf("%d-%d", worker, n), "CUSTOM")); err != nil {
					t.Error(err)
				}
			} else {
				s, err := m.GetEvents("g", "m")
				if err != nil {
					t.Error(err)
					return
				}
				if s.State != "open" || s.Events[0].EventID != "start" {
					t.Error("inconsistent snapshot")
				}
				seen := make(map[string]bool)
				for _, e := range s.Events {
					if seen[e.EventID] || string(e.Data) != string(testEvent("", "", "", "").Data) {
						t.Error("corrupt snapshot")
					}
					seen[e.EventID] = true
				}
				s.Events[0].Data[0] = 'x'
			}
		}
	})
	s, _ := m.GetEvents("g", "m")
	if len(s.Events) != 201 {
		t.Fatal("lost events")
	}
}
