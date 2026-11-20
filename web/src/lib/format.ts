const RTF = typeof Intl !== 'undefined' ? new Intl.RelativeTimeFormat('en', { numeric: 'auto' }) : undefined;

/** "today", "yesterday", "3 days ago", or a date for older items. */
export function relativeTime(iso: string | undefined, now = new Date()): string {
  if (!iso) return '';
  const t = Date.parse(iso);
  if (Number.isNaN(t)) return '';
  // Calendar days in this browser's time zone. A date without a time ("2026-11-11") means that day here, not midnight in London:
  // counting hours from it said "yesterday" for a note changed today, from about midday onwards.
  const day = /^(\d{4})-(\d{2})-(\d{2})$/.exec(iso);
  const then = day ? new Date(Number(day[1]), Number(day[2]) - 1, Number(day[3])) : new Date(t);
  const startOf = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();
  const days = Math.round((startOf(then) - startOf(now)) / 86_400_000);
  if (!RTF) return then.toLocaleDateString();
  if (days >= 0) return 'today';
  if (days > -14) return RTF.format(days, 'day');
  if (days > -60) return RTF.format(Math.round(days / 7), 'week');
  return then.toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });
}

const pad = (n: number) => String(n).padStart(2, '0');

/** The calendar day of a moment in this browser's time zone, as YYYY-MM-DD. */
export function localDay(d: Date): string {
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
}

/** The creation day of a page in this browser's time zone: from its moment when known, else the day of its front matter. */
export function createdDay(row: { created?: string; createdAt?: string }): string {
  const t = row.createdAt ? new Date(row.createdAt) : undefined;
  return t && !Number.isNaN(t.getTime()) ? localDay(t) : (row.created ?? '');
}

/**
 * A created or updated date for display: "2026-11-17 21:52" in this browser's time zone when the moment is known, else the day alone.
 * `title` says the full time and the time zone, or that no time was recorded.
 */
export function stamp(day: string | undefined, at?: string): { text: string; title: string } {
  const t = at ? new Date(at) : undefined;
  if (!t || Number.isNaN(t.getTime())) return { text: day ?? '', title: day ? `${day} (the note records the day, not the time)` : '' };
  const zone = Intl.DateTimeFormat().resolvedOptions().timeZone;
  return { text: `${localDay(t)} ${pad(t.getHours())}:${pad(t.getMinutes())}`, title: `${localDay(t)} ${pad(t.getHours())}:${pad(t.getMinutes())}:${pad(t.getSeconds())} (${zone})` };
}

export const readMinutes = (words: number) => Math.max(1, Math.round(words / 220));

/** Two letters for a library's badge: "ops-notes" gives "ON", "Team wiki" gives "TW". */
export function initials(name: string): string {
  const parts = name.split(/[\s_\-./]+/).filter(Boolean);
  const letters = parts.length > 1 ? (parts[0]![0]! + parts[1]![0]!) : name.slice(0, 2);
  return letters.toUpperCase();
}

export const str = (v: unknown): string => (typeof v === 'string' ? v : typeof v === 'number' ? String(v) : '');

export function download(filename: string, text: string, type = 'text/markdown;charset=utf-8') {
  const url = URL.createObjectURL(new Blob([text], { type }));
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.append(a);
  a.click();
  a.remove();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

export async function copyText(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text);
    return true;
  } catch {
    const area = document.createElement('textarea');
    area.value = text;
    area.style.cssText = 'position:fixed;opacity:0';
    document.body.append(area);
    area.select();
    const ok = document.execCommand('copy');
    area.remove();
    return ok;
  }
}
