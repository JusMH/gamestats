// Package events validates the common v1 envelope without interpreting game data.
package events

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
)

type Event struct {
	SchemaVersion int             `json:"schemaVersion"`
	EventID       string          `json:"eventId"`
	Timestamp     string          `json:"timestamp"`
	GameID        string          `json:"gameId"`
	GameVersion   string          `json:"gameVersion,omitempty"`
	MatchID       string          `json:"matchId"`
	PlayerID      string          `json:"playerId,omitempty"`
	Type          string          `json:"type"`
	Data          json.RawMessage `json:"data"`
}

// time.Parse accepts some non-RFC3339 spellings. Check the shape and offset
// ranges first, then let time.Parse validate the calendar date.
var timestampShape = regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T([01][0-9]|2[0-3]):[0-5][0-9]:[0-5][0-9](\.[0-9]+)?(Z|[+-]([01][0-9]|2[0-3]):[0-5][0-9])$`)

// Decode requires exactly one JSON object. Envelope keys are case-sensitive.
// Unknown keys are allowed; repeated envelope keys are ambiguous and rejected.
func Decode(r io.Reader) (Event, error) {
	var event Event
	decoder := json.NewDecoder(r)
	start, err := decoder.Token()
	if err != nil {
		return event, fmt.Errorf("invalid JSON: %w", err)
	}
	if start != json.Delim('{') {
		return event, fmt.Errorf("event must be a JSON object")
	}
	fields := make(map[string]json.RawMessage)
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return event, fmt.Errorf("invalid JSON: %w", err)
		}
		name, ok := key.(string)
		if !ok {
			return event, fmt.Errorf("invalid JSON object key")
		}
		if _, exists := fields[name]; exists {
			return event, fmt.Errorf("duplicate envelope field %q", name)
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return event, fmt.Errorf("invalid JSON: %w", err)
		}
		fields[name] = value
	}
	if _, err := decoder.Token(); err != nil {
		return event, fmt.Errorf("invalid JSON: %w", err)
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); err != io.EOF {
		return event, fmt.Errorf("body must contain exactly one JSON object")
	}

	version, exists := fields["schemaVersion"]
	if !exists || bytes.Equal(bytes.TrimSpace(version), []byte("null")) || json.Unmarshal(version, &event.SchemaVersion) != nil || event.SchemaVersion != 1 {
		return event, fmt.Errorf("schemaVersion is required and must be the integer 1")
	}
	stringsToRead := []struct {
		name     string
		target   *string
		required bool
	}{
		{"eventId", &event.EventID, true},
		{"timestamp", &event.Timestamp, true},
		{"gameId", &event.GameID, true},
		{"matchId", &event.MatchID, true},
		{"type", &event.Type, true},
		{"gameVersion", &event.GameVersion, false},
		{"playerId", &event.PlayerID, false},
	}
	for _, field := range stringsToRead {
		value, exists := fields[field.name]
		if !exists {
			if field.required {
				return event, fmt.Errorf("%s is required", field.name)
			}
			continue
		}
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) || json.Unmarshal(value, field.target) != nil {
			return event, fmt.Errorf("%s must be a string", field.name)
		}
		if field.required && strings.TrimSpace(*field.target) == "" {
			return event, fmt.Errorf("%s must be a non-empty string", field.name)
		}
	}
	if !timestampShape.MatchString(event.Timestamp) {
		return event, fmt.Errorf("timestamp must be a valid RFC3339 date with a timezone")
	}
	if _, err := time.Parse(time.RFC3339, event.Timestamp); err != nil {
		return event, fmt.Errorf("timestamp must be a valid RFC3339 date with a timezone")
	}
	event.Data = fields["data"]
	data := bytes.TrimSpace(event.Data)
	// The decoder already checked JSON syntax, so '{' identifies an object.
	if len(data) == 0 || data[0] != '{' {
		return event, fmt.Errorf("data is required and must be a JSON object")
	}
	return event, nil
}
