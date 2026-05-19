import React from 'react';
import { Card } from './common.jsx';
import { confidencePct } from '../lib/format.js';

function latestSignals(signals) {
  const raw = signals?.signals;
  if (!raw) return [];
  const values = Array.isArray(raw) ? raw : Object.values(raw).flat();
  return values.filter(Boolean).sort((a, b) => String(a.symbol).localeCompare(String(b.symbol)));
}

function fmt(value, digits = 2) {
  return Number.isFinite(Number(value)) ? Number(value).toFixed(digits) : '—';
}

function PipelineDetails({ row }) {
  const audit = row.audit;
  if (!audit) return <span className="hint">No structured audit payload yet.</span>;
  const scores = audit.raw_strategy_scores || [];
  const risk = audit.risk_gate || {};
  const sizing = audit.order_sizing || {};
  return <details className="pipeline-details">
    <summary>Scoring pipeline</summary>
    <div className="pipeline-grid">
      <div><span>Raw net</span><strong>{fmt(audit.normalized_scores?.raw_net)}</strong></div>
      <div><span>Adjusted net</span><strong>{fmt(audit.normalized_scores?.adjusted_net)}</strong></div>
      <div><span>Canonical confidence</span><strong>{confidencePct(audit.confidence_calculation?.canonical_confidence ?? row.confidence)}</strong></div>
      <div><span>Regime</span><strong>{audit.regime_adjustment?.regime_name || '—'} ×{fmt(audit.regime_adjustment?.regime_multiplier)}</strong></div>
      <div><span>Risk gate</span><strong>{risk.blocks_order ? 'BLOCKED' : risk.passed === false ? 'WARN' : 'PASS'}</strong></div>
      <div><span>Est. qty</span><strong>{fmt(sizing.estimated_qty, 0)}</strong></div>
    </div>
    {risk.violations?.length > 0 && <ul className="risk-list">{risk.violations.map((v) => <li key={v}>{v}</li>)}</ul>}
    {scores.length > 0 && <table className="audit-table"><thead><tr><th>Strategy</th><th>Dir</th><th>Conf</th><th>Weight</th><th>Weighted</th></tr></thead>
      <tbody>{scores.map((s) => <tr key={s.strategy}><td>{s.strategy}</td><td>{fmt(s.direction)}</td><td>{confidencePct(s.confidence)}</td><td>{fmt(s.weight)}</td><td>{fmt(s.weighted_score)}</td></tr>)}</tbody></table>}
  </details>;
}

export default function SignalSummary({ signals }) {
  const rows = latestSignals(signals);
  return <Card title="Latest Signals">
    <p className="hint">Confidence is canonical: the API <code>confidence</code>, reasoning text, and structured audit payload now use the same final value.</p>
    <table className="audit-table signal-table">
      <thead><tr><th>Symbol</th><th>Signal</th><th>Confidence</th><th>Reason</th></tr></thead>
      <tbody>{rows.map((row) => <tr key={row.symbol}>
        <td><strong>{row.symbol}</strong></td><td><span className={`pill ${row.signal}`}>{row.signal}</span></td>
        <td>{confidencePct(row.confidence)}</td><td>{row.reasoning}<PipelineDetails row={row} /></td>
      </tr>)}</tbody>
    </table>
    {rows.length === 0 && <p className="hint">No live signals loaded yet.</p>}
  </Card>;
}
