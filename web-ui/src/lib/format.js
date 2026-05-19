export function num(value) {
  if (typeof value === 'number') return value;
  if (typeof value === 'string') return Number.parseFloat(value.replaceAll(',', ''));
  return Number.NaN;
}

export function money(value) {
  const n = num(value);
  if (!Number.isFinite(n)) return '—';
  return n.toLocaleString(undefined, { style: 'currency', currency: 'USD', maximumFractionDigits: 0 });
}

export function pct(value) {
  const n = num(value);
  if (!Number.isFinite(n)) return '—';
  return `${(n * 100).toFixed(1)}%`;
}

export function confidencePct(value) {
  const n = num(value);
  if (!Number.isFinite(n)) return '—';
  return `${(n * 100).toFixed(0)}%`;
}

export function formatDate(value) {
  if (!value) return '—';
  const d = new Date(value);
  return Number.isNaN(d.getTime()) ? String(value) : d.toLocaleString();
}
