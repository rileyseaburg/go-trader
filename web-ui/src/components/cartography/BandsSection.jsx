import React from 'react';
import BandRow from './BandRow.jsx';
import PhaseWheel from './PhaseWheel.jsx';

export default function BandsSection({ reading }) {
  return <div className="ec-cols">
    <div className="ec-col-wheel"><div className="ec-eyebrow ec-center">✧ THE PHASE COMPASS</div><PhaseWheel bands={reading.bands} composite={reading.composite} /></div>
    <div className="ec-col-bands"><div className="ec-eyebrow">⌁ THE BANDS</div>{reading.bands.map(br => <BandRow key={br.band.key} br={br} />)}</div>
  </div>;
}
