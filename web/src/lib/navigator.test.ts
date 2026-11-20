import { describe, expect, it } from 'vitest';
import { chainFor, childrenOf, columnsFor, libraryItems, moveWithin, nodeItems, searchResults, type Trees } from './navigator';
import type { SourceInfo, TreeNode } from './types';

const lib = (id: string, name = id): SourceInfo => ({ id, name, kind: 'library', profile: 'plain', root: '/x/' + id, pages: 3, folders: 1, updated: '', indexing: false });
const page = (path: string, title: string, id?: string): TreeNode => ({ kind: 'page', name: path.split('/').pop()!, path, title, id, url: `/a/${path.replace(/\.md$/, '')}` });
const folder = (path: string, children: TreeNode[]): TreeNode => ({ kind: 'folder', name: path.split('/').pop()!, path, children, count: children.length });

const libs = [lib('a', 'Alpha notes'), lib('b', 'Beta')];
const trees: Trees = {
  a: {
    info: libs[0]!,
    tree: {
      kind: 'folder', name: '', path: '', children: [
        folder('svc', [page('svc/one.md', 'One', 'A-0001'), folder('svc/deep', [page('svc/deep/two.md', 'Two')])]),
        page('top.md', 'Top page'),
      ],
    },
  },
};

describe('chains and columns', () => {
  it('a chain leads from the library down to the page, and has one column more than selections that can be opened', () => {
    const chain = chainFor(libs, trees, 'a', 'svc/deep/two.md');
    expect(chain.map((i) => i.kind + ':' + i.label)).toEqual(['library:Alpha notes', 'folder:svc', 'folder:deep', 'page:Two']);
    const cols = columnsFor(chain, libs, trees);
    expect(cols.map((c) => c.items.map((i) => i.label))).toEqual([['Alpha notes', 'Beta'], ['svc', 'Top page'], ['One', 'deep'], ['Two']]);
    expect(cols.map((c) => c.selected)).toEqual(['a', 'a:svc/', 'a:svc/deep/', 'a:svc/deep/two.md']);
  });
  it('selecting a folder shows its contents in a new column with nothing selected in it', () => {
    const chain = chainFor(libs, trees, 'a', 'svc/one.md').slice(0, 2); // library > svc
    const cols = columnsFor(chain, libs, trees);
    expect(cols).toHaveLength(3);
    expect(cols[2]!.items.map((i) => i.label)).toEqual(['One', 'deep']);
    expect(cols[2]!.selected).toBeUndefined();
  });
  it('a library whose tree has not loaded yet shows a loading column', () => {
    const chain = chainFor(libs, trees, 'b');
    const cols = columnsFor(chain, libs, trees);
    expect(cols[1]).toEqual({ items: [], selected: undefined, loading: true });
    expect(childrenOf(chain[0]!, trees)).toBeUndefined();
  });
  it('an unknown library or page gives a short chain, never a crash', () => {
    expect(chainFor(libs, trees, 'nope')).toEqual([]);
    expect(chainFor(libs, trees, 'a', 'svc/missing.md').map((i) => i.label)).toEqual(['Alpha notes', 'svc']);
  });
  it('items carry the address Open goes to, with each folder segment encoded', () => {
    const [svc] = nodeItems('a b', [folder('café/x y', [])]);
    expect(svc!.url).toBe('/a%20b/caf%C3%A9/x%20y/');
    expect(libraryItems(libs)[0]!.url).toBe('/a/');
  });
});

describe('moving and searching', () => {
  const items = nodeItems('a', [page('1.md', 'One'), page('2.md', 'Two'), page('3.md', 'Three')]);
  it('moves within a column and stops at its ends', () => {
    expect(moveWithin(items, items[0]!.key, 1)!.label).toBe('Two');
    expect(moveWithin(items, items[2]!.key, 1)!.label).toBe('Three');
    expect(moveWithin(items, items[0]!.key, -1)!.label).toBe('One');
    expect(moveWithin(items, undefined, 1)!.label).toBe('One');
    expect(moveWithin(items, undefined, -1)!.label).toBe('Three');
    expect(moveWithin(items, items[1]!.key, 'last')!.label).toBe('Three');
    expect(moveWithin([], undefined, 1)).toBeUndefined();
  });
  it('searching lists the matching pages of a library flat, saying where each one is', () => {
    const r = searchResults('a', trees, 'two');
    expect(r.map((i) => [i.label, i.sub])).toEqual([['Two', 'svc/deep']]);
    expect(searchResults('a', trees, 'a-0001')[0]!.sub).toBe('A-0001 · svc');
    expect(searchResults('a', trees, 'zzz')).toEqual([]);
    expect(searchResults('b', trees, 'x')).toEqual([]);
  });
});
