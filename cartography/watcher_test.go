package cartography

import (
	"testing"
	"time"
)

func mkFeed(signals ...DataSignal) *DataFeed {
	return &DataFeed{Signals: signals, LastFetched: time.Now()}
}

func sig(source, name string, triggered bool) DataSignal {
	return DataSignal{
		Source:    source,
		Name:      name,
		Triggered: triggered,
	}
}

// TestFirstObservationEmitsNothing — at process boot, even already-firing
// signals must not produce events. Otherwise every restart would spam.
func TestFirstObservationEmitsNothing(t *testing.T) {
	w := NewSignalWatcher()
	events := w.Observe(mkFeed(
		sig("FRED:UNRATE", "Sahm", true),
		sig("FRED:T10Y2Y", "Curve", true),
	))
	if len(events) != 0 {
		t.Errorf("expected no events on first observation, got %d", len(events))
	}
}

// TestNoChangeNoEvents — same state two refreshes in a row produces nothing.
func TestNoChangeNoEvents(t *testing.T) {
	w := NewSignalWatcher()
	w.Observe(mkFeed(sig("FRED:UNRATE", "Sahm", false)))
	events := w.Observe(mkFeed(sig("FRED:UNRATE", "Sahm", false)))
	if len(events) != 0 {
		t.Errorf("expected no events on unchanged state, got %d", len(events))
	}
}

// TestClearToTriggeredEmits — the canonical alert path.
func TestClearToTriggeredEmits(t *testing.T) {
	w := NewSignalWatcher()
	w.Observe(mkFeed(sig("FRED:UNRATE", "Sahm", false)))
	events := w.Observe(mkFeed(sig("FRED:UNRATE", "Sahm", true)))
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Kind != SignalTriggered {
		t.Errorf("expected triggered, got %s", events[0].Kind)
	}
	if events[0].Previous == nil || events[0].Previous.Triggered {
		t.Errorf("previous state wrong: %+v", events[0].Previous)
	}
}

// TestTriggeredToClearEmits — the all-clear path.
func TestTriggeredToClearEmits(t *testing.T) {
	w := NewSignalWatcher()
	w.Observe(mkFeed(sig("FRED:T10Y2Y", "Curve", true)))
	events := w.Observe(mkFeed(sig("FRED:T10Y2Y", "Curve", false)))
	if len(events) != 1 || events[0].Kind != SignalCleared {
		t.Errorf("expected one cleared event, got %+v", events)
	}
}

// TestMultiSignalIndependence — flipping one signal must not emit events
// for the others, and the output is sorted by source.
func TestMultiSignalIndependence(t *testing.T) {
	w := NewSignalWatcher()
	w.Observe(mkFeed(
		sig("FRED:UNRATE", "Sahm", false),
		sig("FRED:T10Y2Y", "Curve", false),
		sig("FRED:NFCI", "NFCI", true),
	))
	events := w.Observe(mkFeed(
		sig("FRED:UNRATE", "Sahm", true),     // flip
		sig("FRED:T10Y2Y", "Curve", false),   // unchanged
		sig("FRED:NFCI", "NFCI", true),       // unchanged
	))
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Signal.Source != "FRED:UNRATE" {
		t.Errorf("wrong signal flipped: %s", events[0].Signal.Source)
	}
}

// TestSimultaneousFlipsSorted — two flips in one observation come back
// sorted by source for deterministic logging/notifications.
func TestSimultaneousFlipsSorted(t *testing.T) {
	w := NewSignalWatcher()
	w.Observe(mkFeed(
		sig("FRED:T10Y2Y", "Curve", false),
		sig("FRED:NFCI", "NFCI", false),
	))
	events := w.Observe(mkFeed(
		sig("FRED:T10Y2Y", "Curve", true),
		sig("FRED:NFCI", "NFCI", true),
	))
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Signal.Source != "FRED:NFCI" || events[1].Signal.Source != "FRED:T10Y2Y" {
		t.Errorf("events not sorted by source: %s, %s",
			events[0].Signal.Source, events[1].Signal.Source)
	}
}

// TestNilFeed — must not panic on nil.
func TestNilFeed(t *testing.T) {
	w := NewSignalWatcher()
	if events := w.Observe(nil); events != nil {
		t.Errorf("expected nil events, got %d", len(events))
	}
}
