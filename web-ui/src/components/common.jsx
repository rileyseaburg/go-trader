import React from 'react';

export function Card({ title, children }) {
  return <section className="card"><h2>{title}</h2>{children}</section>;
}

export function JsonBlock({ value }) {
  return <pre className="json">{JSON.stringify(value, null, 2)}</pre>;
}
