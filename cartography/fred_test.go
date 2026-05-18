package cartography

import (
	"testing"
)

// fixture builds a desc-sorted observation slice from a list of (date, value).
func fixture(rows []struct {
	date, value string
}) []fredObservation {
	out := make([]fredObservation, len(rows))
	for i, r := range rows {
		out[i] = fredObservation{Date: r.date, Value: r.value}
	}
	return out
}

// TestSahmTriggers cross-checks Sahm against a known historical episode:
// April 2020, when COVID drove unemployment from ~3.5% to ~14.7%. Any
// reasonable implementation should fire.
func TestSahmTriggers(t *testing.T) {
	// Desc-sorted: index 0 = most recent. Last 3 entries = current 3MMA.
	// Use historical UNRATE values around April 2020.
	rows := []struct{ date, value string }{
		{"2020-04-01", "14.7"}, // current
		{"2020-03-01", "4.4"},
		{"2020-02-01", "3.5"},
		// trailing 12 months — pre-COVID, low and steady
		{"2020-01-01", "3.5"},
		{"2019-12-01", "3.6"},
		{"2019-11-01", "3.5"},
		{"2019-10-01", "3.6"},
		{"2019-09-01", "3.5"},
		{"2019-08-01", "3.7"},
		{"2019-07-01", "3.7"},
		{"2019-06-01", "3.6"},
		{"2019-05-01", "3.6"},
		{"2019-04-01", "3.6"},
		{"2019-03-01", "3.8"},
		{"2019-02-01", "3.8"},
	}
	sig := computeSahm(fixture(rows))
	if sig == nil {
		t.Fatal("computeSahm returned nil")
	}
	if !sig.Triggered {
		t.Errorf("expected Sahm to trigger at COVID onset; got Δ=%.2f", sig.Value)
	}
	if sig.Value < 1.0 {
		t.Errorf("Sahm Δ should be huge here; got %.2f", sig.Value)
	}
}

// TestSahmClear checks the indicator stays cool during a calm expansion.
func TestSahmClear(t *testing.T) {
	rows := []struct{ date, value string }{
		{"2019-12-01", "3.6"},
		{"2019-11-01", "3.5"},
		{"2019-10-01", "3.6"},
		{"2019-09-01", "3.5"},
		{"2019-08-01", "3.7"},
		{"2019-07-01", "3.7"},
		{"2019-06-01", "3.6"},
		{"2019-05-01", "3.6"},
		{"2019-04-01", "3.6"},
		{"2019-03-01", "3.8"},
		{"2019-02-01", "3.8"},
		{"2019-01-01", "4.0"},
		{"2018-12-01", "3.9"},
		{"2018-11-01", "3.7"},
		{"2018-10-01", "3.8"},
	}
	sig := computeSahm(fixture(rows))
	if sig == nil {
		t.Fatal("computeSahm returned nil")
	}
	if sig.Triggered {
		t.Errorf("Sahm fired during a calm expansion; Δ=%.2f", sig.Value)
	}
}

func TestYieldCurveInversion(t *testing.T) {
	sig := computeYieldCurve(fixture([]struct{ date, value string }{
		{"2024-06-01", "-0.42"},
	}))
	if sig == nil || !sig.Triggered {
		t.Errorf("inverted curve should trigger; got %+v", sig)
	}
	sig = computeYieldCurve(fixture([]struct{ date, value string }{
		{"2024-06-01", "0.55"},
	}))
	if sig == nil || sig.Triggered {
		t.Errorf("normal slope should not trigger; got %+v", sig)
	}
}

func TestYieldCurveSkipsMissing(t *testing.T) {
	// FRED uses "." for missing observations.
	sig := computeYieldCurve(fixture([]struct{ date, value string }{
		{"2024-06-02", "."},
		{"2024-06-01", "0.10"},
	}))
	if sig == nil {
		t.Fatal("expected to fall through to last known value")
	}
	if sig.Value != 0.10 {
		t.Errorf("expected 0.10, got %v", sig.Value)
	}
}

func TestNFCITightening(t *testing.T) {
	sig := computeNFCI(fixture([]struct{ date, value string }{
		{"2024-06-01", "0.30"},
	}))
	if sig == nil || !sig.Triggered {
		t.Errorf("positive NFCI should trigger; got %+v", sig)
	}
}

func TestHYSpreadStress(t *testing.T) {
	sig := computeHYSpread(fixture([]struct{ date, value string }{
		{"2024-06-01", "8.2"},
	}))
	if sig == nil || !sig.Triggered {
		t.Errorf("HY OAS at 8.2%% should trigger; got %+v", sig)
	}
	sig = computeHYSpread(fixture([]struct{ date, value string }{
		{"2024-06-01", "3.5"},
	}))
	if sig == nil || sig.Triggered {
		t.Errorf("HY OAS at 3.5%% should not trigger; got %+v", sig)
	}
}

// TestComposeMultiplier verifies that multiple triggered signals compound.
func TestComposeMultiplier(t *testing.T) {
	signals := []DataSignal{
		{Name: "Sahm Recession Indicator", Triggered: true},
		{Name: "10Y-2Y Treasury Spread", Triggered: true},
	}
	mult, fired := composeMultiplier(signals)
	want := 0.40 * 0.85 // 0.34
	if mult < want-0.001 || mult > want+0.001 {
		t.Errorf("expected ≈%.2f, got %.4f", want, mult)
	}
	if len(fired) != 2 {
		t.Errorf("expected 2 fired triggers, got %v", fired)
	}
}

// TestComposeMultiplierClear — no fired signals → 1.0.
func TestComposeMultiplierClear(t *testing.T) {
	mult, fired := composeMultiplier([]DataSignal{
		{Name: "Sahm Recession Indicator", Triggered: false},
		{Name: "10Y-2Y Treasury Spread", Triggered: false},
	})
	if mult != 1.0 {
		t.Errorf("expected 1.0, got %v", mult)
	}
	if len(fired) != 0 {
		t.Errorf("expected no fired triggers, got %v", fired)
	}
}

// TestAppliedMultiplierTakesMin — feed wins when more cautious.
func TestAppliedMultiplierTakesMin(t *testing.T) {
	feed := &DataFeed{Multiplier: 0.40}
	if got := AppliedMultiplier(0.80, feed); got != 0.40 {
		t.Errorf("feed should win at 0.40, got %v", got)
	}
	feed = &DataFeed{Multiplier: 0.95}
	if got := AppliedMultiplier(0.30, feed); got != 0.30 {
		t.Errorf("formula should win at 0.30, got %v", got)
	}
	if got := AppliedMultiplier(0.80, nil); got != 0.80 {
		t.Errorf("nil feed should pass formula through, got %v", got)
	}
}
