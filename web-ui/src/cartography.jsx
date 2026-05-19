import React, { useEffect, useState } from 'react';
import BandsSection from './components/cartography/BandsSection.jsx';
import CartographyFooter from './components/cartography/CartographyFooter.jsx';
import CartographyHeader from './components/cartography/CartographyHeader.jsx';
import ChartSection from './components/cartography/ChartSection.jsx';
import HerePanel from './components/cartography/HerePanel.jsx';
import LiveDataPanel from './components/cartography/LiveDataPanel.jsx';
import TimeNavigator from './components/cartography/TimeNavigator.jsx';
import { currentYearFraction, fetchReading } from './lib/cartographyData.js';

export default function CartographyPanel({ compact = false }) {
  const nowYear = currentYearFraction();
  const [year, setYear] = useState(nowYear);
  const [data, setData] = useState(null);
  const [series, setSeries] = useState(null);
  const [error, setError] = useState(null);

  useEffect(() => {
    fetchReading(nowYear, true).then(d => { setSeries(d.series); setData(d); }).catch(e => setError(String(e)));
  }, []);

  useEffect(() => {
    if (series == null) return;
    fetchReading(year).then(d => setData(prev => ({ ...prev, ...d }))).catch(e => setError(String(e)));
  }, [year, series]);

  if (error) return <div className="ec-error">cartography offline — {error}</div>;
  if (!data) return <div className="ec-loading">drawing the chart…</div>;

  return <section className={`ec-root ${compact ? 'ec-compact' : ''}`}>
    {!compact && <CartographyHeader />}
    <ChartSection series={series} currentYear={data.reading.year} />
    <HerePanel reading={data.reading} appliedMultiplier={data.applied_multiplier} />
    <LiveDataPanel feed={data.feed} integrated={Boolean(data.feed)} />
    {!compact && <BandsSection reading={data.reading} />}
    {!compact && <TimeNavigator year={year} onChange={setYear} />}
    {!compact && <CartographyFooter />}
  </section>;
}
