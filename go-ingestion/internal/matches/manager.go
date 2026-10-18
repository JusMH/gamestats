// Package matches manages validated events and match lifecycles in memory.
package matches

import (
	"errors"
	"sync"

	"gamestats/go-ingestion/internal/events"
)

var (
	ErrNotFound       = errors.New("match not found; MATCH_STARTED must be accepted first")
	ErrDuplicate      = errors.New("eventId already exists in this match")
	ErrAlreadyStarted = errors.New("match has already been started")
	ErrClosed         = errors.New("match is closed")
	ErrLimit          = errors.New("match limit exceeded: maximum 10000 events and 8 MiB; reserve one event and 1 MiB for MATCH_FINISHED")
)

type key struct{ gameID, matchID string }

type match struct {
	mu     sync.RWMutex
	closed bool
	events []events.Event
	ids    map[string]struct{}
	budget snapshotBudget
}

// Manager must be constructed with NewManager and must not be copied.
// Registry locks never remain held while acquiring an existing match's lock.
type Manager struct {
	mu      sync.RWMutex
	matches map[key]*match
}

func NewManager() *Manager {
	return &Manager{matches: make(map[key]*match)}
}

type Snapshot struct {
	GameID     string   `json:"gameId"`
	MatchID    string   `json:"matchId"`
	State      string   `json:"state"`
	EventCount int      `json:"eventCount"`
	EventIDs   []string `json:"eventIds"`
}

// Accept stores an already validated event. Callers must not mutate event.Data
// during the call. A private copy is retained after acceptance.
func (m *Manager) Accept(event events.Event) error {
	k := key{event.GameID, event.MatchID}
	var current *match
	if event.Type == "MATCH_STARTED" {
		// Prepare ownership before locking the shared registry.
		first := cloneEvent(event)
		budget, budgetErr := newSnapshotBudget(first)
		m.mu.Lock()
		current = m.matches[k]
		if current == nil {
			if budgetErr != nil {
				m.mu.Unlock()
				return budgetErr
			}
			m.matches[k] = &match{
				events: []events.Event{first},
				ids:    map[string]struct{}{event.EventID: {}},
				budget: budget,
			}
			m.mu.Unlock()
			return nil
		}
		m.mu.Unlock()
	} else {
		m.mu.RLock()
		current = m.matches[k]
		m.mu.RUnlock()
		if current == nil {
			return ErrNotFound
		}
	}

	current.mu.Lock()
	defer current.mu.Unlock()
	// Duplicate policy takes precedence, even after closure.
	if _, exists := current.ids[event.EventID]; exists {
		return ErrDuplicate
	}
	if current.closed {
		return ErrClosed
	}
	if event.Type == "MATCH_STARTED" {
		return ErrAlreadyStarted
	}
	owned := cloneEvent(event)
	size, err := encodedSize(owned)
	if err != nil {
		return err
	}
	if !current.budget.fits(len(current.events), size, event.Type == "MATCH_FINISHED") {
		return ErrLimit
	}

	current.events = append(current.events, owned)
	current.budget.eventBytes += size
	current.ids[event.EventID] = struct{}{}
	if event.Type == "MATCH_FINISHED" {
		current.closed = true
	}
	return nil
}

// Get returns a consistent, detached summary in the order events were accepted.
func (m *Manager) Get(gameID, matchID string) (Snapshot, error) {
	m.mu.RLock()
	current := m.matches[key{gameID, matchID}]
	m.mu.RUnlock()
	if current == nil {
		return Snapshot{}, ErrNotFound
	}

	current.mu.RLock()
	defer current.mu.RUnlock()
	state := "open"
	if current.closed {
		state = "closed"
	}
	ids := make([]string, len(current.events))
	for i, event := range current.events {
		ids[i] = event.EventID
	}
	return Snapshot{
		GameID: gameID, MatchID: matchID, State: state,
		EventCount: len(ids), EventIDs: ids,
	}, nil
}

func cloneEvent(event events.Event) events.Event {
	event.Data = append([]byte(nil), event.Data...)
	return event
}

// EventSnapshot is a detached collection of all accepted v1 events.
// It describes one instant, including for matches still receiving events.
type EventSnapshot struct {
	GameID  string         `json:"gameId"`
	MatchID string         `json:"matchId"`
	State   string         `json:"state"`
	Events  []events.Event `json:"events"`
}

// GetEvents copies state and event data under the same match read lock.
// All locks are released before returning to the caller (and any transport).
func (m *Manager) GetEvents(gameID, matchID string) (EventSnapshot, error) {
	m.mu.RLock()
	current := m.matches[key{gameID, matchID}]
	m.mu.RUnlock()
	if current == nil {
		return EventSnapshot{}, ErrNotFound
	}
	current.mu.RLock()
	defer current.mu.RUnlock()
	snapshot := EventSnapshot{GameID: gameID, MatchID: matchID, State: "open", Events: make([]events.Event, len(current.events))}
	if current.closed {
		snapshot.State = "closed"
	}
	for i, event := range current.events {
		snapshot.Events[i] = cloneEvent(event)
	}
	return snapshot, nil
}
