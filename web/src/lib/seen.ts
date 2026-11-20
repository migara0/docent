// Which notes you have not looked at yet. Note IDs only go up and are never reused, so one number per library is enough to say "everything up to here was there
// when you started" (base); the notes above it that you have opened since are listed in `opened`. Nothing is stored on the server: this is per browser.

import type { LatestNote } from './types';

export interface SeenRecord {
  base: number;
  opened: number[];
}
export type SeenStore = Record<string, SeenRecord>;

/** The number of a note ID: "DOC-0034" is 34. */
export function idNum(id: string | undefined): number {
  const m = /(\d+)\s*$/.exec(id ?? '');
  return m?.[1] ? parseInt(m[1], 10) : 0;
}

export const isUnseen = (rec: SeenRecord | undefined, num: number): boolean => !!rec && num > rec.base && !rec.opened.includes(num);

/** Opening a note marks it seen. When the notes just above the base are all seen, the base moves up, so the list stays short. */
export function markOpened(rec: SeenRecord, num: number): SeenRecord {
  if (!isUnseen(rec, num)) return rec;
  const opened = [...rec.opened, num].sort((a, b) => a - b);
  let base = rec.base;
  while (opened[0] === base + 1) {
    base++;
    opened.shift();
  }
  return { base, opened };
}

export const markAllSeen = (maxId: number): SeenRecord => ({ base: maxId, opened: [] });

/** Reads what was stored, trusting nothing (it may be from an older version, or edited by hand). */
export function parseStore(raw: string | null): SeenStore {
  const out: SeenStore = {};
  try {
    const v = JSON.parse(raw ?? '{}') as Record<string, { base?: unknown; opened?: unknown }>;
    for (const [k, r] of Object.entries(v ?? {})) {
      if (!r || typeof r.base !== 'number' || !Number.isFinite(r.base) || r.base < 0) continue;
      const base = Math.floor(r.base);
      const opened = Array.isArray(r.opened) ? r.opened.filter((n): n is number => typeof n === 'number' && Number.isInteger(n) && n > base).slice(0, 500) : [];
      out[k] = { base, opened };
    }
  } catch {
    /* unreadable: start again */
  }
  return out;
}

/** Today as YYYY-MM-DD in this browser's time zone. */
export function today(now = new Date()): string {
  const p = (n: number) => String(n).padStart(2, '0');
  return `${now.getFullYear()}-${p(now.getMonth() + 1)}-${p(now.getDate())}`;
}

/** Whether a YYYY-MM-DD date is within the last `days` days (today counts). */
export function isRecent(created: string | undefined, days: number, now = new Date()): boolean {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(created ?? '');
  if (!m) return false;
  const [, y = '0', mo = '1', d = '1'] = m;
  const then = new Date(Number(y), Number(mo) - 1, Number(d)).getTime();
  const start = new Date(now.getFullYear(), now.getMonth(), now.getDate()).getTime();
  return then >= start - (days - 1) * 86_400_000;
}

export interface NewNote extends LatestNote {
  source: string;
  library: string;
  unseen: boolean;
}

/** Newest first: by creation date, then by ID. */
export const byNewest = (a: { created?: string; num: number }, b: { created?: string; num: number }): number =>
  (b.created ?? '').localeCompare(a.created ?? '') || b.num - a.num;

/** The notes worth showing as new: the ones not seen yet, and the ones made in the last `days` days. */
export function whatsNew(sources: { source: string; name: string; notes: LatestNote[] }[], store: SeenStore, days: number, limit: number, now = new Date()): NewNote[] {
  const out: NewNote[] = [];
  for (const s of sources) {
    for (const n of s.notes) {
      const unseen = isUnseen(store[s.source], n.num);
      if (unseen || isRecent(n.created, days, now)) out.push({ ...n, source: s.source, library: s.name, unseen });
    }
  }
  return out.sort(byNewest).slice(0, limit);
}

/** The notes that are in `now` and were not in `before`, and that are still unseen: what to tell the reader about. */
export function appeared(before: Set<string> | null, sources: { source: string; notes: LatestNote[] }[], store: SeenStore): { source: string; note: LatestNote }[] {
  if (!before) return [];
  const out: { source: string; note: LatestNote }[] = [];
  for (const s of sources) for (const n of s.notes) if (!before.has(`${s.source}:${n.id}`) && isUnseen(store[s.source], n.num)) out.push({ source: s.source, note: n });
  return out.sort((a, b) => byNewest(a.note, b.note));
}
