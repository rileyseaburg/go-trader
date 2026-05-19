import React from 'react';

export default function CartographyFooter() {
  return <footer className="ec-footer">
    <div className="ec-eyebrow">❦ MARGINALIA</div>
    <p>
      A map, not a forecast. Periods, amplitudes, and phases are illustrative constants chosen to roughly trace the shape of major historical inflections — 1929, 1973, 2008, 2020 — without curve-fitting them. Real bands drift, couple non-linearly, and answer to forcing functions the formula cannot anticipate. Use this instrument to <em>see</em> structure, not to predict it. The waveform <em>is</em> the territory; cycles are only where the waveform stores its energy. The regime multiplier shown above can govern position sizing, but it remains a risk overlay rather than a proven alpha model; live FRED inputs are displayed separately from the formula waveform.
    </p>
    <div className="ec-eyebrow ec-mute">DRAWN FROM FIRST PRINCIPLES · NO HISTORICAL DATA INGESTED · v0.1</div>
  </footer>;
}
