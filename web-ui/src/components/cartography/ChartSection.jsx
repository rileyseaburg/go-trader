import React from 'react';
import CompositeChart from './CompositeChart.jsx';

export default function ChartSection({ series, currentYear }) {
  return <div className="ec-chart-wrap">
    <div className="ec-eyebrow ec-pad">⌁ THE COMPOSITE WAVEFORM · 1925 ─ 2040</div>
    <CompositeChart series={series} currentYear={currentYear} />
    <div className="ec-legend">
      <span><i style={{ background: '#c2410c' }} />KONDRATIEV</span><span><i style={{ background: '#a16207' }} />KUZNETS</span>
      <span><i style={{ background: '#0e7490' }} />JUGLAR</span><span><i style={{ background: '#4d7c0f' }} />KITCHIN</span>
      <span><i style={{ background: '#e8d4a8' }} />COMPOSITE</span><span><i style={{ background: '#7f1d1d', opacity: 0.6 }} />RECESSION</span>
    </div>
  </div>;
}
