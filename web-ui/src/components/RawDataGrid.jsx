import React from 'react';
import { Card, JsonBlock } from './common.jsx';

function RawDetails({ title, children, open = false }) {
  return <details className="raw-details" open={open}><summary>{title}</summary>{children}</details>;
}

export default function RawDataGrid({ data, symbolsText, riskText, onSymbolsChange, onRiskChange, onTickersSubmit, onRiskSubmit }) {
  return <Card title="Raw Data">
    <p className="hint">Raw JSON is available for debugging, but the operator and strategy tiers above are the primary UI.</p>
    <div className="raw-grid">
      <RawDetails title="Broker Account"><JsonBlock value={data.account || {}} /></RawDetails>
      <RawDetails title="Broker Positions"><JsonBlock value={data.positions || []} /></RawDetails>
      <RawDetails title="Broker Orders"><JsonBlock value={data.orders || []} /></RawDetails>
      <RawDetails title="Raw Signals"><JsonBlock value={data.signals || {}} /></RawDetails>
      <RawDetails title="Notifications"><JsonBlock value={data.notifications || []} /></RawDetails>
      <RawDetails title="Tickers + controls" open><form onSubmit={onTickersSubmit} className="stack">
        <label>Symbols, comma separated</label><input value={symbolsText} onChange={e => onSymbolsChange(e.target.value)} placeholder="AAPL,MSFT,TSLA" />
        <button type="submit">Update tickers</button></form><JsonBlock value={data.tickers || {}} /></RawDetails>
      <RawDetails title="Risk Parameters" open><form onSubmit={onRiskSubmit} className="stack">
        <label>JSON risk parameter patch</label><textarea value={riskText} onChange={e => onRiskChange(e.target.value)} rows={10} />
        <button type="submit">Save risk parameters</button></form></RawDetails>
    </div>
  </Card>;
}
