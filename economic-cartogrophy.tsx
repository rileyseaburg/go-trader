import React, { useState, useMemo } from 'react';
import {
  LineChart, Line, XAxis, YAxis, CartesianGrid,
  ReferenceLine, ReferenceArea, ResponsiveContainer
} from 'recharts';

// ─────────────────────────────────────────────────────────────────
// FORMULA:  Y(t) = Σₙ Aₙ · sin(2π·fₙ·t + φₙ) + ε(t)
// Time origin: t = 0 corresponds to year 2000.
// Band parameters chosen so the composite roughly traces
// 1929, 1973, 2008, 2020 inflections without curve-fitting them.
// ─────────────────────────────────────────────────────────────────

const BANDS = [
  { key: 'kondratiev', period: 52,  amplitude: 1.10, phase: 0.25,
    color: '#c2410c', name: 'Kondratiev',
    desc: 'Techno-economic paradigm', range: '40–60y' },
  { key: 'kuznets',    period: 22,  amplitude: 0.70, phase: 0.36,
    color: '#a16207', name: 'Kuznets',
    desc: 'Infrastructure & demographics', range: '15–25y' },
  { key: 'juglar',     period: 11.5,amplitude: 0.90, phase: 0.01,
    color: '#0e7490', name: 'Juglar',
    desc: 'Capital & credit', range: '7–11y' },
  { key: 'kitchin',    period: 4,   amplitude: 0.35, phase: 0.00,
    color: '#4d7c0f', name: 'Kitchin',
    desc: 'Inventory', range: '3–5y' },
];

const RECESSIONS = [
  { start: 1929.7, end: 1933.3, label: 'GREAT DEPRESSION' },
  { start: 1937.4, end: 1938.5, label: '' },
  { start: 1973.9, end: 1975.2, label: 'OIL SHOCK' },
  { start: 1980.0, end: 1980.5, label: '' },
  { start: 1981.5, end: 1982.9, label: 'VOLCKER' },
  { start: 1990.5, end: 1991.2, label: 'S&L' },
  { start: 2001.2, end: 2001.9, label: 'DOTCOM' },
  { start: 2007.9, end: 2009.5, label: 'GFC' },
  { start: 2020.1, end: 2020.4, label: 'COVID' },
];

const PRESETS = [
  { t: -71,    label: '1929' },
  { t: -27,    label: '1973' },
  { t: 0,      label: '2000' },
  { t: 8.7,    label: '2008' },
  { t: 20.16,  label: '2020' },
  { t: 26.4,   label: 'NOW' },
  { t: 35,     label: '2035' },
];

const bandValue = (b, t) =>
  b.amplitude * Math.sin(2 * Math.PI * (t / b.period + b.phase));

const bandPhaseDeg = (b, t) => {
  let p = ((t / b.period + b.phase) % 1) * 360;
  return p < 0 ? p + 360 : p;
};

const bandPosition = (p) => {
  if (p < 45)  return { state: 'ASCENDING',  desc: 'rising from midline' };
  if (p < 90)  return { state: 'CRESTING',   desc: 'approaching peak' };
  if (p < 135) return { state: 'CRESTING',   desc: 'just past peak' };
  if (p < 225) return { state: 'DESCENDING', desc: 'falling toward midline' };
  if (p < 270) return { state: 'TROUGHING',  desc: 'approaching trough' };
  if (p < 315) return { state: 'TROUGHING',  desc: 'just past trough' };
  return { state: 'ASCENDING', desc: 'rising toward midline' };
};

const compositeValue = (t) =>
  BANDS.reduce((s, b) => s + bandValue(b, t), 0);

// deterministic pseudo-noise — the ε(t) term made visible
const noise = (t) =>
  0.10 * Math.sin(t * 7.31) * Math.cos(t * 2.73)
+ 0.06 * Math.sin(t * 13.11);

// ─────────────────────────────────────────────────────────────────

const STYLE = `
  @import url('https://fonts.googleapis.com/css2?family=Fraunces:opsz,wght@9..144,300;9..144,400;9..144,500;9..144,600&family=JetBrains+Mono:wght@300;400;500&display=swap');
  .ec-display { font-family: 'Fraunces', serif; font-variation-settings: "opsz" 144; }
  .ec-body    { font-family: 'Fraunces', serif; font-variation-settings: "opsz" 14; }
  .ec-mono    { font-family: 'JetBrains Mono', monospace; }
  .ec-chart   { background: radial-gradient(ellipse at center, #0f1e2c 0%, #0a141c 100%); }
  .ec-grain::before {
    content: ''; position: absolute; inset: 0; pointer-events: none;
    opacity: 0.04;
    background-image: url("data:image/svg+xml,%3Csvg viewBox='0 0 200 200' xmlns='http://www.w3.org/2000/svg'%3E%3Cfilter id='n'%3E%3CfeTurbulence type='fractalNoise' baseFrequency='0.85' numOctaves='2'/%3E%3C/filter%3E%3Crect width='100%25' height='100%25' filter='url(%23n)'/%3E%3C/svg%3E");
  }
  input[type="range"]::-webkit-slider-thumb {
    -webkit-appearance: none; appearance: none;
    width: 18px; height: 18px; border-radius: 50%;
    background: #d97706; border: 2px solid #1a2a3a;
    cursor: pointer; box-shadow: 0 0 0 1px #d97706;
  }
  input[type="range"]::-moz-range-thumb {
    width: 18px; height: 18px; border-radius: 50%;
    background: #d97706; border: 2px solid #1a2a3a;
    cursor: pointer;
  }
  input[type="range"] {
    -webkit-appearance: none; appearance: none;
    background: transparent; height: 4px;
  }
  input[type="range"]::-webkit-slider-runnable-track {
    background: linear-gradient(to right, #3a4a5a, #5a6a7a); height: 2px; border-radius: 2px;
  }
  input[type="range"]::-moz-range-track {
    background: #3a4a5a; height: 2px; border-radius: 2px;
  }
`;

// ─────────────────────────────────────────────────────────────────

function PhaseWheel({ readings, currentValue }) {
  const cx = 160, cy = 160, R = 130;

  return (
    <svg viewBox="0 0 320 360" className="w-full max-w-sm mx-auto block">
      <defs>
        <radialGradient id="wheelBg" cx="50%" cy="50%">
          <stop offset="0%"   stopColor="#15273a" />
          <stop offset="100%" stopColor="#0a141c" />
        </radialGradient>
        <linearGradient id="rim" x1="0%" y1="0%" x2="0%" y2="100%">
          <stop offset="0%"   stopColor="#8a5a2a" />
          <stop offset="50%"  stopColor="#5a3a1a" />
          <stop offset="100%" stopColor="#3a2410" />
        </linearGradient>
      </defs>

      {/* outer brass rim */}
      <circle cx={cx} cy={cy} r={R + 6} fill="none" stroke="url(#rim)" strokeWidth={3} />
      <circle cx={cx} cy={cy} r={R + 1} fill="url(#wheelBg)" stroke="#5a4a3a" strokeWidth={0.5} />
      <circle cx={cx} cy={cy} r={R - 6} fill="none" stroke="#2a3a4a" strokeWidth={0.5} strokeDasharray="1 4" />
      <circle cx={cx} cy={cy} r={R * 0.62} fill="none" stroke="#2a3a4a" strokeWidth={0.4} strokeDasharray="2 4" />
      <circle cx={cx} cy={cy} r={R * 0.32} fill="none" stroke="#2a3a4a" strokeWidth={0.4} strokeDasharray="2 4" />

      {/* tick marks */}
      {[...Array(72)].map((_, i) => {
        const angle = i * 5 - 90;
        const rad = angle * Math.PI / 180;
        const major = i % 18 === 0;
        const mid = i % 9 === 0 && !major;
        const inset = major ? 14 : mid ? 8 : 4;
        const x1 = cx + Math.cos(rad) * (R - inset);
        const y1 = cy + Math.sin(rad) * (R - inset);
        const x2 = cx + Math.cos(rad) * R;
        const y2 = cy + Math.sin(rad) * R;
        return <line key={i} x1={x1} y1={y1} x2={x2} y2={y2}
          stroke={major ? '#d97706' : mid ? '#8a6a3a' : '#4a5a6a'}
          strokeWidth={major ? 1.2 : 0.5} />;
      })}

      {/* cardinal labels */}
      <text x={cx} y={28} textAnchor="middle" className="ec-mono"
            fontSize="9" fill="#d97706" letterSpacing="3">ASC</text>
      <text x={cx + R + 14} y={cy + 3} textAnchor="start" className="ec-mono"
            fontSize="9" fill="#d97706" letterSpacing="3">PEAK</text>
      <text x={cx} y={cy + R + 22} textAnchor="middle" className="ec-mono"
            fontSize="9" fill="#d97706" letterSpacing="3">DESC</text>
      <text x={cx - R - 14} y={cy + 3} textAnchor="end" className="ec-mono"
            fontSize="9" fill="#d97706" letterSpacing="3">TROUGH</text>

      {/* hands */}
      {readings.map((r, i) => {
        const svgAngle = (r.phaseDeg - 90) * Math.PI / 180;
        const handLen = R * 0.94 - i * 14;
        const x = cx + Math.cos(svgAngle) * handLen;
        const y = cy + Math.sin(svgAngle) * handLen;
        return (
          <g key={r.key}>
            <line x1={cx} y1={cy} x2={x} y2={y}
                  stroke={r.color} strokeWidth={2.4}
                  strokeLinecap="round" opacity={0.9} />
            <circle cx={x} cy={y} r={5}
                    fill={r.color} stroke="#0a141c" strokeWidth={1.5} />
          </g>
        );
      })}

      {/* center hub */}
      <circle cx={cx} cy={cy} r={11} fill="#1a2a3a" stroke="url(#rim)" strokeWidth={2} />
      <circle cx={cx} cy={cy} r={3.5} fill="#d97706" />

      {/* readout below */}
      <text x={cx} y={cy + R + 56} textAnchor="middle" className="ec-mono"
            fontSize="9" fill="#8a9aaa" letterSpacing="3">COMPOSITE  Y(t)</text>
      <text x={cx} y={cy + R + 84} textAnchor="middle" className="ec-display"
            fontSize="32" fill="#e8d4a8" fontWeight="500">
        {currentValue >= 0 ? '+' : ''}{currentValue.toFixed(2)}σ
      </text>
    </svg>
  );
}

// ─────────────────────────────────────────────────────────────────

function BandPanel({ reading }) {
  const w = 110, h = 30;
  const path = [...Array(81)].map((_, i) => {
    const phase = (i / 80) * 360;
    const y = h / 2 - Math.sin(phase * Math.PI / 180) * (h / 2 - 3);
    const x = (i / 80) * w;
    return `${i === 0 ? 'M' : 'L'} ${x.toFixed(2)} ${y.toFixed(2)}`;
  }).join(' ');
  const dotX = (reading.phaseDeg / 360) * w;
  const dotY = h / 2 - Math.sin(reading.phaseDeg * Math.PI / 180) * (h / 2 - 3);

  return (
    <div className="border-l-2 pl-4 py-3.5" style={{ borderColor: reading.color }}>
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="ec-display text-xl tracking-tight leading-none"
               style={{ color: reading.color, fontWeight: 500 }}>
            {reading.name}
          </div>
          <div className="ec-mono text-[10px] opacity-60 mt-1.5 tracking-wide">
            {reading.desc.toUpperCase()} · {reading.range}
          </div>
        </div>
        <svg viewBox={`0 0 ${w} ${h}`} width={w} height={h} className="flex-shrink-0">
          <line x1={0} y1={h/2} x2={w} y2={h/2}
                stroke="#2a3a4a" strokeWidth={0.4} strokeDasharray="2 3" />
          <path d={path} fill="none" stroke={reading.color}
                strokeWidth={1.2} opacity={0.55} />
          <circle cx={dotX} cy={dotY} r={3.5}
                  fill={reading.color} stroke="#0a141c" strokeWidth={1.5} />
        </svg>
      </div>
      <div className="flex items-baseline gap-5 mt-2.5 ec-mono text-[11px]">
        <div>
          <span className="opacity-50">φ </span>
          <span className="opacity-95">{reading.phaseDeg.toFixed(0)}°</span>
        </div>
        <div>
          <span className="opacity-50">Y </span>
          <span className="opacity-95">
            {reading.value >= 0 ? '+' : ''}{reading.value.toFixed(2)}
          </span>
        </div>
        <div className="uppercase tracking-wider"
             style={{ color: reading.color }}>
          {reading.position.state}
        </div>
      </div>
      <div className="ec-body text-[13px] opacity-65 mt-1 italic">
        {reading.position.desc}
      </div>
    </div>
  );
}

// ─────────────────────────────────────────────────────────────────

export default function EconomicCartography() {
  const [timeT, setTimeT] = useState(26.4); // May 2026

  const chartData = useMemo(() => {
    const data = [];
    for (let t = -75; t <= 40; t += 0.25) {
      data.push({
        year:       parseFloat((2000 + t).toFixed(2)),
        composite:  parseFloat((compositeValue(t) + noise(t)).toFixed(3)),
        kondratiev: parseFloat(bandValue(BANDS[0], t).toFixed(3)),
        kuznets:    parseFloat(bandValue(BANDS[1], t).toFixed(3)),
        juglar:     parseFloat(bandValue(BANDS[2], t).toFixed(3)),
        kitchin:    parseFloat(bandValue(BANDS[3], t).toFixed(3)),
      });
    }
    return data;
  }, []);

  const currentYear = 2000 + timeT;
  const yearInt = Math.floor(currentYear);
  const monthIdx = Math.floor((currentYear - yearInt) * 12);
  const monthName = ['JAN','FEB','MAR','APR','MAY','JUN','JUL','AUG','SEP','OCT','NOV','DEC'][monthIdx];
  const currentValue = compositeValue(timeT);
  const futureValue = compositeValue(timeT + 0.5);
  const direction = futureValue - currentValue;

  const bandReadings = BANDS.map(b => ({
    ...b,
    value: bandValue(b, timeT),
    phaseDeg: bandPhaseDeg(b, timeT),
    position: bandPosition(bandPhaseDeg(b, timeT))
  }));

  let regime;
  if (currentValue > 1.4) regime = {
    name: 'CONSTRUCTIVE INTERFERENCE',
    desc: 'Multiple bands cresting in phase. Following seas — but watch the lee shore.',
    tone: '#d97706'
  };
  else if (currentValue < -1.4) regime = {
    name: 'DESTRUCTIVE INTERFERENCE',
    desc: 'Bands collapsed into trough alignment. Storm waters — capital seeks safe harbor.',
    tone: '#7f1d1d'
  };
  else if (direction > 0.05) regime = {
    name: 'RISING WATERS',
    desc: 'Net ascending. Shorter bands lead — longer bands will lag, then confirm.',
    tone: '#a16207'
  };
  else if (direction < -0.05) regime = {
    name: 'EBBING TIDE',
    desc: 'Net descending. The bottom is not yet visible. Position for the next basin.',
    tone: '#7c2d12'
  };
  else regime = {
    name: 'CROSSWINDS',
    desc: 'Bands in opposition — interference cancellation. Direction undetermined; play structure, not phase.',
    tone: '#5a6a7a'
  };

  return (
    <>
      <style>{STYLE}</style>
      <div className="ec-body relative min-h-screen ec-grain"
           style={{ background: '#0a141c', color: '#e8d4a8' }}>

        {/* ─── HEADER ─────────────────────────────────────────── */}
        <header className="px-5 pt-8 pb-5 border-b" style={{ borderColor: '#1a2a3a' }}>
          <div className="ec-mono text-[10px] tracking-[0.3em] opacity-50">
            CHART № I · CYCLES OF THE INDUSTRIAL ECONOMY
          </div>
          <h1 className="ec-display text-[2.5rem] mt-2.5 leading-[0.95]"
              style={{ fontWeight: 400 }}>
            Economic <em style={{ color: '#d97706', fontStyle: 'italic' }}>
              Cartography</em>
          </h1>
          <div className="ec-mono text-[11px] mt-3.5 opacity-70 tracking-wide">
            Y(t) = ∑ Aₙ · sin(2π · fₙt + φₙ) + ε(t)
          </div>
          <div className="ec-body text-[13px] mt-1.5 opacity-50 italic">
            Four bands. One sea. The waveform <em>is</em> the territory.
          </div>
        </header>

        {/* ─── HERO MAP ───────────────────────────────────────── */}
        <section className="pt-6 pb-3 ec-chart">
          <div className="px-5 ec-mono text-[10px] tracking-[0.25em] opacity-60 mb-3">
            ⌁ THE COMPOSITE WAVEFORM · 1925 ─ 2040
          </div>
          <div style={{ height: 340 }}>
            <ResponsiveContainer>
              <LineChart data={chartData}
                         margin={{ top: 10, right: 18, left: 0, bottom: 5 }}>
                <CartesianGrid stroke="#1a2a3a" strokeDasharray="1 5" vertical={false} />
                <XAxis
                  dataKey="year" type="number" domain={[1925, 2040]}
                  ticks={[1930, 1950, 1970, 1990, 2010, 2030]}
                  tick={{ fill: '#8a9aaa', fontSize: 10, fontFamily: 'JetBrains Mono' }}
                  tickFormatter={v => Math.floor(v).toString()}
                  stroke="#3a4a5a"
                />
                <YAxis
                  domain={[-3.5, 3.5]} ticks={[-3, -2, -1, 0, 1, 2, 3]}
                  tick={{ fill: '#8a9aaa', fontSize: 10, fontFamily: 'JetBrains Mono' }}
                  stroke="#3a4a5a" tickFormatter={v => v.toFixed(0)}
                />
                {RECESSIONS.map((r, i) => (
                  <ReferenceArea key={i} x1={r.start} x2={r.end}
                    fill="#7f1d1d" fillOpacity={0.16}
                    stroke="#7f1d1d" strokeOpacity={0.35} />
                ))}
                <ReferenceLine y={0} stroke="#3a4a5a" strokeWidth={0.5} />
                {BANDS.map(b => (
                  <Line key={b.key} type="monotone" dataKey={b.key}
                        stroke={b.color} strokeWidth={1}
                        dot={false} opacity={0.5} isAnimationActive={false} />
                ))}
                <Line type="monotone" dataKey="composite"
                      stroke="#e8d4a8" strokeWidth={1.8}
                      dot={false} isAnimationActive={false} />
                <ReferenceLine x={currentYear} stroke="#d97706"
                               strokeWidth={1.5} strokeDasharray="3 3" />
              </LineChart>
            </ResponsiveContainer>
          </div>

          <div className="px-5 mt-3 flex flex-wrap gap-x-4 gap-y-1.5 ec-mono text-[10px]">
            {BANDS.map(b => (
              <div key={b.key} className="flex items-center gap-1.5">
                <div className="w-3 h-[2px]" style={{ background: b.color }} />
                <span className="opacity-65 tracking-wide">{b.name.toUpperCase()}</span>
              </div>
            ))}
            <div className="flex items-center gap-1.5">
              <div className="w-3 h-[2px]" style={{ background: '#e8d4a8' }} />
              <span className="opacity-65 tracking-wide">COMPOSITE</span>
            </div>
            <div className="flex items-center gap-1.5">
              <div className="w-3 h-[8px]" style={{ background: '#7f1d1d', opacity: 0.4 }} />
              <span className="opacity-65 tracking-wide">RECESSION</span>
            </div>
          </div>
        </section>

        {/* ─── YOU ARE HERE ───────────────────────────────────── */}
        <section className="px-5 py-6 border-y"
                 style={{ borderColor: '#1a2a3a', background: '#080f15' }}>
          <div className="flex items-baseline justify-between gap-4 flex-wrap">
            <div>
              <div className="ec-mono text-[10px] tracking-[0.3em] opacity-60">
                ✦ YOU ARE HERE
              </div>
              <div className="ec-display text-[2rem] mt-1 leading-none"
                   style={{ color: '#d97706', fontWeight: 500 }}>
                {monthName} {yearInt}
              </div>
            </div>
            <div className="text-right">
              <div className="ec-mono text-[10px] tracking-[0.25em] opacity-60">
                REGIME
              </div>
              <div className="ec-display text-[1.05rem] mt-1 tracking-tight"
                   style={{ color: regime.tone, fontWeight: 500 }}>
                {regime.name}
              </div>
            </div>
          </div>
          <div className="ec-body text-[14px] mt-4 opacity-80 italic leading-relaxed">
            {regime.desc}
          </div>
        </section>

        {/* ─── PHASE COMPASS ──────────────────────────────────── */}
        <section className="px-5 pt-8 pb-2">
          <div className="ec-mono text-[10px] tracking-[0.3em] opacity-60 mb-4 text-center">
            ✧ THE PHASE COMPASS
          </div>
          <PhaseWheel readings={bandReadings} currentValue={currentValue} />
        </section>

        {/* ─── BAND READOUTS ──────────────────────────────────── */}
        <section className="px-5 pt-6 pb-2">
          <div className="ec-mono text-[10px] tracking-[0.3em] opacity-60 mb-4">
            ⌁ THE BANDS
          </div>
          <div className="space-y-1">
            {bandReadings.map(r => <BandPanel key={r.key} reading={r} />)}
          </div>
        </section>

        {/* ─── TIME NAVIGATION ────────────────────────────────── */}
        <section className="px-5 pt-8 pb-6 mt-6 border-t"
                 style={{ borderColor: '#1a2a3a' }}>
          <div className="ec-mono text-[10px] tracking-[0.3em] opacity-60 mb-4">
            ⊹ NAVIGATE TIME
          </div>
          <input
            type="range" min={-75} max={40} step={0.1}
            value={timeT}
            onChange={e => setTimeT(parseFloat(e.target.value))}
            className="w-full"
          />
          <div className="flex justify-between ec-mono text-[10px] mt-2 opacity-50">
            <span>1925</span>
            <span>1980</span>
            <span>2040</span>
          </div>
          <div className="flex flex-wrap gap-2 mt-5">
            {PRESETS.map(p => {
              const active = Math.abs(timeT - p.t) < 0.5;
              return (
                <button key={p.label}
                  onClick={() => setTimeT(p.t)}
                  className="ec-mono text-[11px] px-3 py-1.5 border tracking-wider transition-all"
                  style={{
                    borderColor: active ? '#d97706' : '#3a4a5a',
                    color:       active ? '#d97706' : '#8a9aaa',
                    background:  active ? 'rgba(217,119,6,0.08)' : 'transparent'
                  }}>
                  {p.label}
                </button>
              );
            })}
          </div>
        </section>

        {/* ─── CARTOUCHE ──────────────────────────────────────── */}
        <footer className="px-5 py-7 border-t"
                style={{ borderColor: '#1a2a3a', background: '#080f15' }}>
          <div className="ec-mono text-[10px] tracking-[0.3em] opacity-50 mb-3">
            ❦ MARGINALIA
          </div>
          <div className="ec-body text-[12.5px] opacity-60 leading-relaxed italic">
            A map, not a forecast. Periods, amplitudes, and phases are
            illustrative constants chosen to roughly trace the shape of
            major historical inflections — 1929, 1973, 2008, 2020 — without
            curve-fitting them. Real bands drift, couple non-linearly, and
            answer to forcing functions the formula cannot anticipate. Use
            this instrument to <em>see</em> structure, not to predict it.
            The waveform <em>is</em> the territory; cycles are only where
            the waveform stores its energy.
          </div>
          <div className="ec-mono text-[9px] tracking-[0.25em] opacity-40 mt-5">
            DRAWN FROM FIRST PRINCIPLES · NO HISTORICAL DATA INGESTED · v0.1
          </div>
        </footer>
      </div>
    </>
  );
}
