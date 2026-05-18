// Package cartography is a Go port of the Economic Cartography model.
//
//	Y(t) = Σₙ Aₙ · sin(2π · fₙt + φₙ) + ε(t)
//
// Time origin t = 0 corresponds to year 2000. The four bands (Kondratiev,
// Kuznets, Juglar, Kitchin) are illustrative constants chosen to roughly
// trace the shape of major historical inflections (1929, 1973, 2008, 2020)
// without curve-fitting them. The output is a navigational reading — a map,
// not a forecast — used by the trading algorithm to bias risk and surfaced
// through the /api/cartography endpoint for the UI.
package cartography

import (
	"math"
	"time"
)

// Band parameters. Names, periods, amplitudes, phases, and colors mirror
// the original economic-cartogrophy.tsx component verbatim.
type Band struct {
	Key         string  `json:"key"`
	Name        string  `json:"name"`
	Period      float64 `json:"period"`    // years
	Amplitude   float64 `json:"amplitude"` // sigma
	Phase       float64 `json:"phase"`     // fraction of cycle
	Color       string  `json:"color"`
	Description string  `json:"description"`
	Range       string  `json:"range"`
}

// BandKey identifiers.
const (
	BandKondratiev = "kondratiev"
	BandKuznets    = "kuznets"
	BandJuglar     = "juglar"
	BandKitchin    = "kitchin"
)

// Bands defines the four oscillators of the model in descending wavelength.
var Bands = []Band{
	{Key: BandKondratiev, Name: "Kondratiev", Period: 52.0, Amplitude: 1.10, Phase: 0.25,
		Color: "#c2410c", Description: "Techno-economic paradigm", Range: "40–60y"},
	{Key: BandKuznets, Name: "Kuznets", Period: 22.0, Amplitude: 0.70, Phase: 0.36,
		Color: "#a16207", Description: "Infrastructure & demographics", Range: "15–25y"},
	{Key: BandJuglar, Name: "Juglar", Period: 11.5, Amplitude: 0.90, Phase: 0.01,
		Color: "#0e7490", Description: "Capital & credit", Range: "7–11y"},
	{Key: BandKitchin, Name: "Kitchin", Period: 4.0, Amplitude: 0.35, Phase: 0.00,
		Color: "#4d7c0f", Description: "Inventory", Range: "3–5y"},
}

// Regime classifications.
const (
	RegimeConstructive = "CONSTRUCTIVE INTERFERENCE"
	RegimeDestructive  = "DESTRUCTIVE INTERFERENCE"
	RegimeRising       = "RISING WATERS"
	RegimeEbbing       = "EBBING TIDE"
	RegimeCrosswinds   = "CROSSWINDS"
)

// BandReading is the instantaneous state of a single band.
type BandReading struct {
	Band         Band    `json:"band"`
	Value        float64 `json:"value"`     // signed amplitude at t
	PhaseDegrees float64 `json:"phase_deg"` // 0..360
	State        string  `json:"state"`     // ASCENDING / CRESTING / DESCENDING / TROUGHING
	Description  string  `json:"description"`
}

// Regime is the projection layer — the formula compressed into a single
// navigational call the trading algorithm and UI can act on.
type Regime struct {
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Tone        string  `json:"tone"`
	Multiplier  float64 `json:"multiplier"` // recommended risk multiplier (0..1+)
}

// Reading is the full instrument-panel state at a given time.
type Reading struct {
	T              float64       `json:"t"` // years from 2000
	Year           float64       `json:"year"`
	AsOf           time.Time     `json:"as_of"`
	Composite      float64       `json:"composite"` // Y(t)
	CompositeAhead float64       `json:"composite_ahead"`
	Direction      float64       `json:"direction"` // d/dt sign-of-slope estimate
	Bands          []BandReading `json:"bands"`
	Regime         Regime        `json:"regime"`
}

// SeriesPoint is a single sample of the chart.
type SeriesPoint struct {
	Year       float64 `json:"year"`
	Composite  float64 `json:"composite"`
	Kondratiev float64 `json:"kondratiev"`
	Kuznets    float64 `json:"kuznets"`
	Juglar     float64 `json:"juglar"`
	Kitchin    float64 `json:"kitchin"`
}

// epoch is the year corresponding to t = 0.
const epoch = 2000.0

// TimeToT converts a wall-clock time to model time t (years past epoch).
func TimeToT(at time.Time) float64 {
	year := float64(at.Year())
	// fractional year, ignoring leap years (the formula's resolution doesn't
	// require exact day counts).
	day := float64(at.YearDay()) - 1
	frac := day / 365.0
	return (year + frac) - epoch
}

// BandValue returns A · sin(2π · (t/period + phase)).
func BandValue(b Band, t float64) float64 {
	return b.Amplitude * math.Sin(2*math.Pi*(t/b.Period+b.Phase))
}

// BandPhaseDeg returns the phase angle of the band at t in degrees [0,360).
func BandPhaseDeg(b Band, t float64) float64 {
	p := math.Mod(t/b.Period+b.Phase, 1.0) * 360.0
	if p < 0 {
		p += 360
	}
	return p
}

// describePhase maps a phase angle to a human-readable position label.
// Mirrors bandPosition() in the React component.
func describePhase(deg float64) (string, string) {
	switch {
	case deg < 45:
		return "ASCENDING", "rising from midline"
	case deg < 90:
		return "CRESTING", "approaching peak"
	case deg < 135:
		return "CRESTING", "just past peak"
	case deg < 225:
		return "DESCENDING", "falling toward midline"
	case deg < 270:
		return "TROUGHING", "approaching trough"
	case deg < 315:
		return "TROUGHING", "just past trough"
	default:
		return "ASCENDING", "rising toward midline"
	}
}

// composite returns Σ Aₙ · sin(2π · fₙt + φₙ) without the noise term —
// internal callers use this for the regime classification so the regime
// is stable rather than jittered by ε(t).
func composite(t float64) float64 {
	sum := 0.0
	for _, b := range Bands {
		sum += BandValue(b, t)
	}
	return sum
}

// noise is the deterministic pseudo-noise term ε(t) — visible in the chart
// but excluded from the regime classification.
func noise(t float64) float64 {
	return 0.10*math.Sin(t*7.31)*math.Cos(t*2.73) + 0.06*math.Sin(t*13.11)
}

// classifyRegime applies the regime rules from the React component and
// attaches a risk multiplier the algorithm can apply to position sizing.
//
//	CONSTRUCTIVE INTERFERENCE  Y > +1.4    multiplier 1.10  (following seas)
//	DESTRUCTIVE INTERFERENCE   Y < -1.4    multiplier 0.30  (storm waters)
//	RISING WATERS              direction > +0.05  multiplier 1.00
//	EBBING TIDE                direction < -0.05  multiplier 0.60
//	CROSSWINDS                 otherwise          multiplier 0.80
func classifyRegime(current, direction float64) Regime {
	switch {
	case current > 1.4:
		return Regime{
			Name:        RegimeConstructive,
			Description: "Multiple bands cresting in phase. Following seas — but watch the lee shore.",
			Tone:        "#d97706",
			Multiplier:  1.10,
		}
	case current < -1.4:
		return Regime{
			Name:        RegimeDestructive,
			Description: "Bands collapsed into trough alignment. Storm waters — capital seeks safe harbor.",
			Tone:        "#7f1d1d",
			Multiplier:  0.30,
		}
	case direction > 0.05:
		return Regime{
			Name:        RegimeRising,
			Description: "Net ascending. Shorter bands lead — longer bands will lag, then confirm.",
			Tone:        "#a16207",
			Multiplier:  1.00,
		}
	case direction < -0.05:
		return Regime{
			Name:        RegimeEbbing,
			Description: "Net descending. The bottom is not yet visible. Position for the next basin.",
			Tone:        "#7c2d12",
			Multiplier:  0.60,
		}
	default:
		return Regime{
			Name:        RegimeCrosswinds,
			Description: "Bands in opposition — interference cancellation. Direction undetermined; play structure, not phase.",
			Tone:        "#5a6a7a",
			Multiplier:  0.80,
		}
	}
}

// ReadingAt returns the full instrument-panel state at wall-clock time at.
func ReadingAt(at time.Time) Reading {
	t := TimeToT(at)
	current := composite(t)
	ahead := composite(t + 0.5)
	dir := ahead - current

	bands := make([]BandReading, len(Bands))
	for i, b := range Bands {
		deg := BandPhaseDeg(b, t)
		state, desc := describePhase(deg)
		bands[i] = BandReading{
			Band:         b,
			Value:        BandValue(b, t),
			PhaseDegrees: deg,
			State:        state,
			Description:  desc,
		}
	}

	return Reading{
		T:              t,
		Year:           epoch + t,
		AsOf:           at,
		Composite:      current,
		CompositeAhead: ahead,
		Direction:      dir,
		Bands:          bands,
		Regime:         classifyRegime(current, dir),
	}
}

// Series returns the chart series — dense samples from startYear to
// endYear at the given step (years per sample). Used by /api/cartography
// to feed the UI's composite waveform plot.
func Series(startYear, endYear, step float64) []SeriesPoint {
	if step <= 0 {
		step = 0.25
	}
	if endYear <= startYear {
		return nil
	}
	n := int((endYear-startYear)/step) + 1
	out := make([]SeriesPoint, 0, n)
	for y := startYear; y <= endYear+1e-9; y += step {
		t := y - epoch
		c := composite(t) + noise(t)
		out = append(out, SeriesPoint{
			Year:       round2(y),
			Composite:  round3(c),
			Kondratiev: round3(BandValue(Bands[0], t)),
			Kuznets:    round3(BandValue(Bands[1], t)),
			Juglar:     round3(BandValue(Bands[2], t)),
			Kitchin:    round3(BandValue(Bands[3], t)),
		})
	}
	return out
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }
func round3(v float64) float64 { return math.Round(v*1000) / 1000 }
