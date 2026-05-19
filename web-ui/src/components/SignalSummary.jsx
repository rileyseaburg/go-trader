import React from 'react';
import { Card } from './common.jsx';
import { confidencePct } from '../lib/format.js';

function latestSignals(signals) {
  const raw = signals?.signals;
  if (!raw) return [];
  const values = Array.isArray(raw) ? raw : Object.values(raw).flat();
  return values.filter(Boolean).sort((a, b) => String(a.symbol).localeCompare(String(b.symbol)));
}

export default function SignalSummary({ signals }) {
  const rows = latestSignals(signals);
  return <Card title="Latest Signals">
    <p className="hint">Confidence is shown from the signal object's single <code>confidence</code> field; no UI placeholder is used.</p>
    <table className="audit-table signal-table">
      <thead><tr><th>Symbol</th><th>Signal</th><th>Confidence</th><th>Reason</th></tr></thead>
      <tbody>{rows.map((row) => <tr key={row.symbol}>
        <td><strong>{row.symbol}</strong></td><td><span className={`pill ${row.signal}`}>{row.signal}</span></td>
        <td>{confidencePct(row.confidence)}</td><td>{row.reasoning}</td>
      </tr>)}</tbody>
    </table>
    {rows.length === 0 && <p className="hint">No live signals loaded yet.</p>}
  </Card>;
}
