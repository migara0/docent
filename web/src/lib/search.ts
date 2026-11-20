import { api, type SearchDoc } from './api';
import type { SearchHit } from './types';

// Small libraries: the browser downloads one compact index and searches it itself (instant, typo tolerant).
// Large ones: the server searches the files for each query (see docs/adr/0006).

type Mini = import('minisearch').default;
interface ClientIndex {
  mode: 'client';
  mini: Mini;
  docs: Map<number, SearchDoc>;
}
interface ServerIndex {
  mode: 'server';
}

const indexes = new Map<string, Promise<ClientIndex | ServerIndex>>();

/** Forget a library's index (its pages changed); it is rebuilt on the next search. */
export function invalidateSearch(src: string) {
  indexes.delete(src);
}

async function build(src: string): Promise<ClientIndex | ServerIndex> {
  const res = await api.searchIndex(src);
  if (res.mode === 'server' || !res.docs) return { mode: 'server' };
  const { default: MiniSearch } = await import('minisearch');
  const mini = new MiniSearch<SearchDoc>({ fields: ['title', 'headings', 'text'], idField: 'id', searchOptions: { boost: { title: 8, headings: 3 }, prefix: true, fuzzy: 0.2, combineWith: 'AND' } });
  mini.addAll(res.docs);
  return { mode: 'client', mini, docs: new Map(res.docs.map((d) => [d.id, d])) };
}

/** A short passage of the page around the first query word. */
function snippet(text: string, terms: string[]): string {
  const lower = text.toLowerCase();
  let at = -1;
  for (const t of terms) {
    at = lower.indexOf(t.toLowerCase());
    if (at >= 0) break;
  }
  if (at < 0) return text.slice(0, 140).trim();
  const start = Math.max(0, at - 70);
  const out = text.slice(start, at + 90).replace(/\s+/g, ' ').trim();
  return (start > 0 ? '…' : '') + out + (at + 90 < text.length ? '…' : '');
}

export async function searchSource(src: string, query: string, signal?: AbortSignal): Promise<SearchHit[]> {
  const q = query.trim();
  if (!q) return [];
  let idx = indexes.get(src);
  if (!idx) {
    idx = build(src);
    indexes.set(src, idx);
    idx.catch(() => indexes.delete(src));
  }
  const ix = await idx;
  if (signal?.aborted) return [];
  if (ix.mode === 'server') return (await api.search(src, q, signal)).hits;
  const terms = q.split(/\s+/).filter(Boolean);
  return ix.mini.search(q).slice(0, 30).flatMap((r) => {
    const d = ix.docs.get(r.id as number);
    return d ? [{ path: d.path, url: d.url, title: d.title, folder: d.folder, score: r.score, snippet: snippet(d.text, terms) }] : [];
  });
}

/** Wraps the matching words in <mark>, escaping everything else. */
export function highlight(text: string, query: string): string {
  const esc = (s: string) => s.replace(/[&<>"]/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' })[c]!);
  const terms = query.split(/\s+/).filter(Boolean).map((t) => t.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'));
  if (!terms.length) return esc(text);
  const re = new RegExp(`(${terms.join('|')})`, 'gi');
  return text.split(re).map((part, i) => (i % 2 ? `<mark>${esc(part)}</mark>` : esc(part))).join('');
}
