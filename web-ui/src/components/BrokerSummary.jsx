import React from 'react';
import { Card } from './common.jsx';
import { money, num, pct } from '../lib/format.js';

function buildBrokerSummary(account, positions) {
  const equity = num(account?.portfolio_value ?? account?.equity);
  const rows = Array.isArray(positions) ? positions : [];
  const exposures = rows.map((p) => {
    const marketValue = Math.abs(num(p.market_value));
    return { symbol: p.symbol, qty: p.qty, marketValue,
      share: Number.isFinite(equity) && equity > 0 ? marketValue / equity : Number.NaN,
      unrealizedPlpc: p.unrealized_plpc };
  }).sort((a, b) => (b.marketValue || 0) - (a.marketValue || 0));
  return { equity, exposures, top: exposures[0] };
}

export default function BrokerSummary({ account, positions }) {
  const { equity, exposures, top } = buildBrokerSummary(account, positions);
  return <Card title="Broker Portfolio Exposure">
    <div className="metric-grid">
      <div><span>Equity</span><strong>{money(equity)}</strong></div><div><span>Cash</span><strong>{money(account?.cash)}</strong></div>
      <div className={top?.share > 0.25 ? 'warn-metric' : ''}><span>Top concentration</span><strong>{top ? `${top.symbol} ${pct(top.share)}` : '—'}</strong></div>
    </div>
    <table className="audit-table"><thead><tr><th>Symbol</th><th>Qty</th><th>Market Value</th><th>Portfolio %</th><th>Unrealized</th></tr></thead>
      <tbody>{exposures.map((p) => <tr key={p.symbol}><td><strong>{p.symbol}</strong></td><td>{p.qty}</td><td>{money(p.marketValue)}</td><td>{pct(p.share)}</td><td>{pct(p.unrealizedPlpc)}</td></tr>)}</tbody></table>
    {top?.share > 0.25 && <p className="warning">Concentration warning: {top.symbol} is above 25% of portfolio value.</p>}
  </Card>;
}
