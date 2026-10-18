package matches

import (
	"encoding/json"
	"fmt"
	"strings"

	"gamestats/go-ingestion/internal/events"
)

// Keep these transport limits aligned with Scala Decoding and HttpService.
const (
	MaxEvents        = 10000
	MaxSnapshotBytes = 8 << 20
	// The reservation includes the encoded closing event, not its HTTP body.
	MaxFinishBytes = 1 << 20
)

// Byte counts use precisely the encoding used by the HTTP analysis client,
// including escaped strings, compacted data, envelope and array separators.
// Only the new event is encoded; existing events are not re-encoded per write.
type snapshotBudget struct{ envelopeBytes, eventBytes int }

func encodedSize(value any) (int, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return 0, fmt.Errorf("encode validated event: %w", err)
	}
	return len(encoded), nil
}

func newSnapshotBudget(first events.Event) (snapshotBudget, error) {
	empty := EventSnapshot{GameID: first.GameID, MatchID: first.MatchID, State: "closed", Events: []events.Event{}}
	base, err := encodedSize(empty)
	if err != nil {
		return snapshotBudget{}, err
	}
	budget := snapshotBudget{envelopeBytes: base}
	size, err := encodedSize(first)
	if err != nil {
		return snapshotBudget{}, err
	}
	// Long identifiers must still allow a minimal closure within both the POST
	// limit and reserved bytes: a unique <=64 ASCII ID, UTC time and data:{}.
	minimal := events.Event{SchemaVersion: 1, EventID: strings.Repeat("x", 64),
		Timestamp: "2000-01-01T00:00:00Z", GameID: first.GameID,
		MatchID: first.MatchID, Type: "MATCH_FINISHED", Data: json.RawMessage(`{}`)}
	closeSize, err := encodedSize(minimal)
	if err != nil {
		return snapshotBudget{}, err
	}
	if closeSize > MaxFinishBytes || !budget.fits(0, size, false) {
		return snapshotBudget{}, ErrLimit
	}
	budget.eventBytes = size
	return budget, nil
}

// Called under the match lock before changing storage or its ID index.
func (b snapshotBudget) fits(count, size int, finishing bool) bool {
	nextCount := count + 1
	bytes := b.envelopeBytes + b.eventBytes + size + nextCount - 1
	if finishing {
		return nextCount <= MaxEvents && size <= MaxFinishBytes && bytes <= MaxSnapshotBytes
	}
	// closed is longer than open; its envelope also covers open snapshots.
	// Reserve the closing comma as well as the closing event.
	return nextCount < MaxEvents && bytes+1+MaxFinishBytes <= MaxSnapshotBytes
}
