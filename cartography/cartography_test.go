package cartography

import (
	"math"
	"testing"
	"time"
)

// TestReadingMay2026 cross-checks the Go implementation against the JSX
// reference: at the chart's "NOW" preset (t = 26.4, May 2026) the regime
// should be EBBING TIDE — composite slightly negative and direction
// descending.
func TestReadingMay2026(t *testing.T) {
	// t = 26.4 corresponds to ~May 2026 in the model's coordinate system.
	at := time.Date(2026, time.May, 26, 0, 0, 0, 0, time.UTC) // ≈ t=26.4
	r := ReadingAt(at)

	if math.Abs(r.T-26.4) > 0.05 {
		t.Errorf("TimeToT mismatch: got %.3f, want ~26.4", r.T)
	}
	if r.Regime.Name != RegimeEbbing {
		t.Errorf("Regime mismatch: got %q, want %q", r.Regime.Name, RegimeEbbing)
	}
	if r.Composite >= 0 {
		t.Errorf("Composite at May 2026 expected negative, got %.3f", r.Composite)
	}
}

// TestBandPhaseRange asserts phase angles always fall in [0,360).
func TestBandPhaseRange(t *testing.T) {
	for ty := -75.0; ty <= 40.0; ty += 0.5 {
		for _, b := range Bands {
			deg := BandPhaseDeg(b, ty)
			if deg < 0 || deg >= 360 {
				t.Errorf("phase out of range: band=%s t=%.2f deg=%.3f", b.Key, ty, deg)
			}
		}
	}
}

// TestSeriesShape ensures the chart series spans the requested window.
func TestSeriesShape(t *testing.T) {
	s := Series(1925, 2040, 1)
	if len(s) < 100 {
		t.Fatalf("series too short: %d", len(s))
	}
	if s[0].Year != 1925 {
		t.Errorf("series start mismatch: %v", s[0].Year)
	}
}

// TestRegimeMultipliers sanity-check the risk multipliers — destructive
// must clamp risk down, constructive must scale it up.
func TestRegimeMultipliers(t *testing.T) {
	cases := []struct {
		composite, direction float64
		want                 string
		minMult, maxMult     float64
	}{
		{2.0, 0.0, RegimeConstructive, 1.05, 1.15},
		{-2.0, 0.0, RegimeDestructive, 0.0, 0.4},
		{0.0, 0.1, RegimeRising, 0.95, 1.05},
		{0.0, -0.1, RegimeEbbing, 0.5, 0.7},
		{0.0, 0.0, RegimeCrosswinds, 0.7, 0.9},
	}
	for _, c := range cases {
		got := classifyRegime(c.composite, c.direction)
		if got.Name != c.want {
			t.Errorf("regime mismatch: composite=%.2f dir=%.2f got=%s want=%s",
				c.composite, c.direction, got.Name, c.want)
		}
		if got.Multiplier < c.minMult || got.Multiplier > c.maxMult {
			t.Errorf("multiplier out of range for %s: got %.2f, want [%.2f,%.2f]",
				c.want, got.Multiplier, c.minMult, c.maxMult)
		}
	}
}
