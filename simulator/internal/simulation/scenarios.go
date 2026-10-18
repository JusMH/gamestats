package simulation

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"
)

var runIDShape = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,47}$`)
var epoch = time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)

func raw(v any) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}
func event(s *Scenario, second int, player, kind string, data any) {
	id := fmt.Sprintf("%s-e%02d", s.Name, len(s.Events)+1)
	s.Events = append(s.Events, Event{SchemaVersion: 1, EventID: id, Timestamp: epoch.Add(time.Duration(second) * time.Second).Format(time.RFC3339), GameID: s.Events[0].GameID, MatchID: s.Name, GameVersion: "sim-1", PlayerID: player, Type: kind, Data: raw(data)})
}

func start(run, name, game string, symbols bool) Scenario {
	s := Scenario{Name: run + "-" + name}
	data := map[string]any{"players": []string{"alice", "bob"}}
	if symbols {
		data["symbols"] = map[string]string{"alice": "X", "bob": "O"}
	}
	s.Events = []Event{{SchemaVersion: 1, EventID: s.Name + "-e01", Timestamp: epoch.Format(time.RFC3339), GameID: game, MatchID: s.Name, GameVersion: "sim-1", Type: "MATCH_STARTED", Data: raw(data)}}
	return s
}

// Scenarios are owned, newly allocated data. No clock, randomness, network or
// fixture loading participates in generation. Expected totals are independent
// literal tables, not analytics calculated from the events or received results.
func Scenarios(run string) ([]Scenario, error) {
	if !runIDShape.MatchString(run) {
		return nil, fmt.Errorf("run-id must match %s", runIDShape)
	}
	scenarios := []Scenario{board(run, false), snake(run, true), race(run, false), board(run, true), snake(run, false), race(run, true)}
	for i := range scenarios {
		prepare(&scenarios[i])
	}
	return scenarios, nil
}
func board(run string, draw bool) Scenario {
	name := "ttt-win"
	if draw {
		name = "ttt-draw"
	}
	s := start(run, name, "tic-tac-toe", true)
	positions := [][2]int{{2, 0}, {0, 0}, {2, 1}, {0, 1}, {2, 2}}
	if draw {
		positions = [][2]int{{0, 0}, {1, 1}, {0, 2}, {0, 1}, {2, 1}, {1, 0}, {1, 2}, {2, 2}, {2, 0}}
	}
	step := 2
	end := 12
	if draw {
		step = 3
		end = 30
	}
	for i, cell := range positions {
		player, symbol := "alice", "X"
		if i%2 == 1 {
			player, symbol = "bob", "O"
		}
		event(&s, (i+1)*step, player, "MOVE_PLACED", map[string]any{"turn": i + 1, "row": cell[0], "column": cell[1], "symbol": symbol})
	}
	event(&s, end, "", "MATCH_FINISHED", map[string]any{})
	counts := [][2]int{{0, 0}, {1, 1}, {2, 2}, {3, 2}, {3, 2}}
	after := []int{1, 3, 5, 6, 7}
	if draw {
		counts = [][2]int{{0, 0}, {1, 1}, {2, 2}, {5, 4}, {5, 4}}
		after = []int{1, 3, 5, 10, 11}
	}
	for i, n := range after {
		r := resultBase(s, n)
		metric(&r, "moves.player", "Movimientos por jugador", "player", "alice", counts[i][0], "events", "event-metrics/v1", "moves")
		metric(&r, "moves.player", "Movimientos por jugador", "player", "bob", counts[i][1], "events", "event-metrics/v1", "moves")
		metric(&r, "moves.match", "Movimientos de la partida", "match", s.Name, counts[i][0]+counts[i][1], "events", "event-metrics/v1", "moves")
		duration(&r, end)
		outcome := "in_progress"
		var winner any
		if i >= 3 {
			if draw {
				outcome = "draw"
			} else {
				outcome = "win"
				winner = "alice"
			}
		}
		metric(&r, "board.result", "Resultado derivado del tablero", "match", s.Name, map[string]any{"outcome": outcome, "winnerId": winner}, "result", "tic-tac-toe-line/v1", "board")
		var occurrences []Occurrence
		if i >= 3 && !draw {
			occurrences = []Occurrence{occurrence(s, []int{1, 3, 5}, map[string]any{"winnerId": "alice", "cells": []map[string]int{{"row": 2, "column": 0}, {"row": 2, "column": 1}, {"row": 2, "column": 2}}})}
		}
		pattern(&r, "board.winning_line", "Primera línea ganadora", "match", s.Name, "tic-tac-toe-line/v1", "board", occurrences)
		s.Checkpoints = append(s.Checkpoints, Checkpoint{n, r})
	}
	return s
}

type playerStats struct {
	count, sum int
	avg        string
}

func snake(run string, positive bool) Scenario {
	name := "snake-positive"
	if !positive {
		name = "snake-negative"
	}
	s := start(run, name, "snake", false)
	times := []int{1, 2, 5, 9, 12}
	players := []string{"alice", "bob", "alice", "alice", "bob"}
	points := []int{4, 6, 8, 12, 14}
	end, d1, d2 := 20, 15, 18
	values := [][2]playerStats{
		{{0, 0, ""}, {0, 0, ""}}, {{2, 12, "6"}, {1, 6, "6"}}, {{3, 24, "8"}, {2, 20, "10"}}, {{3, 24, "8"}, {2, 20, "10"}}, {{3, 24, "8"}, {2, 20, "10"}},
	}
	if !positive {
		times = []int{1, 3, 12, 16, 23}
		players = []string{"alice", "bob", "alice", "bob", "alice"}
		points = []int{5, 7, 15, 13, 25}
		end, d1, d2 = 30, 26, 28
		values = [][2]playerStats{
			{{0, 0, ""}, {0, 0, ""}}, {{2, 20, "10"}, {1, 7, "7"}}, {{3, 45, "15"}, {2, 20, "10"}}, {{3, 45, "15"}, {2, 20, "10"}}, {{3, 45, "15"}, {2, 20, "10"}},
		}
	}
	for i, t := range times {
		event(&s, t, players[i], "APPLE_EATEN", map[string]int{"points": points[i]})
	}
	event(&s, d1, "alice", "PLAYER_DIED", map[string]string{"cause": "wall"})
	event(&s, d2, "bob", "PLAYER_DIED", map[string]string{"cause": "self_collision"})
	event(&s, end, "", "MATCH_FINISHED", map[string]any{})
	for phase, n := range []int{1, 4, 6, 8, 9} {
		r := resultBase(s, n)
		duration(&r, end)
		for p, player := range []string{"alice", "bob"} {
			v := values[phase][p]
			metric(&r, "apples.player", "Manzanas por jugador", "player", player, v.count, "events", "event-metrics/v1", "apples")
			metric(&r, "points.player", "Puntos por jugador", "player", player, v.sum, "points", "event-metrics/v1", "apples")
			average(&r, "points_per_apple.player", "Puntos por manzana", player, v.avg, "points/apple", "apples")
			deaths := 0
			if phase >= 3 {
				deaths = 1
			}
			metric(&r, "deaths.player", "Muertes por jugador", "player", player, deaths, "events", "event-metrics/v1", "apples")
			life := d1
			if p == 1 {
				life = d2
			}
			metric(&r, "lifetime.player", "Tiempo hasta la muerte", "player", player, life, "seconds", "duration/v1", "lifetime")
			if phase < 3 {
				unavailable(&r.Metrics[len(r.Metrics)-1], "awaiting_end")
			}
			var occurrences []Occurrence
			if positive && phase >= 2 && p == 0 {
				occurrences = []Occurrence{occurrence(s, []int{1, 3, 4}, nil)}
			}
			pattern(&r, "apples.burst", "Tres manzanas en diez segundos", "player", player, "temporal-window/v1", "burst", occurrences)
		}
		metric(&r, "apples.match", "Manzanas de la partida", "match", s.Name, values[phase][0].count+values[phase][1].count, "events", "event-metrics/v1", "apples")
		s.Checkpoints = append(s.Checkpoints, Checkpoint{n, r})
	}
	return s
}
func race(run string, second bool) Scenario {
	name := "race-sprint"
	if second {
		name = "race-endurance"
	}
	s := start(run, name, "racing", false)
	times := []int{8, 9, 18, 20, 30}
	durations := []int{8000, 9000, 10000, 11000, 12000}
	end := 32
	values := [][2]playerStats{
		{{0, 0, ""}, {0, 0, ""}}, {{2, 18000, "9000"}, {1, 9000, "9000"}}, {{2, 18000, "9000"}, {2, 20000, "10000"}}, {{3, 30000, "10000"}, {2, 20000, "10000"}}, {{3, 30000, "10000"}, {2, 20000, "10000"}},
	}
	if second {
		times = []int{11, 12, 24, 26, 33}
		durations = []int{11000, 12000, 13000, 14000, 9000}
		end = 35
		values = [][2]playerStats{
			{{0, 0, ""}, {0, 0, ""}}, {{2, 24000, "12000"}, {1, 12000, "12000"}}, {{2, 24000, "12000"}, {2, 26000, "13000"}}, {{3, 33000, "11000"}, {2, 26000, "13000"}}, {{3, 33000, "11000"}, {2, 26000, "13000"}},
		}
	}
	for i, t := range times {
		player := "alice"
		if i%2 == 1 {
			player = "bob"
		}
		event(&s, t, player, "LAP_COMPLETED", map[string]int{"lap": i/2 + 1, "durationMs": durations[i]})
	}
	event(&s, end, "", "MATCH_FINISHED", map[string]any{})
	for phase, n := range []int{1, 4, 5, 6, 7} {
		r := resultBase(s, n)
		duration(&r, end)
		for p, player := range []string{"alice", "bob"} {
			v := values[phase][p]
			metric(&r, "laps.player", "Vueltas por jugador", "player", player, v.count, "events", "event-metrics/v1", "laps")
			metric(&r, "lap_time.total.player", "Tiempo acumulado de vueltas", "player", player, v.sum, "ms", "event-metrics/v1", "laps")
			average(&r, "lap_time.avg.player", "Tiempo medio por vuelta", player, v.avg, "ms", "laps")
		}
		metric(&r, "laps.match", "Vueltas de la partida", "match", s.Name, values[phase][0].count+values[phase][1].count, "events", "event-metrics/v1", "laps")
		s.Checkpoints = append(s.Checkpoints, Checkpoint{n, r})
	}
	return s
}
