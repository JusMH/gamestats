package simulation

import (
	"context"
	"errors"
	"fmt"
	"net/http"
)

type NegativeCheck struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Error  string `json:"error,omitempty"`
}
type NegativeReport struct {
	Checks []NegativeCheck `json:"checks"`
}

// Negative cases use separate match IDs and never contaminate normal scenarios.
func CheckNegative(ctx context.Context, c *Client, run string, log func(string)) (NegativeReport, error) {
	if !runIDShape.MatchString(run) {
		return NegativeReport{}, fmt.Errorf("invalid run-id")
	}
	report := NegativeReport{Checks: []NegativeCheck{}}
	for _, check := range []struct {
		name string
		fn   func() error
	}{
		{"envelope_rejected_without_mutation", func() error { return checkEnvelope(ctx, c, run) }},
		{"invalid_domain_independent_pattern", func() error { return checkDomain(ctx, c, run) }},
		{"unknown_game_empty_results", func() error { return checkUnknown(ctx, c, run) }},
	} {
		err := check.fn()
		item := NegativeCheck{Name: check.name, Passed: err == nil}
		if err != nil {
			item.Error = err.Error()
		}
		report.Checks = append(report.Checks, item)
		if log != nil {
			log(fmt.Sprintf("NEGATIVE %s passed=%v %s", item.Name, item.Passed, item.Error))
		}
	}
	failed := 0
	for _, item := range report.Checks {
		if !item.Passed {
			failed++
		}
	}
	if failed > 0 {
		return report, fmt.Errorf("%d negative checks failed", failed)
	}
	return report, nil
}
func expectHTTP(err error, status int) error {
	var failure *HTTPError
	if !errors.As(err, &failure) || failure.Status != status {
		return fmt.Errorf("want HTTP %d, got %v", status, err)
	}
	return nil
}
func checkEnvelope(ctx context.Context, c *Client, run string) error {
	s := start(run, "negative-envelope", "snake", false)
	// First reject an invalid start; it must not create a match.
	invalid := s.Events[0]
	invalid.SchemaVersion = 2
	if err := expectHTTP(c.Exchange(ctx, http.MethodPost, "/events", raw(invalid), nil), 400); err != nil {
		return err
	}
	if _, err := c.Match(ctx, s.Events[0]); expectHTTP(err, 404) != nil {
		return fmt.Errorf("rejected start created state: %v", err)
	}
	if err := c.Send(ctx, s.Events[0]); err != nil {
		return err
	}
	// A malformed subsequent envelope must leave the already-created match unchanged.
	event(&s, 1, "alice", "APPLE_EATEN", map[string]int{"points": 10})
	invalid = s.Events[1]
	invalid.EventID = ""
	if err := expectHTTP(c.Exchange(ctx, http.MethodPost, "/events", raw(invalid), nil), 400); err != nil {
		return err
	}
	m, err := c.Match(ctx, s.Events[0])
	if err != nil {
		return err
	}
	if err := verifyMatch(m, s, 1); err != nil {
		return err
	}
	if err := c.Send(ctx, s.Events[1]); err != nil {
		return err
	}
	event(&s, 2, "", "MATCH_FINISHED", map[string]any{})
	if err := c.Send(ctx, s.Events[2]); err != nil {
		return err
	}
	if err := expectHTTP(c.Send(ctx, s.Events[1]), 409); err != nil {
		return err
	}
	event(&s, 3, "alice", "APPLE_EATEN", map[string]int{"points": 1})
	if err := expectHTTP(c.Send(ctx, s.Events[3]), 409); err != nil {
		return err
	}
	m, err = c.Match(ctx, s.Events[0])
	if err != nil {
		return err
	}
	return verifyMatch(m, s, 3)
}
func checkDomain(ctx context.Context, c *Client, run string) error {
	s := start(run, "negative-domain", "snake", false)
	event(&s, 1, "alice", "APPLE_EATEN", map[string]any{"points": "not-a-number"})
	event(&s, 4, "alice", "APPLE_EATEN", map[string]int{"points": 3})
	event(&s, 7, "alice", "APPLE_EATEN", map[string]int{"points": 9})
	event(&s, 9, "alice", "PLAYER_DIED", map[string]string{"cause": "other"})
	event(&s, 10, "", "MATCH_FINISHED", map[string]any{})
	// Count and temporal evidence are valid; only numeric outputs for alice fail.
	for _, n := range []int{5, 6} {
		r := resultBase(s, n)
		duration(&r, 10)
		for _, player := range []string{"alice", "bob"} {
			count, sum, avg, deaths, life := 0, 0, "", 0, 0
			if player == "alice" {
				count = 3
				deaths = 1
				life = 9
			}
			metric(&r, "apples.player", "Manzanas por jugador", "player", player, count, "events", "event-metrics/v1", "apples")
			metric(&r, "points.player", "Puntos por jugador", "player", player, sum, "points", "event-metrics/v1", "apples")
			if player == "alice" {
				row := &r.Metrics[len(r.Metrics)-1]
				row.Value = raw(nil)
				row.Status = "invalid"
				row.Reason = "invalid_number"
			}
			average(&r, "points_per_apple.player", "Puntos por manzana", player, avg, "points/apple", "apples")
			if player == "alice" {
				row := &r.Metrics[len(r.Metrics)-1]
				row.Status = "invalid"
				row.Reason = "invalid_number"
			}
			metric(&r, "deaths.player", "Muertes por jugador", "player", player, deaths, "events", "event-metrics/v1", "apples")
			metric(&r, "lifetime.player", "Tiempo hasta la muerte", "player", player, life, "seconds", "duration/v1", "lifetime")
			if player == "bob" {
				reason := "awaiting_end"
				if r.State == "closed" {
					reason = "missing_end"
				}
				unavailable(&r.Metrics[len(r.Metrics)-1], reason)
			}
			var occurrences []Occurrence
			if player == "alice" {
				occurrences = []Occurrence{occurrence(s, []int{1, 2, 3}, nil)}
			}
			pattern(&r, "apples.burst", "Tres manzanas en diez segundos", "player", player, "temporal-window/v1", "burst", occurrences)
		}
		metric(&r, "apples.match", "Manzanas de la partida", "match", s.Name, 3, "events", "event-metrics/v1", "apples")
		s.Checkpoints = append(s.Checkpoints, Checkpoint{n, r})
	}
	prepare(&s)
	sent := 0
	for _, cp := range s.Checkpoints {
		for sent < cp.After {
			if err := c.Send(ctx, s.Events[sent]); err != nil {
				return err
			}
			sent++
		}
		for repeat := 0; repeat < 2; repeat++ {
			got, err := c.Results(ctx, s.Events[0])
			if err != nil {
				return err
			}
			if err := Verify(got, cp.Expected); err != nil {
				return err
			}
		}
	}
	return nil
}
func checkUnknown(ctx context.Context, c *Client, run string) error {
	s := start(run, "negative-unknown", "unprofiled-demo", false)
	event(&s, 1, "alice", "CUSTOM_FACT", map[string]any{"nested": map[string]any{"anything": []any{true, 42, "ok"}}})
	event(&s, 2, "", "MATCH_FINISHED", map[string]any{})
	for i, e := range s.Events {
		if err := c.Send(ctx, e); err != nil {
			return err
		}
		if i > 0 {
			got, err := c.Results(ctx, e)
			if err != nil {
				return err
			}
			if err := Verify(got, resultBase(s, i+1)); err != nil {
				return err
			}
		}
	}
	return nil
}
