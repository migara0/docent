// Navigation helpers for the page tree: shorter labels, filtering, and highlighting. Pure functions, no DOM.

import type { TreeNode } from './types';

const STOP = new Set(['the', 'a', 'an', 'how', 'to', 'why', 'what', 'of', 'and', 'for', 'in', 'on', 'with', 'about']);
const stripEnd = (w: string) => w.replace(/[:;,.\-–—]+$/u, '').toLowerCase();

/**
 * Titles that start with the same words ("Payments block 1: foundation", "Payments block 5: users") are cut after what they share, so the part that tells them apart is
 * what the narrow column shows. Only display: the title itself is not changed (it is the tooltip). At least three pages have to share a start, an index page is left out,
 * and a number that follows the shared words stays with the label ("Block 1: foundation", not "1: foundation").
 */
export function elideLabels(children: TreeNode[]): Record<string, string> {
  const pages = children.filter((c) => c.kind === 'page' && !/^index\.(md|markdown)$/i.test(c.name));
  if (pages.length < 3) return {};
  const words = pages.map((p) => (p.title || p.name).trim().split(/\s+/));
  const room = Math.min(...words.map((w) => w.length)) - 1; // at least one word is always left
  let k = 0;
  while (k < room && words.every((w) => stripEnd(w[k]!) === stripEnd(words[0]![k]!))) k++;
  if (k > 0 && words.some((w) => /^\d/.test(w[k]!))) k--;
  if (k === 0 || words[0]!.slice(0, k).every((w) => STOP.has(stripEnd(w)))) return {};
  const out: Record<string, string> = {};
  pages.forEach((p, i) => {
    const rest = words[i]!.slice(k).join(' ').replace(/^[:;,.\-–—\s]+/u, '');
    out[p.path] = rest.charAt(0).toUpperCase() + rest.slice(1);
  });
  return out;
}

export interface Filtered {
  nodes: TreeNode[];
  matches: number;
  total: number;
}

/** A word of three or more characters matches anywhere ("ment" finds "payments"); one or two characters only at the start of a word, so "5" finds "block 5" and not "2026-10-15". */
function termMatches(hay: string, term: string): boolean {
  if (term.length >= 3) return hay.includes(term);
  return new RegExp(`(^|[^a-z0-9])${term.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}`).test(hay);
}

export const termsOf = (query: string) => query.toLowerCase().split(/\s+/).filter(Boolean);

function countPages(nodes: TreeNode[]): number {
  return nodes.reduce((n, c) => n + (c.kind === 'page' ? 1 : countPages(c.children ?? [])), 0);
}

/**
 * The tree with only the pages that match every word typed. A word can match the page's title, its path (so folder names count: "payments block 5") or its note ID.
 * Folders stay only if something inside matches.
 */
export function filterTree(children: TreeNode[], query: string): Filtered {
  const terms = termsOf(query);
  let matches = 0;
  const walk = (nodes: TreeNode[]): TreeNode[] => {
    const out: TreeNode[] = [];
    for (const n of nodes) {
      if (n.kind === 'page') {
        const hay = `${n.path} ${n.title ?? ''} ${n.id ?? ''}`.toLowerCase();
        if (terms.every((t) => termMatches(hay, t))) {
          out.push(n);
          matches++;
        }
      } else {
        const kept = walk(n.children ?? []);
        if (kept.length) out.push({ ...n, children: kept, count: countPages(kept) });
      }
    }
    return out;
  };
  return { nodes: terms.length ? walk(children) : children, matches: terms.length ? matches : countPages(children), total: countPages(children) };
}

export interface Part {
  text: string;
  hit: boolean;
}

/** Splits text so the typed words can be marked. */
export function highlightParts(text: string, terms: string[]): Part[] {
  if (!terms.length) return [{ text, hit: false }];
  const re = new RegExp(`(${terms.map((t) => t.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')).join('|')})`, 'gi');
  // split with a capturing group alternates: text, match, text, match ...
  return text
    .split(re)
    .map((t, i) => ({ text: t, hit: i % 2 === 1 }))
    .filter((p) => p.text !== '');
}

/**
 * Long titles are cut in the middle instead of at the end: the start (which says what it is about) and the last word or two (which is often what tells siblings apart:
 * "Integration Phase 1 ... decisions" and "Integration Phase 1 ... questions") both stay visible. The tail is at most about 16 characters; short titles are not split.
 */
export function splitTail(text: string): { head: string; tail: string } {
  if (text.length <= 22) return { head: text, tail: '' };
  const words = text.split(' ');
  let tail = '';
  while (words.length > 2 && (tail === '' || (tail.length < 8 && (words[words.length - 1]!.length + tail.length + 1) <= 18))) {
    tail = words.pop()! + (tail ? ' ' + tail : '');
  }
  return { head: words.join(' '), tail: tail ? ' ' + tail : '' };
}
