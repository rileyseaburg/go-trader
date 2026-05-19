import React from 'react';

export default function BandRow({ br }) {
  const w = 100, h = 26;
  const path = [...Array(81)].map((_, i) => {
    const phase = (i / 80) * 360;
    const y = h / 2 - Math.sin(phase * Math.PI / 180) * (h / 2 - 3);
    return `${i === 0 ? 'M' : 'L'} ${((i / 80) * w).toFixed(1)} ${y.toFixed(1)}`;
  }).join(' ');
  const dotX = (br.phase_deg / 360) * w;
  const dotY = h / 2 - Math.sin(br.phase_deg * Math.PI / 180) * (h / 2 - 3);
  return <div className="ec-band" style={{ borderColor: br.band.color }}>
    <div className="ec-band-head"><div><div className="ec-band-name" style={{ color: br.band.color }}>{br.band.name}</div><div className="ec-band-meta">{br.band.description.toUpperCase()} · {br.band.range}</div></div>
      <svg viewBox={`0 0 ${w} ${h}`} width={w} height={h}><line x1={0} y1={h/2} x2={w} y2={h/2} stroke="#2a3a4a" strokeWidth={0.4} strokeDasharray="2 3" /><path d={path} fill="none" stroke={br.band.color} strokeWidth={1.1} opacity={0.5} /><circle cx={dotX} cy={dotY} r={3} fill={br.band.color} stroke="#0a141c" strokeWidth={1.2} /></svg>
    </div>
    <div className="ec-band-stats"><span><span className="o50">φ </span>{br.phase_deg.toFixed(0)}°</span><span><span className="o50">Y </span>{br.value >= 0 ? '+' : ''}{br.value.toFixed(2)}</span><span style={{ color: br.band.color }}>{br.state}</span></div>
    <div className="ec-band-desc">{br.description}</div>
  </div>;
}
