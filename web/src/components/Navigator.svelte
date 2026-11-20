<script lang="ts">
  import X from '@lucide/svelte/icons/x';
  import Search from '@lucide/svelte/icons/search';
  import Folder from '@lucide/svelte/icons/folder';
  import FileText from '@lucide/svelte/icons/file-text';
  import BookOpen from '@lucide/svelte/icons/book-open';
  import ChevronRight from '@lucide/svelte/icons/chevron-right';
  import ArrowLeft from '@lucide/svelte/icons/arrow-left';
  import FolderTree from '@lucide/svelte/icons/folder-tree';
  import { api } from '../lib/api';
  import { app } from '../lib/app.svelte';
  import { copyText } from '../lib/format';
  import { chainFor, childrenOf, columnsFor, excerptHtml, moveWithin, searchResults, type NavItem } from '../lib/navigator';
  import { router } from '../lib/router.svelte';
  import { toast } from '../lib/toast.svelte';
  import { ui } from '../lib/ui.svelte';
  import { highlightParts, termsOf } from '../lib/treenav';
  import type { PageData } from '../lib/types';

  let dialog: HTMLDialogElement | undefined = $state();
  let input: HTMLInputElement | undefined = $state();
  let colsEl: HTMLElement | undefined = $state();
  let chain = $state<NavItem[]>([]); // one selected item per column, from the libraries column to the right
  let query = $state('');
  let hit = $state(0); // the selected result while a search is typed
  let view = $state<number | null>(null); // on a phone only one column shows: null is the newest, Back moves to the one before
  let pristine = true; // true until the reader moves: until then the popup keeps following the page being read (its tree may still be loading when it opens)

  const libs = $derived(app.listed);
  const cols = $derived(columnsFor(chain, libs, app.trees));
  const searching = $derived(query.trim().length > 0);
  const scope = $derived(chain[0]?.lib ?? (router.route.kind === 'library' ? router.route.src : ''));
  const results = $derived(searching && scope ? searchResults(scope, app.trees, query) : []);
  const active = $derived<NavItem | undefined>(searching ? results[hit] : chain.at(-1));
  const terms = $derived(termsOf(query));
  const activeCol = $derived(Math.max(chain.length - 1, 0));
  const shown = $derived(Math.min(view ?? cols.length - 1, cols.length - 1));
  $effect(() => {
    chain; // choosing something shows the newest column again
    view = null;
  });

  // ───────── opening ─────────
  $effect(() => {
    if (ui.navigatorOpen && dialog && !dialog.open) {
      query = '';
      hit = 0;
      pristine = true;
      chain = startChain();
      dialog.showModal();
      queueMicrotask(() => input?.focus());
    }
    if (!ui.navigatorOpen && dialog?.open) dialog.close();
  });

  const startChain = () => {
    const here = router.route.kind === 'library' ? router.route.src : '';
    return here ? chainFor(libs, app.trees, here, app.currentPath) : [];
  };

  // when the tree of the library being read arrives after the popup opened, show where the reader is
  $effect(() => {
    const here = router.route.kind === 'library' ? router.route.src : '';
    if (ui.navigatorOpen && pristine && here && app.trees[here]) chain = startChain();
  });

  // trees load when a library is first selected
  $effect(() => {
    const lib = chain[0];
    if (lib && !app.trees[lib.lib]) void app.loadTree(lib.lib);
  });

  // keep the columns scrolled so the newest one and the selected rows are in view
  $effect(() => {
    chain;
    cols.length;
    queueMicrotask(() => {
      colsEl?.scrollTo({ left: colsEl.scrollWidth, behavior: 'auto' });
      dialog?.querySelectorAll('.col [aria-selected="true"]').forEach((el) => el.scrollIntoView({ block: 'nearest', inline: 'nearest' }));
    });
  });
  $effect(() => {
    hit;
    queueMicrotask(() => dialog?.querySelector('.results [aria-selected="true"]')?.scrollIntoView({ block: 'nearest' }));
  });
  $effect(() => {
    query;
    hit = 0;
  });

  // ───────── the preview of what is selected ─────────
  let preview = $state<{ key: string; page?: PageData; error?: string } | null>(null);
  let timer: ReturnType<typeof setTimeout> | undefined;
  let ctl: AbortController | undefined;
  $effect(() => {
    const it = active;
    clearTimeout(timer);
    ctl?.abort();
    if (!it || it.kind !== 'page' || !ui.navigatorOpen) {
      preview = null;
      return;
    }
    if (preview?.key !== it.key) preview = { key: it.key };
    timer = setTimeout(async () => {
      ctl = new AbortController();
      try {
        const page = await api.page(it.lib, it.path, ctl.signal);
        preview = { key: it.key, page };
      } catch (e) {
        if (!ctl?.signal.aborted) preview = { key: it.key, error: e instanceof Error ? e.message : 'Could not load the preview' };
      }
    }, 120);
    return () => clearTimeout(timer);
  });
  const excerpt = $derived(preview?.page ? excerptHtml(preview.page.html) : '');
  const meta = $derived((preview?.page?.meta ?? {}) as Record<string, unknown>);
  const str = (v: unknown) => (typeof v === 'string' || typeof v === 'number' ? String(v) : '');
  const tags = $derived(Array.isArray(meta.tags) ? (meta.tags as unknown[]).map(String).slice(0, 5) : []);

  // ───────── moving ─────────
  const rowsOf = (i: number) => cols[i]?.items ?? [];

  function select(item: NavItem, col: number) {
    pristine = false;
    chain = [...chain.slice(0, col), item];
  }

  /** Into the selected folder or library: its first item becomes the selection. */
  async function enter() {
    const it = chain.at(-1);
    if (!it || it.kind === 'page') return;
    pristine = false;
    if (it.kind === 'library' && !app.trees[it.lib]) await app.loadTree(it.lib);
    const first = childrenOf(it, app.trees)?.[0];
    if (first) chain = [...chain, first];
  }

  function open(item: NavItem | undefined) {
    if (!item?.url) return;
    ui.navigatorOpen = false;
    router.go(item.url);
  }

  function step(delta: number | 'first' | 'last') {
    if (searching) {
      hit = typeof delta === 'number' ? Math.min(Math.max(hit + delta, 0), Math.max(results.length - 1, 0)) : delta === 'first' ? 0 : Math.max(results.length - 1, 0);
      return;
    }
    pristine = false;
    const next = moveWithin(rowsOf(activeCol), chain.at(-1)?.key, delta);
    if (next) chain = chain.length ? [...chain.slice(0, -1), next] : [next];
  }

  function key(e: KeyboardEvent) {
    const empty = !searching;
    if (e.key === 'ArrowDown') step(1);
    else if (e.key === 'ArrowUp') step(-1);
    else if (e.key === 'PageDown') step(8);
    else if (e.key === 'PageUp') step(-8);
    else if (e.key === 'Home' && empty) step('first');
    else if (e.key === 'End' && empty) step('last');
    else if (e.key === 'ArrowRight' && empty) void enter();
    else if ((e.key === 'ArrowLeft' || e.key === 'Backspace') && empty && chain.length > 1) {
      pristine = false;
      chain = chain.slice(0, -1);
    }
    else if (e.key === 'Enter') {
      if (searching || e.shiftKey || active?.kind === 'page') open(active);
      else void enter();
    } else return;
    e.preventDefault();
  }

  function cancel(e: Event) {
    if (query) {
      e.preventDefault(); // Escape clears the search first, and closes the popup the next time
      query = '';
    }
  }

  async function copyPath() {
    if (!active) return;
    toast.show((await copyText(active.kind === 'library' ? libs.find((l) => l.id === active.lib)?.root ?? active.path : active.path)) ? 'Path copied' : 'Could not copy');
  }

  const crumbs = $derived([{ label: 'Libraries', at: 0 }, ...chain.map((c, i) => ({ label: c.label, at: i + 1 }))]);
  const activeId = $derived(active ? (searching ? `nav-r-${hit}` : `nav-${activeCol}-${rowsOf(activeCol).findIndex((x) => x.key === active.key)}`) : undefined);
  const sizeText = (it: NavItem) => (it.kind === 'page' ? '' : it.count !== undefined ? String(it.count) : '');
  const rootOf = (lib: string) => libs.find((l) => l.id === lib)?.root ?? '';
</script>

<dialog class="navdialog" bind:this={dialog} aria-label="Browse files" oncancel={cancel} onclose={() => { if (!dialog?.open) ui.navigatorOpen = false; }} onclick={(e) => { if (e.target === dialog) ui.navigatorOpen = false; }}>
  <div class="sheet nav">
    <header class="nav__head">
      <button class="btn btn--ghost btn--icon btn--sm only-mobile" type="button" aria-label="Back" disabled={shown === 0} onclick={() => (view = Math.max(shown - 1, 0))}><ArrowLeft class="icon" /></button>
      <FolderTree class="icon nav__logo" />
      <nav class="nav__crumbs" aria-label="Where you are">
        {#each crumbs as c, i (i)}
          {#if i > 0}<ChevronRight class="icon icon--sm" />{/if}
          <button type="button" class="crumb" aria-current={c.at === chain.length ? 'location' : undefined} onclick={() => { pristine = false; chain = chain.slice(0, c.at); }}>{c.label}</button>
        {/each}
      </nav>
      <button class="btn btn--ghost btn--icon btn--sm" type="button" aria-label="Close" onclick={() => (ui.navigatorOpen = false)}><X class="icon" /></button>
    </header>

    <div class="nav__search">
      <Search class="icon icon--sm" />
      <input bind:this={input} bind:value={query} onkeydown={key} type="text" spellcheck="false" autocomplete="off" aria-label="Find a page in {scope ? libs.find((l) => l.id === scope)?.name : 'a library'}"
        aria-activedescendant={activeId} placeholder={scope ? 'Type to find a page here. Arrow keys move around.' : 'Choose a library, then browse its folders.'} />
      {#if searching}<span class="nav__count" role="status">{results.length}{results.length >= 200 ? '+' : ''} {results.length === 1 ? 'page' : 'pages'}</span>{/if}
    </div>

    <div class="nav__body">
      {#if searching}
        <div class="results" role="listbox" aria-label="Matching pages">
          {#each results as r, i (r.key)}
            <button class="row" id="nav-r-{i}" type="button" role="option" tabindex="-1" aria-selected={i === hit} onmouseenter={() => (hit = i)} onclick={() => (hit = i)} ondblclick={() => open(r)}>
              <FileText class="icon icon--sm" />
              <span class="label"><span class="t">{#each highlightParts(r.label, terms) as p, j (j)}{#if p.hit}<mark>{p.text}</mark>{:else}{p.text}{/if}{/each}</span>{#if r.sub}<span class="s">{r.sub}</span>{/if}</span>
            </button>
          {:else}
            <p class="empty">No page matches “{query.trim()}”{scope ? '' : '. Choose a library first.'}</p>
          {/each}
        </div>
      {:else}
        <div class="cols" bind:this={colsEl}>
          {#each cols as col, ci (ci)}
            <div class="col" class:is-active={ci === activeCol} class:is-shown={ci === shown} role="listbox" aria-label={ci === 0 ? 'Libraries' : `Inside ${chain[ci - 1]?.label}`}>
              {#if col.loading}
                <div class="skel"><div class="skeleton"></div><div class="skeleton" style="width: 80%"></div><div class="skeleton" style="width: 90%"></div></div>
              {:else}
                {#each col.items as it, i (it.key)}
                  <button class="row" id="nav-{ci}-{i}" type="button" role="option" tabindex="-1" aria-selected={it.key === col.selected} onclick={() => { select(it, ci); input?.focus(); }} ondblclick={() => open(it)}>
                    {#if it.kind === 'library'}<BookOpen class="icon icon--sm" />{:else if it.kind === 'folder'}<Folder class="icon icon--sm" />{:else}<FileText class="icon icon--sm" />{/if}
                    <span class="label"><span class="t">{it.label}</span>{#if it.sub}<span class="s">{it.sub}</span>{/if}</span>
                    {#if sizeText(it)}<span class="n">{sizeText(it)}</span>{/if}
                    {#if it.kind !== 'page'}<ChevronRight class="icon icon--sm go" />{/if}
                  </button>
                {:else}
                  <p class="empty">{ci === 0 ? 'No libraries yet.' : 'Nothing in here.'}</p>
                {/each}
              {/if}
            </div>
          {/each}
        </div>
      {/if}

      <aside class="preview" aria-label="Preview">
        {#if !active}
          <p class="hint">Select a page to see what is in it before you open it.</p>
        {:else if active.kind === 'page'}
          <h3>{preview?.page?.title ?? active.label}</h3>
          <p class="path">{active.path}</p>
          <div class="chips">
            {#if active.sub}<span class="chip">{active.sub.split(' · ')[0]}</span>{/if}
            {#if str(meta.date)}<span class="chip">{str(meta.date)}</span>{/if}
            {#if str(meta.status)}<span class="chip">{str(meta.status)}</span>{/if}
            {#each tags as t (t)}<span class="chip">{t}</span>{/each}
          </div>
          {#if preview?.error}<p class="hint">{preview.error}</p>
          {:else if !preview?.page}<div class="skel"><div class="skeleton"></div><div class="skeleton" style="width: 85%"></div><div class="skeleton" style="width: 70%"></div></div>
          {:else}<div class="prose excerpt">{@html excerpt}</div>{/if}
        {:else}
          <h3>{active.label}</h3>
          <p class="path">{active.kind === 'library' ? rootOf(active.lib) : active.path}</p>
          <p>{active.count ?? 0} {active.count === 1 ? 'page' : 'pages'}{active.kind === 'library' ? ' in this library' : ' in this folder'}.</p>
          <p class="hint">Press → to look inside, or Shift+Enter to open its page.</p>
        {/if}
      </aside>
    </div>

    <footer class="nav__foot">
      <span class="keys"><kbd>↑</kbd><kbd>↓</kbd> move <kbd>→</kbd> open folder <kbd>←</kbd> back <kbd>Enter</kbd> open <kbd>Esc</kbd> close</span>
      <span class="grow"></span>
      <button class="btn btn--sm" type="button" disabled={!active} onclick={copyPath}>Copy path</button>
      <button class="btn btn--sm btn--primary" type="button" disabled={!active} onclick={() => open(active)}>{active?.kind === 'page' || !active ? 'Open' : active.kind === 'library' ? 'Open library' : 'Open folder'}</button>
    </footer>
  </div>
</dialog>

<style>
  .nav { width: min(76rem, calc(100vw - 1.5rem)); height: min(44rem, calc(100dvh - 3rem)); display: grid; grid-template-columns: minmax(0, 1fr); grid-template-rows: auto auto minmax(0, 1fr) auto; }
  .nav > * { min-width: 0; }
  .nav__head { display: flex; align-items: center; gap: var(--space-2); padding: var(--space-3) var(--space-4); border-bottom: 1px solid var(--border); }
  :global(.nav__logo) { color: var(--accent-strong); flex: none; }
  .nav__crumbs { flex: 1; min-width: 0; display: flex; align-items: center; gap: 2px; overflow-x: auto; white-space: nowrap; scrollbar-width: none; }
  .crumb { border: 0; background: transparent; padding: 0.2rem 0.4rem; border-radius: 6px; color: var(--text-muted); font-size: var(--text-ui); }
  .crumb:hover { background: var(--surface-2); color: var(--text); }
  .crumb[aria-current='location'] { color: var(--text); font-weight: 600; }
  .nav__search { display: flex; align-items: center; gap: var(--space-2); padding: 0 var(--space-4); height: 2.75rem; border-bottom: 1px solid var(--border); color: var(--text-faint); }
  .nav__search input { flex: 1; height: 100%; border: 0; outline: 0; background: transparent; color: var(--text); font-size: var(--text-ui); }
  .nav__count { font-size: var(--text-xs); color: var(--text-muted); }
  .nav__body { display: grid; grid-template-columns: minmax(0, 1fr) 21rem; min-height: 0; }
  .cols { display: flex; overflow-x: auto; min-width: 0; min-height: 0; }
  .col, .results { flex: 0 0 17rem; overflow-y: auto; padding: var(--space-2); border-right: 1px solid var(--border); min-height: 0; }
  .results { flex: 1; border-right: 0; }
  .row { display: flex; align-items: flex-start; gap: var(--space-2); width: 100%; padding: 0.4rem var(--space-2); border: 0; border-radius: var(--radius-md); background: transparent; color: var(--text); text-align: left; font-size: var(--text-ui); }
  .row :global(.icon) { flex: none; margin-top: 0.15rem; color: var(--text-faint); }
  .row:hover { background: var(--surface-2); }
  .row[aria-selected='true'] { background: var(--surface-3); }
  .col.is-active .row[aria-selected='true'], .results .row[aria-selected='true'] { background: var(--accent-soft); }
  .col.is-active .row[aria-selected='true'] :global(.icon), .results .row[aria-selected='true'] :global(.icon) { color: var(--accent-strong); }
  .label { flex: 1; min-width: 0; display: grid; gap: 1px; }
  .t { overflow-wrap: anywhere; line-height: 1.3; }
  .s { font: 450 var(--text-xs)/1.3 var(--font-mono); color: var(--text-faint); overflow-wrap: anywhere; }
  .n { font-size: var(--text-2xs); color: var(--text-faint); font-variant-numeric: tabular-nums; margin-top: 0.2rem; }
  .row :global(.go) { margin-left: -0.25rem; }
  .row mark { background: var(--accent-soft); color: inherit; border-radius: 3px; padding: 0 1px; font-weight: 650; }
  .empty, .hint { margin: var(--space-3); color: var(--text-muted); font-size: var(--text-sm); }
  .skel { display: grid; gap: 0.7rem; padding: var(--space-3); }
  .preview { overflow-y: auto; padding: var(--space-4) var(--space-5); background: var(--surface-2); border-left: 1px solid var(--border); min-height: 0; }
  .preview h3 { margin: 0 0 var(--space-1); font-size: 1.05rem; letter-spacing: -0.01em; line-height: 1.3; }
  .preview .path { margin: 0 0 var(--space-3); font: 450 var(--text-xs)/1.4 var(--font-mono); color: var(--text-faint); overflow-wrap: anywhere; }
  .chips { display: flex; flex-wrap: wrap; gap: var(--space-1); margin-bottom: var(--space-3); }
  .excerpt { font-size: 0.86rem; line-height: 1.55; max-height: 22rem; overflow: hidden; -webkit-mask-image: linear-gradient(#000 80%, transparent); mask-image: linear-gradient(#000 80%, transparent); }
  .excerpt :global(h2), .excerpt :global(h3) { font-size: 0.95rem; margin: 0.8em 0 0.3em; }
  .excerpt :global(pre), .excerpt :global(.code), .excerpt :global(table) { font-size: 0.78rem; }
  .excerpt :global(.anchor), .excerpt :global(.code__bar) { display: none; }
  .nav__foot { display: flex; align-items: center; gap: var(--space-2); padding: var(--space-3) var(--space-4); border-top: 1px solid var(--border); background: var(--surface-2); }
  .keys { font-size: var(--text-xs); color: var(--text-faint); display: flex; align-items: center; gap: 0.3rem; flex-wrap: wrap; }
  .grow { flex: 1; }
  @media (max-width: 860px) {
    .nav { height: calc(100dvh - 1.5rem); }
    .nav__body { grid-template-columns: minmax(0, 1fr); }
    .preview, .keys { display: none; }
    .col { display: none; }
    .col.is-shown { display: block; flex: 1; border-right: 0; }
    :global(.nav__logo) { display: none; }
  }
</style>
