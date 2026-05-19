import React, { useEffect, useState } from 'react';
import { createRoot } from 'react-dom/client';
import './styles.css';
import './cartography.css';
import ApiStatus from './components/ApiStatus.jsx';
import BrokerSummary from './components/BrokerSummary.jsx';
import DebugView from './components/DebugView.jsx';
import OperatorStatus from './components/OperatorStatus.jsx';
import StrategyView from './components/StrategyView.jsx';
import { api, endpoints } from './lib/api.js';

function App() {
  const [data, setData] = useState({});
  const [errors, setErrors] = useState({});
  const [loading, setLoading] = useState(false);
  const [symbolsText, setSymbolsText] = useState('AAPL,MSFT,TSLA');
  const [riskText, setRiskText] = useState('{}');

  async function loadAll() {
    setLoading(true);
    const nextData = {}, nextErrors = {};
    await Promise.all(Object.entries(endpoints).map(async ([key, path]) => {
      try { nextData[key] = await api(path); } catch (err) { nextErrors[key] = err.message; }
    }));
    setData(nextData); setErrors(nextErrors);
    if (nextData.tickers?.symbols) setSymbolsText(nextData.tickers.symbols.join(','));
    if (nextData.risk) setRiskText(JSON.stringify(nextData.risk, null, 2));
    setLoading(false);
  }

  async function updateTickers(event) {
    event.preventDefault();
    const symbols = symbolsText.split(',').map(s => s.trim().toUpperCase()).filter(Boolean);
    await api('/api/tickers', { method: 'POST', body: JSON.stringify({ symbols }) });
    await loadAll();
  }

  async function updateRisk(event) {
    event.preventDefault();
    await api('/api/risk-parameters', { method: 'POST', body: riskText });
    await loadAll();
  }

  useEffect(() => { loadAll(); }, []);

  return <main className="shell">
    <header className="hero">
      <div><h1>Go Trader</h1><p>Trading cockpit: what the bot believes, what it may do, what it did, and what could go wrong.</p></div>
      <button onClick={loadAll} disabled={loading}>{loading ? 'Refreshing…' : 'Refresh'}</button>
    </header>
    <ApiStatus errors={errors} />

    <section className="tier operator-tier">
      <div className="tier-heading">
        <span>Tier 1</span>
        <h2>Operator view</h2>
        <p>Immediate answer to “is it live, connected, and trading?” plus portfolio exposure.</p>
      </div>
      <OperatorStatus data={data} />
      <BrokerSummary account={data.account || {}} positions={data.positions || []} />
    </section>

    <StrategyView data={data} />

    <DebugView
      data={data}
      symbolsText={symbolsText}
      riskText={riskText}
      onSymbolsChange={setSymbolsText}
      onRiskChange={setRiskText}
      onTickersSubmit={updateTickers}
      onRiskSubmit={updateRisk}
    />
  </main>;
}

createRoot(document.getElementById('root')).render(<App />);
