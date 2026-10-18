// Package simulation produces deterministic external events and checks public HTTP results.
package simulation

import "encoding/json"

type Event struct {
	SchemaVersion int             `json:"schemaVersion"`
	EventID       string          `json:"eventId"`
	Timestamp     string          `json:"timestamp"`
	GameID        string          `json:"gameId"`
	MatchID       string          `json:"matchId"`
	GameVersion   string          `json:"gameVersion,omitempty"`
	PlayerID      string          `json:"playerId,omitempty"`
	Type          string          `json:"type"`
	Data          json.RawMessage `json:"data"`
}

type Basis struct {
	EventCount  int    `json:"eventCount"`
	LastEventID string `json:"lastEventId"`
}
type Source struct {
	TemplateID     string `json:"templateId,omitempty"`
	AnalyzerID     string `json:"analyzerId,omitempty"`
	InstanceID     string `json:"instanceId"`
	ProfileVersion string `json:"profileVersion"`
}
type Row struct {
	ID          string          `json:"id"`
	Label       string          `json:"label"`
	Scope       string          `json:"scope"`
	SubjectID   string          `json:"subjectId"`
	Status      string          `json:"status"`
	Value       json.RawMessage `json:"value,omitempty"`
	Unit        string          `json:"unit,omitempty"`
	Count       *int            `json:"count,omitempty"`
	Occurrences []Occurrence    `json:"occurrences,omitempty"`
	Reason      string          `json:"reason,omitempty"`
	Source      Source          `json:"source"`
	Basis       Basis           `json:"basis"`
}
type Occurrence struct {
	EventIDs       []string        `json:"eventIds"`
	StartTimestamp string          `json:"startTimestamp"`
	EndTimestamp   string          `json:"endTimestamp"`
	Details        json.RawMessage `json:"details,omitempty"`
}
type Result struct {
	GameID   string `json:"gameId"`
	MatchID  string `json:"matchId"`
	State    string `json:"state"`
	Metrics  []Row  `json:"metrics"`
	Patterns []Row  `json:"patterns"`
}
type Match struct {
	GameID     string   `json:"gameId"`
	MatchID    string   `json:"matchId"`
	State      string   `json:"state"`
	EventCount int      `json:"eventCount"`
	EventIDs   []string `json:"eventIds"`
}
type Checkpoint struct {
	After    int
	Expected Result
}
type Scenario struct {
	Name        string
	Events      []Event
	Checkpoints []Checkpoint
}
type MatchReport struct {
	GameID   string   `json:"gameId"`
	MatchID  string   `json:"matchId"`
	Accepted int      `json:"accepted"`
	Queries  int      `json:"queries"`
	Verified bool     `json:"verified"`
	Results  []Result `json:"results"`
	Error    string   `json:"error,omitempty"`
}
type Report struct {
	Matches      []MatchReport `json:"matches"`
	PeakRequests int           `json:"peakRequests"`
}

// MarshalJSON keeps the public distinction between metric values and pattern
// counts, including null and empty occurrences in evidence and test responses.
func (r Row) MarshalJSON() ([]byte, error) {
	object := map[string]any{
		"id": r.ID, "label": r.Label, "scope": r.Scope, "subjectId": r.SubjectID,
		"status": r.Status, "source": r.Source, "basis": r.Basis,
	}
	if r.Value != nil || r.Unit != "" {
		object["value"] = r.Value
		object["unit"] = r.Unit
	} else {
		object["count"] = r.Count
		object["occurrences"] = r.Occurrences
	}
	if r.Reason != "" {
		object["reason"] = r.Reason
	}
	return json.Marshal(object)
}
