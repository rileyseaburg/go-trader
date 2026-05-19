import React from 'react';
import { Card } from './common.jsx';
import { formatDate, money, pct } from '../lib/format.js';

function latestSignal(signals) {
  const raw = signals?.signals;
  if (!raw) return null;
  const values = (Array.isArray(raw) ? raw : Object.values(raw).flat()).filter(Boolean);
  return values.sort((a, b) => new Date(b.timestamp || 0) - new Date(a.timestamp || 0))[0] || null;
}

function latestBrokerOrder(orders) {
  const rows = Array.isArray(orders) ? orders : [];
  return rows
    .filter(Boolean)
    .sort((a, b) => new Date(b.submitted_at || b.created_at || b.updated_at || 0) - new Date(a.submitted_at || a.created_at || a.updated_at || 0))[0] || null;
}

function latestAuditRow(rows) {
  return (rows || [])
    .filter(Boolean)
    .sort((a, b) => new Date(b.created_at || 0) - new Date(a.created_at || 0))[0] || null;
}

function modeText(status) {
  const mode = status?.trading_mode || 'UNKNOWN';
  if (mode === 'LIVE_PAPER') return 'LIVE PAPER';
  if (mode === 'DRY_RUN') return 'DRY RUN';
  if (mode === 'MOCK') return 'MOCK';
  return mode.replaceAll('_', ' ');
}

export default function OperatorStatus({ data }) {
  const status = data.runtimeStatus || {};
  const account = data.account || {};
  const positions = Array.isArray(data.positions) ? data.positions : [];
  const latest = latestSignal(data.signals);
  const auditOrder = latestAuditRow(data.auditOrders?.rows);
  const auditFill = latestAuditRow(data.auditFills?.rows);
  const brokerOrder = latestBrokerOrder(data.orders);
  const exposure = positions.reduce((sum, p) => sum + Math.abs(Number.parseFloat(p.market_value || 0) || 0), 0);
  const equity = Number.parseFloat(account.portfolio_value || account.equity || 0);
  const mode = status.trading_mode || 'UNKNOWN';
  const modeClass = mode === 'LIVE_PAPER' ? 'live' : mode === 'DRY_RUN' || mode === 'MOCK' ? 'dry' : 'unknown';

  return <Card title="Operator Status">
    <div className={`operator-badge ${modeClass}`}>
      <div>
        <span>MODE</span>
        <strong>{modeText(status)}</strong>
      </div>
      <div>
        <span>Broker connected</span>
        <strong>{status.broker ? 'YES' : 'UNKNOWN'}</strong>
      </div>
      <div>
        <span>Orders allowed</span>
        <strong>{status.order_submission_enabled ? 'YES' : 'NO'}</strong>
      </div>
      <div>
        <span>Algorithm</span>
        <strong>{status.algorithm_running ? 'RUNNING' : 'STOPPED'}</strong>
      </div>
    </div>

    <div className="metric-grid operator-metrics">
      <div><span>Portfolio value</span><strong>{money(account.portfolio_value || account.equity)}</strong></div>
      <div><span>Cash</span><strong>{money(account.cash)}</strong></div>
      <div><span>Exposure</span><strong>{money(exposure)} <small>{Number.isFinite(equity) && equity > 0 ? pct(exposure / equity) : '—'}</small></strong></div>
      <div><span>Open positions</span><strong>{positions.length}</strong></div>
      <div><span>Regime</span><strong>{status.regime_name || '—'} <small>{status.regime_multiplier ? `×${Number(status.regime_multiplier).toFixed(2)}` : ''}</small></strong></div>
      <div><span>Broker env</span><strong>{status.broker_environment || '—'}</strong></div>
    </div>

    <div className="operator-timeline">
      <div><span>Last signal</span><strong>{latest ? `${latest.symbol} ${latest.signal}` : '—'}</strong><em>{formatDate(latest?.timestamp)}</em></div>
      <div><span>Last submitted bot order</span><strong>{auditOrder ? `${auditOrder.symbol || '—'} ${auditOrder.side || auditOrder.status || 'order'}` : 'No bot orders audited'}</strong><em>{formatDate(auditOrder?.created_at)}</em></div>
      <div><span>Last fill</span><strong>{auditFill ? `${auditFill.symbol || '—'} ${auditFill.qty || ''} @ ${auditFill.price || '—'}` : 'No bot fills audited'}</strong><em>{formatDate(auditFill?.created_at)}</em></div>
      <div><span>Latest broker order</span><strong>{brokerOrder ? `${brokerOrder.symbol} ${brokerOrder.side} ${brokerOrder.status}` : '—'}</strong><em>{formatDate(brokerOrder?.submitted_at || brokerOrder?.created_at)}</em></div>
    </div>
  </Card>;
}
