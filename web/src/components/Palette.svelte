<script lang="ts">
  import Search from '@lucide/svelte/icons/search';
  import FileText from '@lucide/svelte/icons/file-text';
  import FolderOpen from '@lucide/svelte/icons/folder-open';
  import House from '@lucide/svelte/icons/house';
  import Sun from '@lucide/svelte/icons/sun';
  import Focus from '@lucide/svelte/icons/focus';
  import Printer from '@lucide/svelte/icons/printer';
  import Tag from '@lucide/svelte/icons/tag';
  import Hash from '@lucide/svelte/icons/hash';
  import FolderTree from '@lucide/svelte/icons/folder-tree';
  import CircleHelp from '@lucide/svelte/icons/circle-question-mark';
  import { app } from '../lib/app.svelte';
  import { prefs } from '../lib/prefs.svelte';
  import { router } from '../lib/router.svelte';
  import { ui } from '../lib/ui.svelte';
  import { openHelp } from '../lib/actions';
  import { highlight, searchSource } from '../lib/search';
  import { recent } from '../lib/recent.svelte';
  import { news } from '../lib/news.svelte';
  import Sparkles from '@lucide/svelte/icons/sparkles';
  import WrapText from '@lucide/svelte/icons/text-wrap';
  import ListTree from '@lucide/svelte/icons/table-of-contents';
  import { api } from '../lib/api';
  import { createdDay, localDay, stamp } from '../lib/format';
  import type { IdMatch, NoteRow, SearchHit } from '../lib/types';
  import type { Component } from 'svelte';

  interface Item { kind: 'page' | 'action'; title: string; sub?: string; snippet?: string; group: string; url?: string; icon: Component; run?: () => void }

  let dialog: HTMLDialogElement | undefined = $state();
  let input: HTMLInputElement | undefined = $state();
  let query = $state('');
  let results = $state<{ library: string; hits: SearchHit[] }[]>([]);
  let idHits = $state<IdMatch[]>([]);
  let sel = $state(0);
  let scopeAll = $state(false);
  let busy = $state(false);

  // "Created on" filter: a day in this browser's time zone ('' = any time). The lists of created days are loaded once per opening.
  let day = $state('');
  let created = $state<Record<string, NoteRow[]>>({}); // library id -> its pages with their created day
  const today = () => localDay(new Date());
  const yesterday = () => localDay(new Date(Date.now() - 86_400_000));
  const dayLabel = $derived(day === today() ? 'today' : day === yesterday() ? 'yesterday' : `on ${day}`);

  const src = $derived(router.route.kind === 'library' ? router.route.src : '');
  const allScope = $derived(scopeAll || !src);
  // libraries with note IDs: typing an ID (DOC-0015, doc-15, 15, DOC-001) jumps straight to the note
  const hasIds = $derived(app.libs.some((l) => l.idPrefix));
  const idShaped = (q: string) => /^(?:[A-Za-z]{2,5}[- ]?)?\d{1,9}$|^[A-Za-z]{2,5}[- ]?$/.test(q);

  const actions = $derived.by<Item[]>(() => {
    const list: Item[] = [
      { kind: 'action', group: 'Actions', title: 'Open a folder or file…', icon: FolderOpen, run: () => (ui.pickerOpen = true) },
      { kind: 'action', group: 'Actions', title: 'Browse files and folders…', icon: FolderTree, run: () => (ui.navigatorOpen = true) },
      { kind: 'action', group: 'Actions', title: 'Open the newest note you have not seen', icon: Sparkles, run: () => news.openNewest() },
      { kind: 'action', group: 'Actions', title: 'Docent guide (help)', icon: CircleHelp, run: openHelp },
      { kind: 'action', group: 'Actions', title: 'All libraries', icon: House, run: () => router.go('/') },
      { kind: 'action', group: 'Actions', title: `Theme: ${prefs.theme}. Change`, icon: Sun, run: () => prefs.cycleTheme() },
      { kind: 'action', group: 'Actions', title: prefs.focus ? 'Show the sidebars' : 'Focus mode', icon: Focus, run: () => prefs.set('focus', !prefs.focus) },
    ];
    if (src) list.push({ kind: 'action', group: 'Actions', title: 'Browse tags', icon: Tag, run: () => router.go(`/${encodeURIComponent(src)}/_tags`) });
    list.push({ kind: 'action', group: 'Actions', title: prefs.wrapCode ? 'Scroll long code lines sideways' : 'Wrap long code lines', icon: WrapText, run: () => prefs.set('wrapCode', !prefs.wrapCode) });
    if (src && app.currentPath) list.push({ kind: 'action', group: 'Actions', title: prefs.toc ? 'Hide the contents list' : 'Show the contents list', icon: ListTree, run: () => prefs.set('toc', !prefs.toc) });
    if (src && app.currentPath) list.push({ kind: 'action', group: 'Actions', title: 'Print or save as PDF…', icon: Printer, run: () => (ui.printOpen = true) });
    return app.readOnly ? list.filter((a) => !a.title.startsWith('Open a folder')) : list;
  });

  const targets = $derived(allScope ? app.libs.filter((l) => !l.indexing) : app.libs.filter((l) => l.id === src));

  $effect(() => {
    if (!day) return;
    const missing = targets.filter((l) => !created[l.id]);
    if (!missing.length) return;
    const ctl = new AbortController();
    Promise.all(missing.map(async (l) => [l.id, (await api.created(l.id, ctl.signal).catch(() => ({ pages: [] as NoteRow[] }))).pages] as const)).then((got) => {
      if (!ctl.signal.aborted) created = { ...created, ...Object.fromEntries(got) };
    });
    return () => ctl.abort();
  });

  // the pages created on the chosen day; with a query, those the search found first, then those whose title, ID or path has every word
  const dayItems = $derived.by<Item[]>(() => {
    if (!day) return [];
    const q = query.trim().toLowerCase();
    const words = q.split(/\s+/).filter(Boolean);
    const out: Item[] = [];
    for (const l of targets) {
      const rows = (created[l.id] ?? []).filter((r) => createdDay(r) === day);
      const hitOrder = new Map((results.find((g) => g.library === l.name)?.hits ?? []).map((h, i) => [h.path, i]));
      const matches = q ? rows.filter((r) => hitOrder.has(r.path) || words.every((w) => `${r.title} ${r.id ?? ''} ${r.path}`.toLowerCase().includes(w))) : rows;
      matches.sort((a, b) => (q ? (hitOrder.get(a.path) ?? 1e9) - (hitOrder.get(b.path) ?? 1e9) : 0) || (b.createdAt ?? '').localeCompare(a.createdAt ?? '') || a.title.localeCompare(b.title));
      for (const r of matches) {
        const time = r.createdAt ? stamp(r.created, r.createdAt).text.slice(11) : '';
        const where = r.folder && r.folder !== '.' ? r.folder : l.name;
        out.push({ kind: 'page', group: allScope ? l.name : `Created ${dayLabel}`, title: r.title, sub: [r.id, time, where].filter(Boolean).join(' · '), url: r.url, icon: FileText });
      }
    }
    return out;
  });
  const dayLoading = $derived(!!day && targets.some((l) => !created[l.id]));

  const items = $derived.by<Item[]>(() => {
    const q = query.trim();
    const out: Item[] = [];
    if (day) return dayItems;
    if (q) {
      for (const m of idHits) {
        out.push({ kind: 'page', group: 'Note ID', title: m.title, sub: `${m.id} · ${m.library}`, url: m.url, icon: Hash });
      }
      for (const g of results) {
        for (const h of g.hits.slice(0, allScope ? 5 : 12)) {
          out.push({ kind: 'page', group: allScope ? g.library : 'Pages', title: h.title, sub: h.folder && h.folder !== '.' ? h.folder : g.library, snippet: h.snippet, url: h.url, icon: FileText });
        }
      }
      out.push(...actions.filter((a) => a.title.toLowerCase().includes(q.toLowerCase())));
    } else {
      for (const n of news.list(5).filter((x) => x.unseen)) out.push({ kind: 'page', group: 'New notes', title: n.title, sub: `${n.id} · ${n.library}`, url: n.url, icon: Sparkles });
      for (const r of recent.list) out.push({ kind: 'page', group: 'Recent pages', title: r.title, sub: r.library, url: r.url, icon: FileText });
      out.push(...actions);
    }
    return out;
  });

  // run the search shortly after typing stops, and cancel the previous one
  let timer: ReturnType<typeof setTimeout> | undefined;
  let ctl: AbortController | undefined;
  $effect(() => {
    const q = query.trim();
    allScope;
    clearTimeout(timer);
    ctl?.abort();
    if (!q) {
      results = [];
      idHits = [];
      busy = false;
      return;
    }
    busy = true;
    timer = setTimeout(async () => {
      ctl = new AbortController();
      const signal = ctl.signal;
      try {
        const [all, ids] = await Promise.all([
          Promise.all(targets.map(async (l) => ({ library: l.name, hits: await searchSource(l.id, q, signal).catch(() => [] as SearchHit[]) }))),
          hasIds && idShaped(q) ? api.ids(q, src, signal).then((r) => r.matches).catch(() => [] as IdMatch[]) : Promise.resolve([] as IdMatch[]),
        ]);
        if (signal.aborted) return;
        results = all.filter((g) => g.hits.length);
        idHits = ids;
        sel = 0;
      } finally {
        if (!signal.aborted) busy = false;
      }
    }, 90);
    return () => clearTimeout(timer);
  });

  $effect(() => {
    if (ui.paletteOpen && dialog && !dialog.open) {
      query = '';
      sel = 0;
      scopeAll = false;
      day = '';
      created = {}; // pages may have been added since the last opening
      dialog.showModal();
      queueMicrotask(() => input?.focus());
    }
    if (!ui.paletteOpen && dialog?.open) dialog.close();
  });

  function choose(i: number) {
    const it = items[i];
    if (!it) return;
    ui.paletteOpen = false;
    if (it.kind === 'action') it.run?.();
    else if (it.url) router.go(it.url);
  }

  function key(e: KeyboardEvent) {
    if (e.key === 'ArrowDown') { e.preventDefault(); sel = (sel + 1) % Math.max(items.length, 1); }
    else if (e.key === 'ArrowUp') { e.preventDefault(); sel = (sel - 1 + items.length) % Math.max(items.length, 1); }
    else if (e.key === 'Enter') { e.preventDefault(); choose(sel); }
    else if (e.key === 'Tab' && src) { e.preventDefault(); scopeAll = !scopeAll; }
    else return;
    queueMicrotask(() => dialog?.querySelector('[aria-selected="true"]')?.scrollIntoView({ block: 'nearest' }));
  }

  const groups = $derived.by(() => {
    const out: { name: string; rows: { it: Item; i: number }[] }[] = [];
    items.forEach((it, i) => {
      let g = out.find((x) => x.name === it.group);
      if (!g) out.push((g = { name: it.group, rows: [] }));
      g.rows.push({ it, i });
    });
    return out;
  });
</script>

<dialog class="palette" bind:this={dialog} aria-label="Search" onclose={() => { if (!dialog?.open) ui.paletteOpen = false; }} onclick={(e) => { if (e.target === dialog) ui.paletteOpen = false; }}>
  <div class="sheet">
    <div class="palette__input">
      <Search class="icon" />
      <input bind:this={input} bind:value={query} onkeydown={key} placeholder={hasIds ? (allScope ? 'Search all libraries, or type a note ID (DOC-0015)' : 'Search this library, or type a note ID (DOC-0015)') : allScope ? 'Search all libraries, or type a command' : 'Search this library, or type a command'} autocomplete="off" spellcheck="false" aria-label="Search" />
      {#if src}
        <button class="btn btn--sm" type="button" aria-pressed={scopeAll} title="Tab switches between this library and all libraries" onclick={() => (scopeAll = !scopeAll)}>{allScope ? 'All libraries' : 'This library'}</button>
      {/if}
      <kbd>Esc</kbd>
    </div>
    <div class="palette__filter" role="group" aria-label="Created">
      <span class="palette__filterlabel">Created</span>
      <button class="chip" type="button" aria-pressed={!day} onclick={() => { day = ''; input?.focus(); }}>Any time</button>
      <button class="chip" type="button" aria-pressed={day === today()} onclick={() => { day = today(); sel = 0; input?.focus(); }}>Today</button>
      <button class="chip" type="button" aria-pressed={day === yesterday()} onclick={() => { day = yesterday(); sel = 0; input?.focus(); }}>Yesterday</button>
      <input class="chip palette__date" class:is-set={!!day && day !== today() && day !== yesterday()} type="date" aria-label="Created on" max={today()} value={day} onchange={(e) => { day = (e.currentTarget as HTMLInputElement).value; sel = 0; }} />
    </div>
    <div class="palette__list" role="listbox" aria-label="Results">
      {#each groups as g (g.name)}
        <div class="palette__group">{g.name}</div>
        {#each g.rows as { it, i } (i)}
          <button class="palette__item" type="button" role="option" aria-selected={i === sel} onmouseenter={() => (sel = i)} onclick={() => choose(i)}>
            <span class="ic"><it.icon class="icon icon--sm" /></span>
            <span class="tx">
              <div class="tt">{@html it.kind === 'page' ? highlight(it.title, query) : it.title}</div>
              {#if it.snippet}<div class="pp">{@html highlight(it.snippet, query)}</div>{:else if it.sub}<div class="pp">{it.sub}</div>{/if}
            </span>
          </button>
        {/each}
      {/each}
      {#if day}
        {#if dayLoading}
          <div class="palette__empty">Loading…</div>
        {:else if !items.length && !busy}
          <div class="palette__empty">{query.trim() ? `No pages created ${dayLabel} match “${query.trim()}”` : `No pages were created ${dayLabel}`}</div>
        {/if}
      {:else if query.trim() && !busy && !results.length}
        <div class="palette__empty">No pages match “{query.trim()}”</div>
      {:else if busy && !results.length}
        <div class="palette__empty">Searching…</div>
      {/if}
    </div>
    <div class="palette__foot"><span><kbd>↑</kbd><kbd>↓</kbd> move</span><span><kbd>↵</kbd> open</span>{#if src}<span><kbd>Tab</kbd> {allScope ? 'this library' : 'all libraries'}</span>{/if}<span><kbd>Esc</kbd> close</span></div>
  </div>
</dialog>
