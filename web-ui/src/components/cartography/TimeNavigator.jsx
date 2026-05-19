import React from 'react';
import { PRESETS } from '../../lib/cartographyData.js';

export default function TimeNavigator({ year, onChange }) {
  return <div className="ec-nav">
    <div className="ec-eyebrow">⊹ NAVIGATE TIME</div>
    <input type="range" min={1925} max={2040} step={0.25} value={year} onChange={e => onChange(parseFloat(e.target.value))} />
    <div className="ec-nav-scale"><span>1925</span><span>1980</span><span>2040</span></div>
    <div className="ec-presets">{PRESETS.map(p => <button key={p.label} onClick={() => onChange(p.year + (p.label === 'NOW' ? new Date().getMonth() / 12 : 0))} className={Math.abs(year - p.year) < 0.5 ? 'on' : ''}>{p.label}</button>)}</div>
  </div>;
}
