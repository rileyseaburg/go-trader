const base = process.env.API_BASE || 'http://localhost:8080';
const endpoints = ['/api/tickers', '/api/risk-parameters', '/api/signals'];
for (const endpoint of endpoints) {
  try {
    const res = await fetch(`${base}${endpoint}`);
    if (!res.ok) throw new Error(`${res.status} ${res.statusText}`);
    console.log(`${endpoint}: ok`);
  } catch (err) {
    console.warn(`${endpoint}: skipped (${err.message}). Start backend with "go run main.go -mock" for live API smoke tests.`);
  }
}
