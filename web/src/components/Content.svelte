<script lang="ts">
  import Topbar from './Topbar.svelte';
  import Home from './Home.svelte';
  import PageView from './PageView.svelte';
  import FolderView from './FolderView.svelte';
  import TagsView from './TagsView.svelte';
  import RecentNotes from './RecentNotes.svelte';
  import { app } from '../lib/app.svelte';
  import { api, ApiError } from '../lib/api';
  import { folderUrl, normalizePath, router } from '../lib/router.svelte';
  import { recent } from '../lib/recent.svelte';
  import { setPrintInfo } from '../lib/print';
  import { str } from '../lib/format';
  import type { Crumb, PageData, TreeNode } from '../lib/types';

  type View =
    | { t: 'home' }
    | { t: 'loading' }
    | { t: 'missing'; what: string }
    | { t: 'folder'; node: TreeNode; title: string; crumbs: Crumb[] }
    | { t: 'tags'; tag: string }
    | { t: 'page'; path: string };

  const route = $derived(router.route);
  const src = $derived(route.kind === 'library' ? route.src : '');

  // load a library's tree the first time it is needed
  $effect(() => {
    if (src && app.loaded && app.lib(src) && !app.trees[src]) app.loadTree(src);
  });

  const view = $derived.by<View>(() => {
    if (route.kind === 'home') return { t: 'home' };
    if (!app.loaded) return { t: 'loading' };
    const lib = app.lib(route.src);
    if (!lib) return { t: 'missing', what: `There is no open library called “${route.src}”.` };
    if (route.rest === '_tags' || route.rest.startsWith('_tags/')) return { t: 'tags', tag: route.rest.slice(6) };
    const idx = app.index(route.src);
    if (!idx) return app.trees[route.src] === undefined && lib && !lib.indexing && app.loaded && treeFailed ? { t: 'missing', what: lib.error || 'This library could not be read.' } : { t: 'loading' };

    const base = `/${route.src}/${route.rest}`;
    const folderKey = normalizePath(folderUrl(route.src, route.rest));
    if (!route.folder) {
      const path = idx.pages.get(base);
      if (path) return { t: 'page', path };
      const f = idx.folders.get(folderKey);
      if (f) return { t: 'folder', node: f, title: f.name || lib.name, crumbs: folderCrumbs(route.src, lib.name, route.rest) };
      return { t: 'missing', what: `There is no page at ${route.rest}.` };
    }
    const f = idx.folders.get(folderKey);
    if (!f) return { t: 'missing', what: `There is no folder called “${route.rest}”.` };
    if (route.rest === '' && idx.firstIndex) return { t: 'page', path: idx.firstIndex };
    return { t: 'folder', node: f, title: route.rest === '' ? lib.name : f.name, crumbs: folderCrumbs(route.src, lib.name, route.rest) };
  });
  let treeFailed = $state(false);

  function folderCrumbs(s: string, libName: string, rest: string): Crumb[] {
    const out: Crumb[] = [{ name: libName, url: `/${encodeURIComponent(s)}/` }];
    const segs = rest ? rest.split('/') : [];
    segs.forEach((seg, i) => out.push({ name: seg, url: i < segs.length - 1 ? folderUrl(s, segs.slice(0, i + 1).join('/')) : undefined }));
    return out;
  }

  // the root index page is the library's front page: show it at the library address without a redirect loop
  let page = $state<PageData | undefined>();
  let pageError = $state('');
  let loadedKey = '';

  $effect(() => {
    app.version; // reload when the server says something changed
    if (view.t !== 'page') {
      app.currentPath = '';
      return;
    }
    const key = `${src}\n${view.path}`;
    const same = key === loadedKey;
    const ctl = new AbortController();
    app.currentPath = view.path;
    pageError = '';
    api
      .page(src, view.path, ctl.signal)
      .then((p) => {
        const first = !same;
        page = p;
        loadedKey = key;
        document.title = `${p.title} · Docent`;
        const lib = app.lib(src);
        recent.remember({ title: p.title, url: p.url, library: lib?.name ?? src });
        setPrintInfo(() => ({ title: p.title, id: str(p.meta?.id), updated: str(p.meta?.updated) || p.modified.slice(0, 10), library: lib?.name ?? 'Docent' }));
        if (first && !location.hash) scrollTo(0, 0);
      })
      .catch((e: unknown) => {
        if (ctl.signal.aborted) return;
        pageError = e instanceof ApiError ? e.message : 'Could not load this page.';
      });
    return () => ctl.abort();
  });

  $effect(() => {
    if (view.t !== 'page') document.title = view.t === 'folder' ? `${view.title} · Docent` : 'Docent';
  });

  const crumbs = $derived<Crumb[]>(
    view.t === 'page' && page?.path === view.path ? page.crumbs : view.t === 'folder' ? view.crumbs : view.t === 'tags' ? [{ name: app.lib(src)?.name ?? src, url: `/${encodeURIComponent(src)}/` }, { name: 'Tags' }] : [{ name: view.t === 'home' ? 'Libraries' : 'Docent' }],
  );
  const shownPage = $derived(view.t === 'page' && page && page.source === src ? page : undefined);
</script>

<div class="main" tabindex="-1">
  <Topbar {crumbs} page={shownPage} />

  {#if view.t === 'home'}
    <Home />
  {:else if view.t === 'page'}
    {#if pageError}
      <div class="state"><h1>Could not open this page</h1><p>{pageError}</p><a class="btn" href={`/${encodeURIComponent(src)}/`}>Back to the library</a></div>
    {:else if shownPage}
      {#key shownPage.path}<PageView page={shownPage} />{/key}
      {#if app.lib(src)?.profile === 'devnotes' && route.kind === 'library' && route.rest === '' && shownPage.path === app.index(src)?.firstIndex}<RecentNotes {src} />{/if}
    {:else}
      <div class="reader"><article style="display:grid; gap:1rem"><div class="skeleton" style="height:2.4rem; width:60%"></div><div class="skeleton"></div><div class="skeleton" style="width:90%"></div><div class="skeleton" style="width:75%"></div></article></div>
    {/if}
  {:else if view.t === 'folder'}
    <FolderView {src} node={view.node} title={view.title} />
  {:else if view.t === 'tags'}
    <TagsView {src} tag={view.tag} />
  {:else if view.t === 'missing'}
    <div class="state"><h1>Nothing here</h1><p>{view.what}</p><a class="btn" href="/">All libraries</a></div>
  {:else}
    <div class="state"><div class="skeleton" style="width: 40%; margin: 0 auto"></div></div>
  {/if}
</div>
