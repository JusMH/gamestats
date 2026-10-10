package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const combatEvent = `{"schemaVersion":1,"eventId":"evt-001","timestamp":"2026-10-10T18:00:00Z","gameId":"arena-combat","gameVersion":"1.0","matchId":"match-001","playerId":"player-01","type":"DAMAGE_DEALT","data":{"targetId":"player-02","amount":35}}`

func changeField(t *testing.T, name string, value string) string {
	t.Helper()
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(combatEvent), &fields); err != nil {
		t.Fatal(err)
	}
	if value == "" {
		delete(fields, name)
	} else {
		fields[name] = json.RawMessage(value)
	}
	body, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func TestEvents(t *testing.T) {
	type eventCase struct {
		name       string
		body       string
		status     int
		errorField string
	}
	cases := []eventCase{
		{"combat", combatEvent, 200, ""},
		{"racing", `{"schemaVersion":1,"eventId":"evt-001","timestamp":"2026-10-10T12:00:00-06:00","gameId":"racing","matchId":"race-01","playerId":"driver-01","type":"LAP_COMPLETED","data":{"lap":2,"durationMs":75000}}`, 200, ""},
		{"match without player", `{"schemaVersion":1,"eventId":"evt-001","timestamp":"2026-10-10T18:00:00Z","gameId":"unknown","matchId":"m1","type":"MATCH_STARTED","data":{}}`, 200, ""},
		{"unknown game and data", `{"schemaVersion":1,"eventId":"evt-001","timestamp":"2026-10-10T18:00:00.123Z","gameId":"puzzle","matchId":"m1","type":"custom","data":{"board":[1,null,{"new":true}],"score":9007199254740993}}`, 200, ""},
		{"data has no domain fields", changeField(t, "data", "{}"), 200, ""},
		{"optional empty player", changeField(t, "playerId", `""`), 200, ""},
		{"optional empty version", changeField(t, "gameVersion", `""`), 200, ""},
		{"unknown envelope key", strings.Replace(combatEvent, `"schemaVersion":1`, `"extension":true,"schemaVersion":1`, 1), 200, ""},
		{"missing eventId", changeField(t, "eventId", ""), 400, "eventId"},
		{"missing gameId", changeField(t, "gameId", ""), 400, "gameId"},
		{"missing matchId", changeField(t, "matchId", ""), 400, "matchId"},
		{"missing type", changeField(t, "type", ""), 400, "type"},
		{"missing timestamp", changeField(t, "timestamp", ""), 400, "timestamp"},
		{"missing version", changeField(t, "schemaVersion", ""), 400, "schemaVersion"},
		{"missing data", changeField(t, "data", ""), 400, "data"},
		{"wrong timestamp", changeField(t, "timestamp", `"yesterday"`), 400, "timestamp"},
		{"timezone missing", changeField(t, "timestamp", `"2026-10-10T18:00:00"`), 400, "timestamp"},
		{"impossible date", changeField(t, "timestamp", `"2026-02-30T18:00:00Z"`), 400, "timestamp"},
		{"single digit hour", changeField(t, "timestamp", `"2026-10-10T8:00:00Z"`), 400, "timestamp"},
		{"comma fraction", changeField(t, "timestamp", `"2026-10-10T18:00:00,123Z"`), 400, "timestamp"},
		{"invalid offset hour", changeField(t, "timestamp", `"2026-10-10T18:00:00+24:00"`), 400, "timestamp"},
		{"invalid offset minute", changeField(t, "timestamp", `"2026-10-10T18:00:00+00:60"`), 400, "timestamp"},
		{"wrong version", changeField(t, "schemaVersion", "2"), 400, "schemaVersion"},
		{"fractional version", changeField(t, "schemaVersion", "1.5"), 400, "schemaVersion"},
		{"decimal version", changeField(t, "schemaVersion", "1.0"), 400, "schemaVersion"},
		{"string version", changeField(t, "schemaVersion", `"1"`), 400, "schemaVersion"},
		{"malformed JSON", `{"schemaVersion":`, 400, "JSON"},
		{"malformed data", strings.Replace(combatEvent, `"amount":35`, `"amount":`, 1), 400, "JSON"},
		{"empty body", "", 400, "JSON"},
		{"top level array", "[]", 400, "object"},
		{"top level null", "null", 400, "object"},
		{"top level string", `"event"`, 400, "object"},
		{"second object", combatEvent + " {}", 400, "exactly one"},
		{"second scalar", combatEvent + " true", 400, "exactly one"},
		{"trailing garbage", combatEvent + " garbage", 400, "exactly one"},
		{"trailing whitespace", combatEvent + " \n\t ", 200, ""},
		{"wrong field case", strings.Replace(combatEvent, "eventId", "EventId", 1), 400, "eventId"},
		{"duplicate key", strings.Replace(combatEvent, `"schemaVersion":1`, `"schemaVersion":2,"schemaVersion":1`, 1), 400, "duplicate"},
		{"too large", changeField(t, "data", `{"text":"`+strings.Repeat("x", maxBodyBytes)+`"}`), 400, "too large"},
	}
	for _, field := range []string{"eventId", "gameId", "matchId", "type", "timestamp", "playerId", "gameVersion", "schemaVersion", "data"} {
		cases = append(cases, eventCase{
			"null " + field, changeField(t, field, "null"), 400, field,
		})
	}
	for _, field := range []string{"eventId", "gameId", "matchId", "type", "timestamp"} {
		for _, value := range []string{`""`, `"   "`, "42", "true", "{}", "[]"} {
			cases = append(cases, eventCase{
				field + " invalid " + value, changeField(t, field, value), 400, field,
			})
		}
	}
	for _, field := range []string{"playerId", "gameVersion"} {
		cases = append(cases, eventCase{
			field + " wrong type", changeField(t, field, "42"), 400, field,
		})
	}
	for _, value := range []string{"[]", `"text"`, "42", "true"} {
		cases = append(cases, eventCase{
			"non-object data " + value, changeField(t, "data", value), 400, "data",
		})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/events", strings.NewReader(tc.body))
			response := httptest.NewRecorder()
			NewHandler().ServeHTTP(response, request)
			if response.Code != tc.status {
				t.Fatalf("status = %d, want %d; body = %s", response.Code, tc.status, response.Body.String())
			}
			if response.Header().Get("Content-Type") != "application/json" {
				t.Fatal("response must be JSON")
			}
			var result map[string]string
			if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if tc.status == 200 {
				if result["status"] != "validated" || result["eventId"] != "evt-001" || len(result) != 2 {
					t.Fatalf("unexpected success: %v", result)
				}
			} else if !strings.Contains(result["error"], tc.errorField) || len(result) != 1 {
				t.Fatalf("unexpected error: %v; want %q", result, tc.errorField)
			}
		})
	}
}

func TestHealth(t *testing.T) {
	response := httptest.NewRecorder()
	NewHandler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d", response.Code)
	}
	if response.Header().Get("Content-Type") != "application/json" {
		t.Fatal("response must be JSON")
	}
	var result map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["status"] != "ok" || len(result) != 1 {
		t.Fatalf("unexpected health: %v", result)
	}
}

func TestMethods(t *testing.T) {
	for _, endpoint := range []struct {
		path    string
		allowed string
	}{{"/health", http.MethodGet}, {"/events", http.MethodPost}} {
		for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead, http.MethodOptions} {
			if method == endpoint.allowed {
				continue
			}
			t.Run(method+endpoint.path, func(t *testing.T) {
				response := httptest.NewRecorder()
				NewHandler().ServeHTTP(response, httptest.NewRequest(method, endpoint.path, nil))
				if response.Code != http.StatusMethodNotAllowed {
					t.Fatalf("status = %d", response.Code)
				}
				if response.Header().Get("Allow") != endpoint.allowed {
					t.Fatal("incorrect Allow header")
				}
				var result map[string]string
				if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
					t.Fatal(err)
				}
				if result["error"] == "" {
					t.Fatal("missing method error")
				}
			})
		}
	}
}
