import React from 'react';
import { Card } from './common.jsx';

export default function ApiStatus({ errors }) {
  if (Object.keys(errors).length === 0) return null;
  return <Card title="API Status">
    <ul className="errors">{Object.entries(errors).map(([k, v]) => <li key={k}><strong>{k}</strong>: {v}</li>)}</ul>
    <p className="hint">Start the backend with <code>go run main.go -mock</code> for local development without Alpaca credentials.</p>
  </Card>;
}
