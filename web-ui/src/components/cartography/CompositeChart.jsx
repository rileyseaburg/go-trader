import React from 'react';
import { RECESSIONS } from '../../lib/cartographyData.js';

export default function CompositeChart({ series, currentYear, height = 260 }) {
  if (!series?.length) return null;
  const xMin = series[0].year, xMax = series[series.length - 1].year;
  const width = 720, padL = 28, padR = 12, padT = 10, padB = 22;
  const xs = x => padL + (x - xMin) / (xMax - xMin) * (width - padL - padR);
  const ys = y => padT + (3.5 - y) / 7 * (height - padT - padB);
  const path = key => series.map((p, i) => `${i === 0 ? 'M' : 'L'} ${xs(p.year).toFixed(1)} ${ys(p[key]).toFixed(1)}`).join(' ');
  return <svg viewBox={`0 0 ${width} ${height}`} className="ec-chart-svg" preserveAspectRatio="xMidYMid meet">
    {[-3,-2,-1,0,1,2,3].map(y => <g key={y}><line x1={padL} x2={width - padR} y1={ys(y)} y2={ys(y)} stroke="#1a2a3a" strokeDasharray="1 5" strokeWidth={0.6} /><text x={padL - 4} y={ys(y) + 3} textAnchor="end" fontSize="9" fill="#8a9aaa" className="ec-mono">{y}</text></g>)}
    {[1930,1950,1970,1990,2010,2030].map(x => <text key={x} x={xs(x)} y={height - 4} textAnchor="middle" fontSize="9" fill="#8a9aaa" className="ec-mono">{x}</text>)}
    {RECESSIONS.map((r, i) => <rect key={i} x={xs(r.start)} y={padT} width={xs(r.end) - xs(r.start)} height={height - padT - padB} fill="#7f1d1d" fillOpacity={0.16} stroke="#7f1d1d" strokeOpacity={0.35} strokeWidth={0.5} />)}
    <line x1={padL} x2={width - padR} y1={ys(0)} y2={ys(0)} stroke="#3a4a5a" strokeWidth={0.6} />
    <path d={path('kondratiev')} fill="none" stroke="#c2410c" strokeWidth={1} opacity={0.5} />
    <path d={path('kuznets')} fill="none" stroke="#a16207" strokeWidth={1} opacity={0.5} />
    <path d={path('juglar')} fill="none" stroke="#0e7490" strokeWidth={1} opacity={0.5} />
    <path d={path('kitchin')} fill="none" stroke="#4d7c0f" strokeWidth={1} opacity={0.5} />
    <path d={path('composite')} fill="none" stroke="#e8d4a8" strokeWidth={1.8} />
    {currentYear != null && <line x1={xs(currentYear)} x2={xs(currentYear)} y1={padT} y2={height - padB} stroke="#d97706" strokeWidth={1.5} strokeDasharray="3 3" />}
  </svg>;
}
