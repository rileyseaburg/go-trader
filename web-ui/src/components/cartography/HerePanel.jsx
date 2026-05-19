import React from 'react';
import { yearParts } from '../../lib/cartographyData.js';

export default function HerePanel({ reading, appliedMultiplier }) {
  const { yearInt, month } = yearParts(reading.year);
  return <div className="ec-here">
    <div><div className="ec-eyebrow">✦ YOU ARE HERE</div><div className="ec-here-date">{month} {yearInt}</div></div>
    <div className="ec-here-right"><div className="ec-eyebrow">REGIME</div><div className="ec-here-regime" style={{ color: reading.regime.tone }}>{reading.regime.name}</div><div className="ec-here-mult">formula × <strong>{reading.regime.multiplier.toFixed(2)}</strong>{appliedMultiplier != null && <span className="o50"> · applied: ×{Number(appliedMultiplier).toFixed(2)}</span>}</div></div>
    <div className="ec-here-desc">{reading.regime.description}</div>
    <div className="ec-capital-warning">Allocation note: the cycle chart is formula-first; only the FRED overlay is data-backed. Treat this multiplier as a risk governor, not a validated alpha signal.</div>
  </div>;
}
