package cartography

import (
	"sort"
	"sync"
	"time"
)

// ChangeKind classifies a signal state transition.
type ChangeKind string

const (
	// SignalTriggered fires when a previously-clear signal flips to triggered.
	// This is the alert moment — Sahm just hit, the curve just inverted.
	SignalTriggered ChangeKind = "triggered"
	// SignalCleared fires when a previously-triggered signal flips back to clear.
	// Less urgent but still worth knowing — credit stress is receding.
	SignalCleared ChangeKind = "cleared"
)

// ChangeEvent captures a single state flip the watcher observed.
type ChangeEvent struct {
	Kind        ChangeKind  `json:"kind"`
	Signal      DataSignal  `json:"signal"`        // current state
	Previous    *DataSignal `json:"previous"`      // last known state, nil on first observation
	ObservedAt  time.Time   `json:"observed_at"`   // wall-clock time of the refresh
}

// SignalWatcher tracks the prior triggered-state of every signal it has
// seen. Call Observe after each FRED refresh; the watcher returns the set
// of state changes since the last observation.
//
// First observation never emits events — there's no "previous" to diff
// against, so an already-triggered signal at boot is treated as the
// baseline rather than a fresh alert. This avoids spamming notifications
// every time the process restarts.
type SignalWatcher struct {
	mu    sync.Mutex
	prior map[string]DataSignal
	seen  bool // set after the first observation
}

// NewSignalWatcher builds an empty watcher.
func NewSignalWatcher() *SignalWatcher {
	return &SignalWatcher{prior: make(map[string]DataSignal)}
}

// Observe diffs the feed against the watcher's prior state and returns
// any state-flip events. The watcher updates its prior to match the feed
// before returning. Safe to call concurrently.
//
// Events are returned sorted by signal source for deterministic ordering
// — useful for tests and stable log lines.
func (w *SignalWatcher) Observe(feed *DataFeed) []ChangeEvent {
	if feed == nil {
		return nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()

	var events []ChangeEvent
	now := feed.LastFetched
	if now.IsZero() {
		now = time.Now()
	}

	for _, s := range feed.Signals {
		prev, hadPrev := w.prior[s.Source]
		// Only emit changes after the first full observation cycle. This
		// prevents boot-time triggered signals from being announced as if
		// they just fired.
		if w.seen && hadPrev && prev.Triggered != s.Triggered {
			kind := SignalCleared
			if s.Triggered {
				kind = SignalTriggered
			}
			prevCopy := prev
			events = append(events, ChangeEvent{
				Kind:       kind,
				Signal:     s,
				Previous:   &prevCopy,
				ObservedAt: now,
			})
		}
		w.prior[s.Source] = s
	}

	w.seen = true
	sort.SliceStable(events, func(i, j int) bool {
		return events[i].Signal.Source < events[j].Signal.Source
	})
	return events
}

// Snapshot returns a copy of the current prior map. Useful for diagnostics
// and the /api/cartography handler.
func (w *SignalWatcher) Snapshot() map[string]DataSignal {
	w.mu.Lock()
	defer w.mu.Unlock()
	out := make(map[string]DataSignal, len(w.prior))
	for k, v := range w.prior {
		out[k] = v
	}
	return out
}
