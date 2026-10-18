package simulation

import (
	"encoding/json"
	"strings"
	"testing"
)

func copyResult(t *testing.T, r Result) Result {
	t.Helper()
	var copied Result
	if err := json.Unmarshal(raw(r), &copied); err != nil {
		t.Fatal(err)
	}
	return copied
}
func TestExactJSONNumericComparison(t *testing.T) {
	for _, pair := range [][2]string{{"1", "1.000000"}, {"9007199254740993", "9007199254740993.0"}, {"0.333333", "0.333333"}} {
		if err := jsonEqual("number", json.RawMessage(pair[0]), json.RawMessage(pair[1])); err != nil {
			t.Fatal(err)
		}
	}
	if err := jsonEqual("number", json.RawMessage("9007199254740993"), json.RawMessage("9007199254740992")); err == nil {
		t.Fatal("precision lost")
	}
}
func TestVerifierRejectsWrongFields(t *testing.T) {
	s := mustScenarios(t, "verify")
	want := s[1].Checkpoints[4].Expected
	for _, change := range []struct {
		name   string
		mutate func(*Result)
	}{
		{"identity", func(r *Result) { r.GameID = "other" }},
		{"match", func(r *Result) { r.MatchID = "other" }},
		{"state", func(r *Result) { r.State = "open" }},
		{"missing_metrics", func(r *Result) { r.Metrics = nil }},
		{"null_patterns", func(r *Result) { r.Patterns = nil }},
		{"row_count", func(r *Result) { r.Metrics = r.Metrics[1:] }},
		{"id", func(r *Result) { r.Metrics[0].ID = "wrong" }},
		{"subject", func(r *Result) { r.Metrics[0].SubjectID = "wrong" }},
		{"unit", func(r *Result) { r.Metrics[0].Unit = "wrong" }},
		{"status", func(r *Result) { r.Metrics[0].Status = "provisional" }},
		{"value", func(r *Result) { r.Metrics[0].Value = raw(999) }},
		{"missing_value", func(r *Result) { r.Metrics[0].Value = nil }},
		{"source", func(r *Result) { r.Metrics[0].Source.TemplateID = "unknown/v1" }},
		{"version", func(r *Result) { r.Metrics[0].Source.ProfileVersion = "9.0.0" }},
		{"basis", func(r *Result) { r.Metrics[0].Basis.EventCount++ }},
		{"lastEvent", func(r *Result) { r.Metrics[0].Basis.LastEventID = "wrong" }},
		{"reason", func(r *Result) { r.Metrics[0].Reason = "invented" }},
		{"count", func(r *Result) { n := 9; r.Patterns[0].Count = &n }},
		{"occurrences", func(r *Result) { r.Patterns[0].Occurrences[0].EventIDs[0] = "fabricated" }},
		{"timestamps", func(r *Result) { r.Patterns[0].Occurrences[0].StartTimestamp = "2000-01-01T00:00:00Z" }},
		{"missing_occurrences", func(r *Result) { r.Patterns[0].Occurrences = nil }},
		{"row_order", func(r *Result) { r.Metrics[0], r.Metrics[1] = r.Metrics[1], r.Metrics[0] }},
	} {
		t.Run(change.name, func(t *testing.T) {
			got := copyResult(t, want)
			change.mutate(&got)
			if err := Verify(got, want); err == nil {
				t.Fatal("unexpected result accepted")
			}
		})
	}
}
func TestVerifierReportsMetricAndSubject(t *testing.T) {
	want := mustScenarios(t, "diagnostic")[2].Checkpoints[4].Expected
	got := copyResult(t, want)
	for i := range got.Metrics {
		if got.Metrics[i].ID == "laps.player" && got.Metrics[i].SubjectID == "alice" {
			got.Metrics[i].Value = raw(6)
		}
	}
	err := Verify(got, want)
	if err == nil || !strings.Contains(err.Error(), "laps.player/player/alice") || !strings.Contains(err.Error(), "value") {
		t.Fatal(err)
	}
}
func TestIndependentInvalidNumericResult(t *testing.T) {
	want := mustScenarios(t, "domain")[1].Checkpoints[4].Expected
	for i := range want.Metrics {
		row := &want.Metrics[i]
		if row.SubjectID == "alice" && (row.ID == "points.player" || row.ID == "points_per_apple.player") {
			row.Value = raw(nil)
			row.Status = "invalid"
			row.Reason = "invalid_number"
		}
	}
	got := copyResult(t, want)
	if err := Verify(got, want); err != nil {
		t.Fatal(err)
	}
	if got.Patterns[0].Count == nil || *got.Patterns[0].Count != 1 || got.Patterns[0].Status != "final" {
		t.Fatal("numeric error leaked into independent pattern")
	}
	for i := range got.Metrics {
		if got.Metrics[i].Status == "invalid" {
			got.Metrics[i].Value = raw(0)
			break
		}
	}
	if err := Verify(got, want); err == nil {
		t.Fatal("invalid numeric silently became zero")
	}
}
func TestBoardDetailsVerified(t *testing.T) {
	want := mustScenarios(t, "details")[0].Checkpoints[4].Expected
	got := copyResult(t, want)
	got.Patterns[0].Occurrences[0].Details = raw(map[string]any{"winnerId": "bob", "cells": []any{}})
	if err := Verify(got, want); err == nil {
		t.Fatal("wrong winning line accepted")
	}
}
