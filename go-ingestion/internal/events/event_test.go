package events

import (
	"strings"
	"testing"
)

func TestDecodePreservesData(t *testing.T) {
	const data = `{"large":9007199254740993,"nested":[null,{"flag":true}]}`
	body := `{"schemaVersion":1,"eventId":"e1","timestamp":"2026-10-10T18:00:00Z","gameId":"unknown","matchId":"m1","type":"CUSTOM","data":` + data + `}`
	event, err := Decode(strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if string(event.Data) != data {
		t.Fatalf("data was changed: %s", event.Data)
	}
	if event.EventID != "e1" || event.GameID != "unknown" || event.MatchID != "m1" || event.SchemaVersion != 1 || event.Type != "CUSTOM" || event.PlayerID != "" || event.GameVersion != "" {
		t.Fatalf("unexpected envelope: %+v", event)
	}
}
