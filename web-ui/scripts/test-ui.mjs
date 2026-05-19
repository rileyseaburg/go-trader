import { existsSync, readFileSync } from 'node:fs';
const required = ['package.json', 'index.html', 'src/main.jsx', 'src/styles.css'];
const missing = required.filter((file) => !existsSync(new URL(`../${file}`, import.meta.url)));
if (missing.length) throw new Error(`Missing UI files: ${missing.join(', ')}`);
const pkg = JSON.parse(readFileSync(new URL('../package.json', import.meta.url), 'utf8'));
for (const script of ['dev', 'test:api', 'test:ui', 'test:all', 'cypress']) {
  if (!pkg.scripts?.[script]) throw new Error(`Missing npm script: ${script}`);
}
console.log('UI smoke test passed');
