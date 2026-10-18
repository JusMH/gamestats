package simulation

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
)

type Runner struct {
	Client      *Client
	Concurrency int
	Verify      bool
	Log         func(string)
	mu          sync.Mutex
}

func (r *Runner) log(format string, args ...any) {
	if r.Log != nil {
		r.mu.Lock()
		defer r.mu.Unlock()
		r.Log(fmt.Sprintf(format, args...))
	}
}

// Run uses bounded batches. Within each batch a worker owns one match. Every
// worker completes a checkpoint before any worker advances to the next phase.
// Failed matches skip future phases while peers continue; no barrier loses a member.
func (r *Runner) Run(ctx context.Context, scenarios []Scenario) (Report, error) {
	if r.Client == nil || r.Concurrency < 2 || r.Concurrency > 16 {
		return Report{}, fmt.Errorf("client and concurrency 2..16 required")
	}
	if len(scenarios) == 0 {
		return Report{}, fmt.Errorf("no scenarios")
	}
	phases := len(scenarios[0].Checkpoints)
	keys := map[string]bool{}
	for _, s := range scenarios {
		if len(s.Events) == 0 || len(s.Checkpoints) != phases || phases < 2 {
			return Report{}, fmt.Errorf("inconsistent scenario phases")
		}
		key := s.Events[0].GameID + "/" + s.Name
		if keys[key] {
			return Report{}, fmt.Errorf("duplicate scenario %s", key)
		}
		keys[key] = true
		previous := 0
		for _, cp := range s.Checkpoints {
			if cp.After <= previous || cp.After > len(s.Events) {
				return Report{}, fmt.Errorf("invalid checkpoint in %s", s.Name)
			}
			previous = cp.After
		}
		if previous != len(s.Events) {
			return Report{}, fmt.Errorf("scenario must include its final event")
		}
	}
	report := Report{Matches: []MatchReport{}}
	for first := 0; first < len(scenarios); first += r.Concurrency {
		last := first + r.Concurrency
		if last > len(scenarios) {
			last = len(scenarios)
		}
		batch := scenarios[first:last]
		reports := make([]MatchReport, len(batch))
		gates := make([]chan struct{}, phases)
		for i := range gates {
			gates[i] = make(chan struct{})
		}
		completed := make(chan struct{}, len(batch))
		var workers sync.WaitGroup
		for i, s := range batch {
			workers.Add(1)
			go func(index int, s Scenario) {
				defer workers.Done()
				result := MatchReport{GameID: s.Events[0].GameID, MatchID: s.Name, Verified: r.Verify, Results: []Result{}}
				for phase, cp := range s.Checkpoints {
					<-gates[phase]
					if result.Error == "" {
						if err := r.phase(ctx, s, cp, &result); err != nil {
							result.Error = err.Error()
							result.Verified = false
							r.log("ERROR %s/%s: %v", result.GameID, result.MatchID, err)
						}
					}
					completed <- struct{}{}
				}
				reports[index] = result
			}(i, s)
		}
		for phase := 0; phase < phases; phase++ {
			close(gates[phase])
			for range batch {
				<-completed
			}
			r.log("PHASE %d complete: %d matches reached barrier", phase+1, len(batch))
		}
		workers.Wait()
		report.Matches = append(report.Matches, reports...)
	}
	sort.Slice(report.Matches, func(i, j int) bool {
		a, b := report.Matches[i], report.Matches[j]
		if a.GameID != b.GameID {
			return a.GameID < b.GameID
		}
		return a.MatchID < b.MatchID
	})
	report.PeakRequests = int(r.Client.peak.Load())
	failed := 0
	for _, m := range report.Matches {
		if m.Error != "" {
			failed++
		}
	}
	if failed > 0 {
		return report, fmt.Errorf("%d of %d matches failed; inspect per-match errors", failed, len(report.Matches))
	}
	return report, nil
}
func (r *Runner) phase(ctx context.Context, s Scenario, cp Checkpoint, m *MatchReport) error {
	for m.Accepted < cp.After {
		e := s.Events[m.Accepted]
		if err := r.Client.Send(ctx, e); err != nil {
			return fmt.Errorf("event %s (%s): %w", e.EventID, e.Type, err)
		}
		m.Accepted++
		r.log("ACCEPTED %s/%s %s %s", e.GameID, e.MatchID, e.EventID, e.Type)
	}
	e := s.Events[0]
	snapshot, err := r.Client.Match(ctx, e)
	if err != nil {
		return err
	}
	if err := verifyMatch(snapshot, s, cp.After); err != nil {
		return err
	}
	var previous Result
	for repeat := 0; repeat < 2; repeat++ {
		result, err := r.Client.Results(ctx, e)
		if err != nil {
			return err
		}
		m.Queries++
		if r.Verify {
			if err := Verify(result, cp.Expected); err != nil {
				return err
			}
		}
		if result.GameID != e.GameID || result.MatchID != e.MatchID || result.State != snapshot.State {
			return fmt.Errorf("unexpected result identity/state")
		}
		if repeat == 1 {
			if err := jsonEqual("repeated snapshot", result, previous); err != nil {
				return err
			}
		}
		previous = result
		m.Results = append(m.Results, result)
		r.log("RESULT %s/%s %s events=%d repeat=%d %s", result.GameID, result.MatchID, result.State, cp.After, repeat+1, highlights(result))
	}
	return nil
}
func highlights(r Result) string {
	var parts []string
	for _, row := range r.Metrics {
		if row.Scope == "match" || row.ID == "points.player" || row.ID == "points_per_apple.player" || row.ID == "lap_time.avg.player" {
			parts = append(parts, fmt.Sprintf("%s@%s=%s(%s)", row.ID, row.SubjectID, row.Value, row.Status))
		}
	}
	for _, row := range r.Patterns {
		count := "null"
		if row.Count != nil {
			count = fmt.Sprint(*row.Count)
		}
		parts = append(parts, fmt.Sprintf("%s@%s=%s(%s)", row.ID, row.SubjectID, count, row.Status))
	}
	return strings.Join(parts, " ")
}
