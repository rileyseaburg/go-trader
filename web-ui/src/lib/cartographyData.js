import { api } from './api.js';

export const RECESSIONS = [
  { start: 1929.7, end: 1933.3, label: 'GREAT DEPRESSION' },
  { start: 1937.4, end: 1938.5, label: '' },
  { start: 1973.9, end: 1975.2, label: 'OIL SHOCK' },
  { start: 1980.0, end: 1980.5, label: '' },
  { start: 1981.5, end: 1982.9, label: 'VOLCKER' },
  { start: 1990.5, end: 1991.2, label: 'S&L' },
  { start: 2001.2, end: 2001.9, label: 'DOTCOM' },
  { start: 2007.9, end: 2009.5, label: 'GFC' },
  { start: 2020.1, end: 2020.4, label: 'COVID' },
];

export const PRESETS = [1929, 1973, 2000, 2008, 2020]
  .map(year => ({ year, label: String(year) }))
  .concat([{ year: new Date().getFullYear(), label: 'NOW' }, { year: 2035, label: '2035' }]);

export const MONTH_NAMES = ['JAN','FEB','MAR','APR','MAY','JUN','JUL','AUG','SEP','OCT','NOV','DEC'];

export function yearToISO(year) {
  const y = Math.floor(year);
  const monthIdx = Math.max(0, Math.min(11, Math.floor((year - y) * 12)));
  return `${y}-${String(monthIdx + 1).padStart(2, '0')}-01`;
}

export function yearParts(year) {
  const yearInt = Math.floor(year);
  const monthIdx = Math.max(0, Math.min(11, Math.floor((year - yearInt) * 12)));
  return { yearInt, month: MONTH_NAMES[monthIdx] };
}

export function currentYearFraction() {
  return new Date().getFullYear() + new Date().getMonth() / 12;
}

export function fetchReading(year, includeSeries = false) {
  const params = new URLSearchParams();
  if (includeSeries) params.set('series', 'true');
  if (year != null) params.set('at', yearToISO(year));
  return api(`/api/cartography?${params.toString()}`);
}
