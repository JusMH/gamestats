package simulation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"
)

func mustScenarios(t *testing.T, run string) []Scenario {
	t.Helper()
	s, err := Scenarios(run)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func TestScenariosDeterministic(t *testing.T) {
	expected := raw(mustScenarios(t, "stable"))
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, err := Scenarios("stable")
			if err != nil || !bytes.Equal(raw(got), expected) {
				t.Error("non deterministic generation", err)
			}
		}()
	}
	wg.Wait()
}
func TestScenarioOwnership(t *testing.T) {
	a, b := mustScenarios(t, "owned"), mustScenarios(t, "owned")
	a[0].Events[0].Data[0] = '!'
	if !json.Valid(b[0].Events[0].Data) {
		t.Fatal("shared data")
	}
	a[1].Checkpoints[3].Expected.Patterns[0].Occurrences = nil
	if reflect.DeepEqual(a, b) {
		t.Fatal("test did not change scenario")
	}
}
func TestRunIDValidation(t *testing.T) {
	for _, id := range []string{"", "with space", "../unsafe", "a/b", "?", "a" + string(bytes.Repeat([]byte{'x'}, 48))} {
		t.Run(fmt.Sprintf("%q", id), func(t *testing.T) {
			if _, err := Scenarios(id); err == nil {
				t.Fatal("invalid ID accepted")
			}
		})
	}
	for _, id := range []string{"demo", "A-1_test"} {
		if _, err := Scenarios(id); err != nil {
			t.Fatal(err)
		}
	}
}
func TestRunIDsOnlyNamespace(t *testing.T) {
	a, b := mustScenarios(t, "one"), mustScenarios(t, "two")
	for i := range a {
		for j, e := range a[i].Events {
			other := b[i].Events[j]
			if e.EventID == other.EventID || e.MatchID == other.MatchID {
				t.Fatal("IDs collided")
			}
			if e.Timestamp != other.Timestamp || e.Type != other.Type || !bytes.Equal(e.Data, other.Data) {
				t.Fatal("namespace changed facts")
			}
		}
	}
}
func TestSixValidEnvelopesAndLifecycles(t *testing.T) {
	counts := map[string]int{}
	for _, s := range mustScenarios(t, "valid") {
		t.Run(s.Name, func(t *testing.T) {
			counts[s.Events[0].GameID]++
			ids := map[string]bool{}
			if s.Events[0].Type != "MATCH_STARTED" || s.Events[len(s.Events)-1].Type != "MATCH_FINISHED" {
				t.Fatal("bad lifecycle")
			}
			last := time.Time{}
			for i, e := range s.Events {
				data := raw(e)
				var wire map[string]json.RawMessage
				if err := json.Unmarshal(data, &wire); err != nil {
					t.Fatal(err)
				}
				for _, field := range []string{"schemaVersion", "eventId", "timestamp", "gameId", "matchId", "type", "data"} {
					if len(wire[field]) == 0 {
						t.Fatal("missing", field)
					}
				}
				if e.SchemaVersion != 1 || e.EventID == "" || e.GameID == "" || e.MatchID != s.Name || e.Type == "" {
					t.Fatal("invalid envelope")
				}
				var object map[string]json.RawMessage
				if err := json.Unmarshal(e.Data, &object); err != nil || object == nil {
					t.Fatal("data not an object")
				}
				instant, err := time.Parse(time.RFC3339, e.Timestamp)
				if err != nil || instant.Before(last) {
					t.Fatal("time order", err)
				}
				last = instant
				if ids[e.EventID] {
					t.Fatal("duplicate event")
				}
				ids[e.EventID] = true
				if i > 0 && e.Type == "MATCH_STARTED" || i < len(s.Events)-1 && e.Type == "MATCH_FINISHED" {
					t.Fatal("bad lifecycle position")
				}
			}
			if len(s.Checkpoints) != 5 || s.Checkpoints[0].After != 1 || s.Checkpoints[4].After != len(s.Events) {
				t.Fatal("checkpoints")
			}
			for i, cp := range s.Checkpoints {
				want := "open"
				if i == 4 {
					want = "closed"
				}
				if cp.Expected.State != want {
					t.Fatal("checkpoint state")
				}
				for _, r := range append(append([]Row{}, cp.Expected.Metrics...), cp.Expected.Patterns...) {
					if r.Basis.EventCount != cp.After || r.Basis.LastEventID != s.Events[cp.After-1].EventID {
						t.Fatal("basis")
					}
				}
			}
		})
	}
	for _, game := range []string{"tic-tac-toe", "snake", "racing"} {
		if counts[game] != 2 {
			t.Fatal("need two matches per game")
		}
	}
}
func TestNormalBoardScenariosCoherent(t *testing.T) {
	for _, s := range mustScenarios(t, "board") {
		if s.Events[0].GameID != "tic-tac-toe" {
			continue
		}
		t.Run(s.Name, func(t *testing.T) {
			occupied := map[[2]int]bool{}
			for i, e := range s.Events[1 : len(s.Events)-1] {
				var move struct {
					Turn, Row, Column int
					Symbol            string
				}
				if err := json.Unmarshal(e.Data, &move); err != nil {
					t.Fatal(err)
				}
				player, symbol := "alice", "X"
				if i%2 == 1 {
					player, symbol = "bob", "O"
				}
				if e.PlayerID != player || move.Symbol != symbol || move.Turn != i+1 || move.Row < 0 || move.Row > 2 || move.Column < 0 || move.Column > 2 {
					t.Fatal("incoherent move")
				}
				cell := [2]int{move.Row, move.Column}
				if occupied[cell] {
					t.Fatal("occupied cell")
				}
				occupied[cell] = true
				var data map[string]any
				_ = json.Unmarshal(e.Data, &data)
				if _, ok := data["outcome"]; ok {
					t.Fatal("producer delivered analysis")
				}
			}
		})
	}
}
func TestSnakeIncrementalPointsAndDeaths(t *testing.T) {
	for _, s := range mustScenarios(t, "snake") {
		if s.Events[0].GameID != "snake" {
			continue
		}
		t.Run(s.Name, func(t *testing.T) {
			dead := map[string]bool{}
			apples := map[string]int{}
			for _, e := range s.Events {
				if e.Type == "APPLE_EATEN" {
					if dead[e.PlayerID] {
						t.Fatal("apple after death")
					}
					var data struct{ Points int }
					if err := json.Unmarshal(e.Data, &data); err != nil || data.Points < 0 {
						t.Fatal("invalid increment")
					}
					apples[e.PlayerID]++
				}
				if e.Type == "PLAYER_DIED" {
					if dead[e.PlayerID] {
						t.Fatal("repeated death")
					}
					dead[e.PlayerID] = true
				}
			}
			if !dead["alice"] || !dead["bob"] || apples["alice"] != 3 || apples["bob"] != 2 {
				t.Fatal("missing player activity")
			}
		})
	}
}
func TestRacingLapFacts(t *testing.T) {
	for _, s := range mustScenarios(t, "race") {
		if s.Events[0].GameID != "racing" {
			continue
		}
		t.Run(s.Name, func(t *testing.T) {
			laps := map[string]int{}
			previous := map[string]time.Time{"alice": epoch, "bob": epoch}
			for _, e := range s.Events {
				if e.Type == "LAP_COMPLETED" {
					var lap struct{ Lap, DurationMs int }
					if err := json.Unmarshal(e.Data, &lap); err != nil {
						t.Fatal(err)
					}
					instant, _ := time.Parse(time.RFC3339, e.Timestamp)
					laps[e.PlayerID]++
					if lap.Lap != laps[e.PlayerID] || lap.DurationMs != int(instant.Sub(previous[e.PlayerID]).Milliseconds()) {
						t.Fatal("incorrect lap facts")
					}
					previous[e.PlayerID] = instant
				}
			}
			if laps["alice"] != 3 || laps["bob"] != 2 {
				t.Fatal("laps")
			}
		})
	}
}
