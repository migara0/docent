import { describe, expect, test } from 'vitest';

// router.svelte.ts touches `location` and `history` at import time, so the pure helpers are tested through a copy of their logic's contract
// by importing them lazily with a minimal stub of the browser globals.
async function load() {
  const g = globalThis as Record<string, unknown>;
  g.location = { pathname: '/', hash: '', href: 'http://x/', search: '', origin: 'http://x' };
  g.history = { pushState() {}, replaceState() {} };
  g.addEventListener = () => {};
  return import('./router.svelte');
}

describe('routes', () => {
  test('home, library, page and folder', async () => {
    const { parseRoute } = await load();
    expect(parseRoute('/')).toEqual({ kind: 'home' });
    expect(parseRoute('/notes/')).toEqual({ kind: 'library', src: 'notes', rest: '', folder: true });
    expect(parseRoute('/notes/guide/syntax')).toEqual({ kind: 'library', src: 'notes', rest: 'guide/syntax', folder: false });
    expect(parseRoute('/notes/guide/')).toEqual({ kind: 'library', src: 'notes', rest: 'guide', folder: true });
  });

  test('percent-encoding is normalised', async () => {
    const { parseRoute, normalizePath } = await load();
    expect(normalizePath('/a/caf%C3%A9')).toBe('/a/café');
    expect(parseRoute('/lib/2026-10-14-plan%20b')).toMatchObject({ rest: '2026-10-14-plan b' });
    expect(normalizePath('/bad/%E0%A4%A')).toBe('/bad/%E0%A4%A'); // an invalid escape is left alone, not thrown on
  });

  test('folder urls end with a slash and encode each segment', async () => {
    const { folderUrl } = await load();
    expect(folderUrl('my lib', '')).toBe('/my%20lib/');
    expect(folderUrl('lib', 'a b/c')).toBe('/lib/a%20b/c/');
  });
});
