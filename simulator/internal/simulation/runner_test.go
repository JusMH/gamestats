package simulation

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeGo struct {
	mu           sync.Mutex
	scenarios    map[string]Scenario
	stored       map[string][]Event
	arrivals     []Event
	fail         string
	mutate       func(Result) Result
	startBarrier int
	starts       int
	release      chan struct{}
}

func newFake(t *testing.T, scenarios []Scenario) (*fakeGo, *Client) {
	t.Helper()
	fake := &fakeGo{scenarios: map[string]Scenario{}, stored: map[string][]Event{}, release: make(chan struct{})}
	for _, s := range scenarios {
		fake.scenarios[s.Name] = s
	}
	server := httptest.NewServer(fake)
	t.Cleanup(server.Close)
	c, err := NewClient(server.URL, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return fake, c
}
func (f *fakeGo) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/events" {
		var e Event
		if json.NewDecoder(r.Body).Decode(&e) != nil || e.SchemaVersion != 1 || e.EventID == "" || len(e.Data) == 0 || e.Data[0] != '{' {
			http.Error(w, "bad envelope", 400)
			return
		}
		f.mu.Lock()
		if e.MatchID == f.fail {
			f.mu.Unlock()
			http.Error(w, "injected failure", 503)
			return
		}
		previous := f.stored[e.MatchID]
		s, exists := f.scenarios[e.MatchID]
		for _, old := range previous {
			if old.EventID == e.EventID {
				f.mu.Unlock()
				http.Error(w, "duplicate", 409)
				return
			}
		}
		if len(previous) > 0 && previous[len(previous)-1].Type == "MATCH_FINISHED" {
			f.mu.Unlock()
			http.Error(w, "closed", 409)
			return
		}
		if !exists || len(previous) >= len(s.Events) || !reflect.DeepEqual(e, s.Events[len(previous)]) {
			f.mu.Unlock()
			http.Error(w, "unexpected event order or isolation", 400)
			return
		}
		f.stored[e.MatchID] = append(previous, e)
		f.arrivals = append(f.arrivals, e)
		wait := e.Type == "MATCH_STARTED" && f.startBarrier > 0
		if wait {
			f.starts++
			if f.starts == f.startBarrier {
				close(f.release)
			}
		}
		f.mu.Unlock()
		if wait {
			select {
			case <-f.release:
			case <-r.Context().Done():
				return
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "accepted", "eventId": e.EventID})
		return
	}
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) != 3 {
		http.NotFound(w, r)
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	s, exists := f.scenarios[parts[2]]
	events := f.stored[parts[2]]
	if !exists || len(events) == 0 || events[0].GameID != parts[1] {
		http.NotFound(w, r)
		return
	}
	state := "open"
	if events[len(events)-1].Type == "MATCH_FINISHED" {
		state = "closed"
	}
	if parts[0] == "matches" {
		ids := []string{}
		for _, e := range events {
			ids = append(ids, e.EventID)
		}
		_ = json.NewEncoder(w).Encode(Match{parts[1], parts[2], state, len(events), ids})
		return
	}
	if parts[0] == "results" {
		result := resultBase(s, len(events))
		for _, cp := range s.Checkpoints {
			if cp.After == len(events) {
				result = cp.Expected
				break
			}
		}
		if f.mutate != nil {
			result = f.mutate(result)
		}
		_ = json.NewEncoder(w).Encode(result)
		return
	}
	_, _ = io.WriteString(w, fmt.Sprint("unsupported route"))
}
func TestConcurrentRunnerOrderIsolationAndLiveResults(t *testing.T) {
	s := mustScenarios(t, "parallel")
	fake, c := newFake(t, s)
	fake.startBarrier = 6
	runner := Runner{Client: c, Concurrency: 6, Verify: true}
	report, err := runner.Run(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	if report.PeakRequests < 6 {
		t.Fatal("did not overlap actual HTTP requests", report.PeakRequests)
	}
	if len(fake.arrivals) < 6 {
		t.Fatal("missing arrivals")
	}
	starts := map[string]bool{}
	for _, e := range fake.arrivals[:6] {
		if e.Type != "MATCH_STARTED" {
			t.Fatal("no simultaneous starts")
		}
		starts[e.GameID] = true
	}
	if len(starts) != 3 {
		t.Fatal("games not interleaved")
	}
	for _, m := range report.Matches {
		if !m.Verified || m.Queries != 10 || len(m.Results) != 10 || m.Results[0].State != "open" || m.Results[9].State != "closed" {
			t.Fatal(m)
		}
		if m.Accepted != len(fake.stored[m.MatchID]) {
			t.Fatal("missing events")
		}
	}
}
func TestBoundedBatches(t *testing.T) {
	s := mustScenarios(t, "bounded")
	_, c := newFake(t, s)
	report, err := (&Runner{Client: c, Concurrency: 2, Verify: true}).Run(context.Background(), s)
	if err != nil || report.PeakRequests > 2 {
		t.Fatal(err, report.PeakRequests)
	}
}
func TestFailedMatchDoesNotHidePeers(t *testing.T) {
	s := mustScenarios(t, "fail")
	fake, c := newFake(t, s)
	fake.fail = s[0].Name
	report, err := (&Runner{Client: c, Concurrency: 6, Verify: true}).Run(context.Background(), s)
	if err == nil {
		t.Fatal("failure hidden")
	}
	good, bad := 0, 0
	for _, m := range report.Matches {
		if m.Error != "" {
			bad++
			if m.Verified {
				t.Fatal("failure marked verified")
			}
		} else {
			good++
			if m.Queries != 10 {
				t.Fatal("peer interrupted")
			}
		}
	}
	if good != 5 || bad != 1 {
		t.Fatal(good, bad)
	}
}
func TestRepeatedRunIDsFailClearly(t *testing.T) {
	s := mustScenarios(t, "repeat")
	_, c := newFake(t, s)
	runner := Runner{Client: c, Concurrency: 6, Verify: true}
	if _, err := runner.Run(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	report, err := runner.Run(context.Background(), s)
	if err == nil {
		t.Fatal("duplicate run accepted")
	}
	for _, m := range report.Matches {
		if m.Accepted != 0 || !strings.Contains(m.Error, "--run-id") {
			t.Fatal(m)
		}
	}
}
func TestUnexpectedAnalyticsFailEvenWithHTTP200(t *testing.T) {
	s := mustScenarios(t, "wrong")
	fake, c := newFake(t, s)
	fake.mutate = func(r Result) Result { r.MatchID = "another-match"; return r }
	report, err := (&Runner{Client: c, Concurrency: 6, Verify: true}).Run(context.Background(), s)
	if err == nil {
		t.Fatal("bad response accepted")
	}
	for _, m := range report.Matches {
		if m.Error == "" {
			t.Fatal("missing error")
		}
	}
}
func TestRunnerCancellationNoBlockedBarriers(t *testing.T) {
	s := mustScenarios(t, "cancel")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	once := sync.Once{}
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		once.Do(cancel)
		<-r.Context().Done()
	}, time.Second)
	done := make(chan error, 1)
	go func() { _, err := (&Runner{Client: c, Concurrency: 6, Verify: true}).Run(ctx, s); done <- err }()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancellation hidden")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("blocked worker or barrier")
	}
}
func TestRunnerConfiguration(t *testing.T) {
	s := mustScenarios(t, "config")
	_, c := newFake(t, s)
	for _, limit := range []int{0, 1, 17} {
		if _, err := (&Runner{Client: c, Concurrency: limit}).Run(context.Background(), s); err == nil {
			t.Fatal("bad limit")
		}
	}
	if _, err := (&Runner{Concurrency: 6}).Run(context.Background(), s); err == nil {
		t.Fatal("nil client")
	}
	bad := append([]Scenario{}, s...)
	bad[0].Checkpoints = nil
	if _, err := (&Runner{Client: c, Concurrency: 6}).Run(context.Background(), bad); err == nil {
		t.Fatal("inconsistent phases")
	}
}
func TestRejectedEnvelopeLeavesStateAndClosedRejections(t *testing.T) {
	s := start("negative", "negative-envelope", "snake", false)
	event(&s, 1, "alice", "APPLE_EATEN", map[string]int{"points": 10})
	event(&s, 2, "", "MATCH_FINISHED", map[string]any{})
	_, c := newFake(t, []Scenario{s})
	if err := checkEnvelope(context.Background(), c, "negative"); err != nil {
		t.Fatal(err)
	}
}
func TestUnknownGameNoInventedAnalytics(t *testing.T) {
	s := start("negative", "negative-unknown", "unprofiled-demo", false)
	event(&s, 1, "alice", "CUSTOM_FACT", map[string]any{"nested": map[string]any{"anything": []any{true, 42, "ok"}}})
	event(&s, 2, "", "MATCH_FINISHED", map[string]any{})
	_, c := newFake(t, []Scenario{s})
	if err := checkUnknown(context.Background(), c, "negative"); err != nil {
		t.Fatal(err)
	}
}
