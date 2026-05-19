const API_BASE = import.meta.env.VITE_API_BASE || '';

export const endpoints = {
  account: '/api/account',
  positions: '/api/positions',
  orders: '/api/orders',
  runtimeStatus: '/api/runtime-status',
  tickers: '/api/tickers',
  signals: '/api/signals',
  risk: '/api/risk-parameters',
  notifications: '/api/notifications',
  auditSummary: '/api/audit/summary',
  auditSignals: '/api/audit/signals?limit=10',
  auditDecisions: '/api/audit/decisions?limit=10',
  auditOrders: '/api/audit/orders?limit=10',
  auditFills: '/api/audit/fills?limit=10',
  auditRegimes: '/api/audit/regime_states?limit=10',
};

export async function api(path, options) {
  const res = await fetch(`${API_BASE}${path}`, {
    headers: { 'Content-Type': 'application/json', ...(options?.headers || {}) },
    ...options,
  });
  const text = await res.text();
  let body = null;
  try { body = text ? JSON.parse(text) : null; } catch { body = text; }
  if (!res.ok) throw new Error(typeof body === 'string' ? body : body?.error || res.statusText);
  return body;
}
