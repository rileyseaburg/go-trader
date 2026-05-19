import React from 'react';
import CartographyPanel from '../cartography.jsx';
import SignalSummary from './SignalSummary.jsx';
import { Card } from './common.jsx';

export default function StrategyView({ data }) {
  return <section className="tier strategy-tier">
    <div className="tier-heading">
      <span>Tier 2</span>
      <h2>Strategy view</h2>
      <p>What the bot believes: current signals, arbitrator reasoning, strategy votes, and macro regime.</p>
    </div>
    <SignalSummary signals={data.signals} />
    <Card title="Macro Regime">
      <p className="hint">Regime is shown for context and position-sizing visibility. Treat it as a capital-governance input, not proof of predictive power.</p>
      <CartographyPanel compact />
    </Card>
  </section>;
}
