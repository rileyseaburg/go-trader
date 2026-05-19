import React from 'react';

export default function PhaseWheel({ bands, composite }) {
  const cx = 130, cy = 130, R = 105;
  return <svg viewBox="0 0 260 290" className="ec-wheel">
    <defs><radialGradient id="wheelBg" cx="50%" cy="50%"><stop offset="0%" stopColor="#15273a" /><stop offset="100%" stopColor="#0a141c" /></radialGradient></defs>
    <circle cx={cx} cy={cy} r={R + 5} fill="none" stroke="#5a3a1a" strokeWidth={2.5} />
    <circle cx={cx} cy={cy} r={R} fill="url(#wheelBg)" stroke="#5a4a3a" strokeWidth={0.5} />
    {[...Array(72)].map((_, i) => {
      const rad = (i * 5 - 90) * Math.PI / 180, major = i % 18 === 0;
      const inset = major ? 11 : i % 9 === 0 ? 7 : 3;
      return <line key={i} x1={cx + Math.cos(rad) * (R - inset)} y1={cy + Math.sin(rad) * (R - inset)} x2={cx + Math.cos(rad) * R} y2={cy + Math.sin(rad) * R} stroke={major ? '#d97706' : '#4a5a6a'} strokeWidth={major ? 1.1 : 0.5} />;
    })}
    <text x={cx} y={20} textAnchor="middle" className="ec-mono" fontSize="9" fill="#d97706" letterSpacing="3">ASC</text>
    <text x={cx + R + 10} y={cy + 3} textAnchor="start" className="ec-mono" fontSize="9" fill="#d97706" letterSpacing="3">PEAK</text>
    <text x={cx} y={cy + R + 18} textAnchor="middle" className="ec-mono" fontSize="9" fill="#d97706" letterSpacing="3">DESC</text>
    <text x={cx - R - 10} y={cy + 3} textAnchor="end" className="ec-mono" fontSize="9" fill="#d97706" letterSpacing="3">TROUGH</text>
    {bands.map((br, i) => {
      const rad = (br.phase_deg - 90) * Math.PI / 180, handLen = R * 0.94 - i * 12;
      const x = cx + Math.cos(rad) * handLen, y = cy + Math.sin(rad) * handLen;
      return <g key={br.band.key}><line x1={cx} y1={cy} x2={x} y2={y} stroke={br.band.color} strokeWidth={2.4} strokeLinecap="round" opacity={0.92} /><circle cx={x} cy={y} r={4.5} fill={br.band.color} stroke="#0a141c" strokeWidth={1.5} /></g>;
    })}
    <circle cx={cx} cy={cy} r={9} fill="#1a2a3a" stroke="#5a3a1a" strokeWidth={2} /><circle cx={cx} cy={cy} r={3} fill="#d97706" />
    <text x={cx} y={cy + R + 38} textAnchor="middle" className="ec-mono" fontSize="9" fill="#8a9aaa" letterSpacing="3">COMPOSITE Y(t)</text>
    <text x={cx} y={cy + R + 60} textAnchor="middle" className="ec-display" fontSize="22" fill="#e8d4a8">{composite >= 0 ? '+' : ''}{composite.toFixed(2)}σ</text>
  </svg>;
}
