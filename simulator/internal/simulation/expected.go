package simulation

import (
	"encoding/json"
	"sort"
)

func resultBase(s Scenario, n int) Result {
	state := "open"
	if s.Events[n-1].Type == "MATCH_FINISHED" {
		state = "closed"
	}
	return Result{GameID: s.Events[0].GameID, MatchID: s.Name, State: state, Metrics: []Row{}, Patterns: []Row{}}
}
func source(id, instance string) Source {
	s := Source{InstanceID: instance, ProfileVersion: "1.0.0"}
	if id == "tic-tac-toe-line/v1" {
		s.AnalyzerID = id
	} else {
		s.TemplateID = id
	}
	return s
}
func status(r Result) string {
	if r.State == "open" {
		return "provisional"
	}
	return "final"
}
func metric(r *Result, id, label, scope, subject string, value any, unit, template, instance string) {
	r.Metrics = append(r.Metrics, Row{ID: id, Label: label, Scope: scope, SubjectID: subject, Value: raw(value), Unit: unit, Status: status(*r), Source: source(template, instance)})
}
func unavailable(r *Row, reason string) {
	r.Status = "unavailable"
	r.Value = raw(nil)
	r.Reason = reason
}
func duration(r *Result, seconds int) {
	metric(r, "duration.match", "Duración de partida", "match", r.MatchID, seconds, "seconds", "duration/v1", "duration")
	if r.State == "open" {
		unavailable(&r.Metrics[len(r.Metrics)-1], "awaiting_finish")
	}
}
func average(r *Result, id, label, player, value, unit, instance string) {
	var number any
	if value != "" {
		number = json.Number(value)
	}
	metric(r, id, label, "player", player, number, unit, "event-metrics/v1", instance)
	if value == "" {
		unavailable(&r.Metrics[len(r.Metrics)-1], "no_samples")
	}
}
func pattern(r *Result, id, label, scope, subject, template, instance string, occurrences []Occurrence) {
	if occurrences == nil {
		occurrences = []Occurrence{}
	}
	count := len(occurrences)
	r.Patterns = append(r.Patterns, Row{ID: id, Label: label, Scope: scope, SubjectID: subject, Status: status(*r), Count: &count, Occurrences: occurrences, Source: source(template, instance)})
}
func occurrence(s Scenario, indices []int, details any) Occurrence {
	o := Occurrence{StartTimestamp: s.Events[indices[0]].Timestamp, EndTimestamp: s.Events[indices[len(indices)-1]].Timestamp}
	for _, i := range indices {
		o.EventIDs = append(o.EventIDs, s.Events[i].EventID)
	}
	if details != nil {
		o.Details = raw(details)
	}
	return o
}
func prepare(s *Scenario) {
	for i := range s.Checkpoints {
		c := &s.Checkpoints[i]
		basis := Basis{c.After, s.Events[c.After-1].EventID}
		for j := range c.Expected.Metrics {
			c.Expected.Metrics[j].Basis = basis
		}
		for j := range c.Expected.Patterns {
			c.Expected.Patterns[j].Basis = basis
		}
		sortRows(c.Expected.Metrics)
		sortRows(c.Expected.Patterns)
	}
}
func sortRows(rows []Row) {
	sort.Slice(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if a.ID != b.ID {
			return a.ID < b.ID
		}
		if a.Scope != b.Scope {
			return a.Scope < b.Scope
		}
		return a.SubjectID < b.SubjectID
	})
}
