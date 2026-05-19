import React, { useState } from 'react';
import { api } from '../../lib/api.js';

export default function LiveDataPanel({ feed, integrated }) {
  const [refreshing, setRefreshing] = useState(false), [refreshError, setRefreshError] = useState(null);
  async function refresh() {
    setRefreshing(true); setRefreshError(null);
    try { await api('/api/cartography/refresh', { method: 'POST' }); window.location.reload(); }
    catch (e) { setRefreshError(String(e.message || e)); }
    finally { setRefreshing(false); }
  }
  if (!integrated) return <div className="ec-feed ec-feed-off"><div className="ec-eyebrow">⚓ LIVE DATA OVERLAY</div><div className="ec-feed-empty">formula-only — set <code>FRED_API_KEY</code> to enable Sahm rule, yield-curve, NFCI, and high-yield-spread overrides.</div></div>;
  if (!feed) return <div className="ec-feed"><div className="ec-eyebrow">⚓ LIVE DATA OVERLAY</div><div className="ec-feed-empty">first refresh in flight…</div></div>;
  const fired = feed.signals?.filter(s => s.triggered).length || 0;
  const fetchedAgo = feed.last_fetched ? new Date(feed.last_fetched).toLocaleString() : '—';
  return <div className="ec-feed"><div className="ec-feed-head"><div><div className="ec-eyebrow">⚓ LIVE DATA OVERLAY · FRED</div><div className="ec-feed-summary">{fired === 0 ? <span className="ec-feed-clear">all signals clear · data ×{feed.multiplier.toFixed(2)}</span> : <span className="ec-feed-hot">{fired} signal{fired === 1 ? '' : 's'} triggered · data ×{feed.multiplier.toFixed(2)}</span>}</div></div><button onClick={refresh} disabled={refreshing} className="ec-feed-refresh">{refreshing ? 'refreshing…' : 'refresh'}</button></div>
    {refreshError && <div className="ec-feed-err">refresh failed: {refreshError}</div>}{feed.warnings?.length > 0 && <div className="ec-feed-warn">partial fetch — {feed.warnings.join(' · ')}</div>}
    <div className="ec-feed-grid">{(feed.signals || []).map(s => <div key={s.source} className={`ec-signal ${s.triggered ? 'on' : 'off'}`}><div className="ec-signal-head"><span className="ec-signal-name">{s.name}</span><span className={`ec-signal-status ${s.triggered ? 'on' : 'off'}`}>{s.triggered ? 'TRIGGERED' : 'CLEAR'}</span></div><div className="ec-signal-value">{s.value >= 0 ? '+' : ''}{s.value} <span className="o50">/ thr {s.threshold}</span></div><div className="ec-signal-desc">{s.description}</div><div className="ec-signal-meta">{s.source} · {new Date(s.as_of).toLocaleDateString()}</div></div>)}</div>
    <div className="ec-feed-foot">cached {fetchedAgo} · refreshes every 6h, or manually</div></div>;
}
