<script lang="ts">
  import ChevronDown from '@lucide/svelte/icons/chevron-down';
  import Search from '@lucide/svelte/icons/search';
  import FolderOpen from '@lucide/svelte/icons/folder-open';
  import House from '@lucide/svelte/icons/house';
  import Plus from '@lucide/svelte/icons/plus';
  import X from '@lucide/svelte/icons/x';
  import ListFilter from '@lucide/svelte/icons/list-filter';
  import Tree from './Tree.svelte';
  import { app } from '../lib/app.svelte';
  import { news } from '../lib/news.svelte';
  import { router } from '../lib/router.svelte';
  import { ui } from '../lib/ui.svelte';
  import { initials } from '../lib/format';
  import { prefs, RAIL_DEFAULT, RAIL_MAX, RAIL_MIN } from '../lib/prefs.svelte';
  import { filterTree } from '../lib/treenav';

  let { src, current }: { src: string; current: string } = $props();
  const lib = $derived(src ? app.lib(src) : undefined);
  // new notes in the libraries you are not looking at: a dot on the switcher says so before you open the menu
  const elsewhere = $derived(news.sources.filter((s) => s.source !== src).reduce((n, s) => n + news.count(s.source), 0));
  const tree = $derived(src ? app.trees[src] : undefined);

  // filter the page tree: every typed word has to match the title, the path or the note ID
  let filter = $state('');
  const filtered = $derived(tree ? filterTree(tree.tree.children ?? [], filter) : undefined);
  const showFilter = $derived((filtered?.total ?? 0) > 8);
  $effect(() => {
    src; // another library: start with an empty filter
    filter = '';
  });

  // dragging the right edge of the column makes it wider or narrower (the width is remembered)
  let dragging = $state(false);
  function startDrag(e: PointerEvent) {
    if (e.button !== 0) return;
    e.preventDefault();
    dragging = true;
    const move = (ev: PointerEvent) => prefs.setRailWidth(ev.clientX);
    const stop = () => {
      dragging = false;
      removeEventListener('pointermove', move);
      removeEventListener('pointerup', stop);
      removeEventListener('pointercancel', stop);
    };
    addEventListener('pointermove', move);
    addEventListener('pointerup', stop);
    addEventListener('pointercancel', stop);
  }
  function resizeKey(e: KeyboardEvent) {
    const step = e.shiftKey ? 48 : 16;
    if (e.key === 'ArrowRight') prefs.setRailWidth(prefs.railWidth + step);
    else if (e.key === 'ArrowLeft') prefs.setRailWidth(prefs.railWidth - step);
    else if (e.key === 'Home') prefs.setRailWidth(RAIL_MIN);
    else if (e.key === 'End') prefs.setRailWidth(RAIL_MAX);
    else if (e.key === 'Enter') prefs.setRailWidth(RAIL_DEFAULT);
    else return;
    e.preventDefault();
  }

  function pick(id: string) {
    ui.libMenuOpen = false;
    router.go(`/${encodeURIComponent(id)}/`);
  }
</script>

<aside class="rail" aria-label="Navigation">
  <div class="rail__head">
    <div style="position: relative">
      <button class="libswitch" type="button" data-menu-toggle aria-haspopup="true" aria-expanded={ui.libMenuOpen}
        onclick={() => (ui.libMenuOpen = !ui.libMenuOpen)}>
        <span class="libswitch__mark">{lib ? initials(lib.name) : 'D'}{#if elsewhere}<span class="newdot newdot--badge" aria-hidden="true"></span>{/if}</span>
        <span class="libswitch__text">
          <span class="libswitch__name" style="display:block">{lib ? lib.name : 'Docent'}</span>
          <span class="libswitch__meta">{lib ? `${lib.pages} ${lib.pages === 1 ? 'page' : 'pages'}${lib.kind === 'library' ? '' : ' · opened once'}` : 'All libraries'}</span>
        </span>
        {#if elsewhere}<span class="sr-only">{elsewhere} new {elsewhere === 1 ? 'note' : 'notes'} in other libraries</span>{/if}
        <ChevronDown class="icon icon--sm" style="color: var(--text-faint)" />
      </button>

      {#if ui.libMenuOpen}
        <div class="popover" style="position:absolute; left:0; right:0; top:calc(100% + 6px); width:auto" aria-label="Libraries">
          <div class="menu-list">
            {#each app.listed as l (l.id)}
              <button class="menu-item" type="button" aria-current={l.id === src ? 'true' : undefined} onclick={() => pick(l.id)}>
                <span class="libswitch__mark libswitch__mark--sm">{initials(l.name)}</span>
                <span class="n">{l.name}</span>{#if news.count(l.id)}<span class="newcount">{news.count(l.id)} new</span>{/if}<span class="muted">{l.pages}</span>
              </button>
            {/each}
            <div class="menu-sep"></div>
            <button class="menu-item" type="button" onclick={() => { ui.libMenuOpen = false; router.go('/'); }}><House class="icon icon--sm" /><span class="n">All libraries</span></button>
            {#if !app.readOnly}
              <button class="menu-item" type="button" style="color: var(--accent-strong)" onclick={() => { ui.libMenuOpen = false; ui.pickerOpen = true; }}><Plus class="icon icon--sm" /><span class="n">Open or add…</span></button>
            {/if}
          </div>
        </div>
      {/if}
    </div>

    <button class="searchbtn" type="button" onclick={() => (ui.paletteOpen = true)}>
      <Search class="icon icon--sm" /><span>Search</span><kbd>Ctrl K</kbd>
    </button>
  </div>

  <nav class="rail__scroll">
    {#if src && tree}
      {#if (tree.tree.children ?? []).length}
        <div class="rail__label">Pages</div>
        {#if showFilter}
          <div class="treefilter">
            <ListFilter class="icon icon--sm" />
            <input type="text" bind:value={filter} placeholder="Filter pages" aria-label="Filter pages by title, folder or ID" spellcheck="false" autocomplete="off"
              onkeydown={(e) => { if (e.key === 'Escape' && filter) { e.stopPropagation(); filter = ''; } }} />
            {#if filter}<button class="btn btn--ghost btn--icon btn--sm" type="button" aria-label="Clear the filter" onclick={() => (filter = '')}><X class="icon icon--sm" /></button>{/if}
          </div>
          {#if filter.trim()}<p class="treefilter__count" role="status">{filtered?.matches} of {filtered?.total} pages</p>{/if}
        {/if}
        {#if filtered && filtered.nodes.length}
          <Tree {src} nodes={filtered.nodes} {current} {filter} />
        {:else}
          <p class="rail__empty">No page matches “{filter}”.</p>
        {/if}
      {:else}
        <p class="rail__empty">{tree.info.error ? tree.info.error : 'No Markdown files in this folder yet.'}</p>
      {/if}
    {:else if src}
      <div style="display:grid; gap:0.7rem; padding: var(--space-3)"><div class="skeleton"></div><div class="skeleton" style="width:80%"></div><div class="skeleton" style="width:90%"></div></div>
    {:else}
      <p class="rail__empty">Pick a library to browse its pages.</p>
    {/if}
  </nav>

  <!-- a focusable separator is the ARIA "window splitter" pattern: it is interactive -->
  <!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
  <div class="rail__resize" class:is-dragging={dragging} role="separator" aria-orientation="vertical" aria-label="Resize the page tree" aria-valuemin={RAIL_MIN} aria-valuemax={RAIL_MAX} aria-valuenow={prefs.railWidth} tabindex="0"
    title="Drag to resize; double-click or Enter to reset" onpointerdown={startDrag} onkeydown={resizeKey} ondblclick={() => prefs.setRailWidth(RAIL_DEFAULT)}></div>

  {#if !app.readOnly}
    <div class="rail__foot">
      <button class="btn btn--sm" type="button" onclick={() => (ui.pickerOpen = true)}><FolderOpen class="icon icon--sm" />Open folder or file</button>
    </div>
  {/if}
</aside>
