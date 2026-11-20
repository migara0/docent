// A tiny history-API router. The app's URLs are /<library>/<page path without .md>, /<library>/<folder>/ and / (home).

/** Decodes every segment, so "/a/caf%C3%A9" and "/a/café" compare equal. */
export function normalizePath(p: string): string {
  return p
    .split('/')
    .map((s) => {
      try {
        return decodeURIComponent(s);
      } catch {
        return s;
      }
    })
    .join('/');
}

export type Route = { kind: 'home' } | { kind: 'library'; src: string; rest: string; folder: boolean };

export function parseRoute(pathname: string): Route {
  const norm = normalizePath(pathname);
  if (norm === '/' || norm === '') return { kind: 'home' };
  const segs = norm.replace(/^\/+/, '').split('/');
  const src = segs.shift() ?? '';
  const folder = norm.endsWith('/');
  const rest = segs.filter(Boolean).join('/');
  return { kind: 'library', src, rest, folder: folder || rest === '' };
}

/** The URL of a folder, with a trailing slash (kept in urls.ts, which has no browser dependencies). */
export { folderUrl } from './urls';

class Router {
  path = $state(location.pathname);
  hash = $state(location.hash);

  constructor() {
    addEventListener('popstate', () => this.sync());
  }

  private sync() {
    this.path = location.pathname;
    this.hash = location.hash;
  }

  get route(): Route {
    return parseRoute(this.path);
  }

  /** Navigates inside the app. Returns false for addresses that belong to the browser (other sites, API, files). */
  go(to: string, opts: { replace?: boolean } = {}): boolean {
    const url = new URL(to, location.href);
    if (url.origin !== location.origin || url.pathname.startsWith('/_api/')) return false;
    const next = url.pathname + url.search + url.hash;
    if (next !== location.pathname + location.search + location.hash) {
      (opts.replace ? history.replaceState : history.pushState).call(history, null, '', next);
    }
    this.sync();
    return true;
  }
}

export const router = new Router();
