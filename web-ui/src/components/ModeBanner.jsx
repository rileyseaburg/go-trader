import React from 'react';

export default function ModeBanner({ status }) {
  const mode = status?.trading_mode || 'UNKNOWN';
  const livePaper = mode === 'LIVE_PAPER';
  const dry = mode === 'DRY_RUN' || mode === 'MOCK';
  return <div className={`mode-banner ${livePaper ? 'live' : dry ? 'dry' : 'unknown'}`}>
    <div>
      <span className="mode-label">{livePaper ? 'LIVE PAPER TRADING' : dry ? mode.replace('_', ' ') : 'TRADING MODE UNKNOWN'}</span>
      <strong>{status?.order_submission_enabled ? 'Broker order submission enabled' : 'No real broker submissions'}</strong>
    </div>
    <div className="mode-meta">
      <span>broker: {status?.broker || '—'}</span>
      <span>environment: {status?.broker_environment || '—'}</span>
      <span>algo: {status?.algorithm_running ? 'running' : 'stopped/unknown'}</span>
    </div>
  </div>;
}
