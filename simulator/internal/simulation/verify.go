package simulation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"reflect"
	"sort"
)

func decodeNumberJSON(data []byte) (any, error) {
	var value any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	err := decoder.Decode(&value)
	return value, err
}

// Compare JSON numbers by exact rational value, never float64.
func compare(path string, a, b any) error {
	switch want := b.(type) {
	case json.Number:
		got, ok := a.(json.Number)
		x, xok := new(big.Rat).SetString(string(got))
		y, yok := new(big.Rat).SetString(string(want))
		if !ok || !xok || !yok || x.Cmp(y) != 0 {
			return fmt.Errorf("%s: got %v, want %v", path, a, b)
		}
	case map[string]any:
		got, ok := a.(map[string]any)
		if !ok || len(got) != len(want) {
			return fmt.Errorf("%s: object keys differ: got %v, want %v", path, a, b)
		}
		keys := make([]string, 0, len(want))
		for key := range want {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			value, exists := got[key]
			if !exists {
				return fmt.Errorf("%s.%s: missing", path, key)
			}
			if err := compare(path+"."+key, value, want[key]); err != nil {
				return err
			}
		}
	case []any:
		got, ok := a.([]any)
		if !ok || len(got) != len(want) {
			return fmt.Errorf("%s: array length/type differs", path)
		}
		for i := range want {
			if err := compare(fmt.Sprintf("%s[%d]", path, i), got[i], want[i]); err != nil {
				return err
			}
		}
	default:
		if !reflect.DeepEqual(a, b) {
			return fmt.Errorf("%s: got %v, want %v", path, a, b)
		}
	}
	return nil
}
func jsonEqual(path string, a, b any) error {
	aa, err := json.Marshal(a)
	if err != nil {
		return err
	}
	bb, err := json.Marshal(b)
	if err != nil {
		return err
	}
	av, err := decodeNumberJSON(aa)
	if err != nil {
		return err
	}
	bv, err := decodeNumberJSON(bb)
	if err != nil {
		return err
	}
	return compare(path, av, bv)
}
func Verify(actual, expected Result) error {
	if actual.GameID != expected.GameID || actual.MatchID != expected.MatchID || actual.State != expected.State {
		return fmt.Errorf("snapshot identity/state: got %s/%s/%s, want %s/%s/%s", actual.GameID, actual.MatchID, actual.State, expected.GameID, expected.MatchID, expected.State)
	}
	if actual.Metrics == nil || actual.Patterns == nil {
		return fmt.Errorf("metrics and patterns must be JSON arrays")
	}
	for _, group := range []struct {
		name      string
		got, want []Row
	}{{"metrics", actual.Metrics, expected.Metrics}, {"patterns", actual.Patterns, expected.Patterns}} {
		if len(group.got) != len(group.want) {
			return fmt.Errorf("%s: got %d rows, want %d", group.name, len(group.got), len(group.want))
		}
		for i, want := range group.want {
			got := group.got[i]
			path := fmt.Sprintf("%s[%s/%s/%s]", group.name, want.ID, want.Scope, want.SubjectID)
			if group.name == "metrics" && len(got.Value) == 0 {
				return fmt.Errorf("%s.value is required, including null", path)
			}
			if group.name == "patterns" && got.Occurrences == nil {
				return fmt.Errorf("%s.occurrences must be an array", path)
			}
			if err := jsonEqual(path, got, want); err != nil {
				return err
			}
		}
	}
	return nil
}
func verifyMatch(m Match, s Scenario, after int) error {
	expected := Match{GameID: s.Events[0].GameID, MatchID: s.Name, State: "open", EventCount: after, EventIDs: []string{}}
	if s.Events[after-1].Type == "MATCH_FINISHED" {
		expected.State = "closed"
	}
	for _, event := range s.Events[:after] {
		expected.EventIDs = append(expected.EventIDs, event.EventID)
	}
	if !reflect.DeepEqual(m, expected) {
		return fmt.Errorf("ingestion snapshot differs: got %+v, want %+v", m, expected)
	}
	return nil
}
