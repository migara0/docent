// The logic behind the Browse popup: Finder-style columns (libraries, then folders, then pages). Pure functions, no DOM.

import { folderUrl } from './urls';
import { filterTree } from './treenav';
import type { SourceInfo, TreeNode, TreeResponse } from './types';

export type ItemKind = 'library' | 'folder' | 'page';

export interface NavItem {
  kind: ItemKind;
  key: string; // unique across everything shown
  lib: string; // the library's id
  label: string;
  path: string; // folder or page path inside the library ('' for a library)
  url: string; // where Open goes
  sub?: string; // the note ID of a page, or where a search result lives
  count?: number; // pages inside a folder or library
  node?: TreeNode;
}

export type Trees = Record<string, TreeResponse | undefined>;

export function libraryItems(libs: SourceInfo[]): NavItem[] {
  return libs.map((l) => ({ kind: 'library', key: l.id, lib: l.id, label: l.name, path: '', url: `/${encodeURIComponent(l.id)}/`, count: l.pages }));
}

export function nodeItems(lib: string, nodes: TreeNode[]): NavItem[] {
  return nodes.map((n) =>
    n.kind === 'folder'
      ? { kind: 'folder', key: `${lib}:${n.path}/`, lib, label: n.name, path: n.path, url: folderUrl(lib, n.path), count: n.count, node: n }
      : { kind: 'page', key: `${lib}:${n.path}`, lib, label: n.title || n.name, path: n.path, url: n.url ?? '', sub: n.id, node: n },
  );
}

/** What is inside a library or folder: undefined while the library's tree is still loading. */
export function childrenOf(item: NavItem, trees: Trees): NavItem[] | undefined {
  if (item.kind === 'page') return [];
  if (item.kind === 'library') {
    const t = trees[item.lib];
    return t ? nodeItems(item.lib, t.tree.children ?? []) : undefined;
  }
  return nodeItems(item.lib, item.node?.children ?? []);
}

export interface Column {
  items: NavItem[];
  selected?: string; // the key of the selected item in this column
  loading: boolean;
}

/** The columns to show for a chain of selections (one selected item per column). A folder or library at the end of the chain adds a column with its contents. */
export function columnsFor(chain: NavItem[], libs: SourceInfo[], trees: Trees): Column[] {
  const cols: Column[] = [{ items: libraryItems(libs), selected: chain[0]?.key, loading: false }];
  for (let i = 0; i < chain.length; i++) {
    const it = chain[i]!;
    if (it.kind === 'page') break;
    const kids = childrenOf(it, trees);
    cols.push({ items: kids ?? [], selected: chain[i + 1]?.key, loading: kids === undefined });
  }
  return cols;
}

/** The chain that leads to a page (or to a library, when there is no page): libraries > library > folders > page. Empty when the library is not known. */
export function chainFor(libs: SourceInfo[], trees: Trees, lib: string, pagePath = ''): NavItem[] {
  const l = libraryItems(libs).find((i) => i.lib === lib);
  if (!l) return [];
  const chain: NavItem[] = [l];
  const t = trees[lib];
  if (!t || !pagePath) return chain;
  let nodes = t.tree.children ?? [];
  const segs = pagePath.split('/');
  for (let i = 0; i < segs.length; i++) {
    const path = segs.slice(0, i + 1).join('/');
    const node = nodes.find((n) => n.path === path);
    if (!node) break;
    chain.push(nodeItems(lib, [node])[0]!);
    if (node.kind === 'page') break;
    nodes = node.children ?? [];
  }
  return chain;
}

/** The item a key press moves to within a column. */
export function moveWithin(items: NavItem[], currentKey: string | undefined, to: number | 'first' | 'last'): NavItem | undefined {
  if (!items.length) return undefined;
  const i = items.findIndex((x) => x.key === currentKey);
  if (to === 'first') return items[0];
  if (to === 'last') return items[items.length - 1];
  if (i < 0) return items[to > 0 ? 0 : items.length - 1];
  return items[Math.min(items.length - 1, Math.max(0, i + to))];
}

/** Pages of a library that match what was typed, flat, each saying which folder it is in. */
export function searchResults(lib: string, trees: Trees, query: string, limit = 200): NavItem[] {
  const t = trees[lib];
  if (!t) return [];
  const out: NavItem[] = [];
  const walk = (nodes: TreeNode[]) => {
    for (const n of nodes) {
      if (out.length >= limit) return;
      if (n.kind === 'page') {
        const it = nodeItems(lib, [n])[0]!;
        const dir = n.path.includes('/') ? n.path.slice(0, n.path.lastIndexOf('/')) : '';
        it.sub = [n.id, dir].filter(Boolean).join(' · ');
        out.push(it);
      } else walk(n.children ?? []);
    }
  };
  walk(filterTree(t.tree.children ?? [], query).nodes);
  return out;
}

/** The first blocks of a rendered page, for the preview (the HTML is already sanitised by the server; this only cuts it down). */
export function excerptHtml(html: string, maxChars = 1400): string {
  const doc = new DOMParser().parseFromString(html, 'text/html');
  const out: string[] = [];
  let chars = 0;
  for (const el of [...doc.body.children]) {
    if (el.matches('h1, .anchor, button, figure.diagram, pre.mermaid')) continue;
    out.push(el.outerHTML);
    chars += el.textContent?.length ?? 0;
    if (chars >= maxChars || out.length >= 8) break;
  }
  return out.join('');
}
