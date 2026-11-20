<script lang="ts">
  import X from '@lucide/svelte/icons/x';
  import Folder from '@lucide/svelte/icons/folder';
  import FolderOpen from '@lucide/svelte/icons/folder-open';
  import FileText from '@lucide/svelte/icons/file-text';
  import ChevronRight from '@lucide/svelte/icons/chevron-right';
  import House from '@lucide/svelte/icons/house';
  import Plus from '@lucide/svelte/icons/plus';
  import { api } from '../lib/api';
  import { ui } from '../lib/ui.svelte';
  import { openPath } from '../lib/actions';
  import { baseName, parentOf } from '../lib/paths';
  import type { FsEntry, FsListing } from '../lib/types';

  let dialog: HTMLDialogElement | undefined = $state();
  let listing = $state<FsListing | null>(null);
  let error = $state('');
  let loading = $state(false);
  let selected = $state<FsEntry | null>(null);
  let name = $state('');
  let profile = $state('plain');
  let typed = $state('');
  let userEdited = false; // the path box was typed in since the last listing the user asked for: a slower listing must not overwrite it
  const KEY = 'docent:picker-dir';

  const isFile = $derived(selected?.kind === 'file');
  // what "Open" and "Add" act on: the highlighted entry, or the folder you are looking at
  const target = $derived(selected?.path ?? listing?.dir ?? '');

  let seq = 0; // only the newest request may update the list: an earlier, slower one must not overwrite it
  async function load(dir: string, keep?: string, asked = false) {
    const mine = ++seq;
    loading = true;
    error = '';
    try {
      const l = await api.fs(dir);
      if (mine !== seq) return;
      listing = l;
      if (!userEdited || asked) typed = l.dir;
      if (asked) userEdited = false;
      selected = keep ? (l.entries.find((e) => e.path === keep) ?? null) : null;
      name = selected && selected.kind === 'dir' ? selected.name : (l.crumbs.at(-1)?.name ?? '');
      try { localStorage.setItem(KEY, l.dir); } catch { /* ignore */ }
    } catch (e) {
      if (mine !== seq) return;
      error = e instanceof Error ? e.message : 'Could not read that folder';
    } finally {
      if (mine === seq) loading = false;
    }
  }

  function select(e: FsEntry) {
    selected = e;
    name = e.kind === 'dir' ? e.name : '';
  }
  function enter(e: FsEntry) {
    if (e.kind === 'dir') load(e.path, undefined, true);
  }
  async function goTyped() {
    const p = typed.trim();
    if (!p) return;
    // a file path selects that file inside its folder
    if (/\.(md|markdown)$/i.test(p)) {
      await load(parentOf(p), p, true);
    } else await load(p, undefined, true);
  }

  $effect(() => {
    if (ui.pickerOpen && dialog && !dialog.open) {
      selected = null;
      profile = 'plain';
      userEdited = false;
      dialog.showModal();
      let start = '~';
      try { start = localStorage.getItem(KEY) || '~'; } catch { /* ignore */ }
      load(start).then(() => { if (error && !typed) load('~'); });
    }
    if (!ui.pickerOpen && dialog?.open) dialog.close();
  });

  async function finish(add: boolean) {
    const path = target;
    ui.pickerOpen = false;
    await openPath(path, add ? { name: name.trim() || baseName(path) || 'Library', profile } : undefined);
  }
</script>

<dialog bind:this={dialog} aria-label="Open a folder or file" onclose={() => { if (!dialog?.open) ui.pickerOpen = false; }} onclick={(e) => { if (e.target === dialog) ui.pickerOpen = false; }}>
  <div class="sheet">
    <div class="sheet__head">
      <FolderOpen class="icon" style="color: var(--accent-strong)" />
      <h2>Open a folder or file</h2>
      <button class="btn btn--ghost btn--icon btn--sm" type="button" aria-label="Close" onclick={() => (ui.pickerOpen = false)}><X class="icon" /></button>
    </div>

    <div class="sheet__body">
      <form onsubmit={(e) => { e.preventDefault(); goTyped(); }} style="margin-bottom: var(--space-3)">
        <input class="field" style="font-family: var(--font-mono); font-size: var(--text-sm)" bind:value={typed} oninput={() => (userEdited = true)} aria-label="Folder path" placeholder="/path/to/folder or paste a path" spellcheck="false" />
      </form>

      {#if listing}
        {#if listing.roots?.length}
          <!-- Windows: one button per drive -->
          <div class="picker__path" role="group" aria-label="Drives">
            {#each listing.roots as r (r)}
              <button type="button" aria-pressed={listing.dir.toUpperCase().startsWith(r.toUpperCase())} onclick={() => load(r, undefined, true)}>{r.replace(/\\$/, '')}</button>
            {/each}
          </div>
        {/if}
        <div class="picker__path">
          {#each listing.crumbs as c, i (c.path)}
            {#if i > 0}<span style="color: var(--text-faint)">{listing.sep ?? '/'}</span>{/if}
            <button type="button" onclick={() => load(c.path, undefined, true)}>{#if c.path === listing.home}<House class="icon icon--sm" />{:else}{c.name || '/'}{/if}</button>
          {/each}
        </div>
      {/if}

      <div class="picker__list" role="listbox" aria-label="Folders and Markdown files">
        {#if error}
          <div class="palette__empty" style="color: var(--c-danger)">{error}</div>
        {:else if loading && !listing}
          <div class="palette__empty">Loading…</div>
        {:else if listing && !listing.entries.length}
          <div class="palette__empty">No folders or Markdown files here</div>
        {:else if listing}
          {#each listing.entries as e (e.path)}
            <button class="picker__row" type="button" role="option" aria-selected={selected?.path === e.path} onclick={() => select(e)} ondblclick={() => enter(e)}
              onkeydown={(ev) => { if (ev.key === 'ArrowRight' || (ev.key === 'Enter' && e.kind === 'dir' && ev.ctrlKey)) enter(e); }}>
              {#if e.kind === 'dir'}<Folder class="icon icon--sm" />{:else}<FileText class="icon icon--sm" />{/if}
              <span class="n">{e.name}</span>
              {#if e.kind === 'dir' && e.md}<span class="badge">{e.md}{e.cap ? '+' : ''} .md</span>{/if}
              {#if e.kind === 'dir'}<span role="button" tabindex="-1" aria-label="Open {e.name}" onclick={(ev) => { ev.stopPropagation(); enter(e); }} onkeydown={() => {}} style="display:inline-flex"><ChevronRight class="icon icon--sm" /></span>{/if}
            </button>
          {/each}
        {/if}
      </div>

      {#if !isFile}
        <div class="form">
          <div class="form__row">
            <label>Name <input class="field" bind:value={name} placeholder="Shown in the app" /></label>
            <label>Style
              <select class="field" bind:value={profile}>
                <option value="plain">Plain Markdown</option>
                <option value="devnotes">Developer notes</option>
              </select>
            </label>
          </div>
        </div>
      {/if}
    </div>

    <div class="sheet__foot">
      <span class="grow" style="font-size: var(--text-xs); color: var(--text-faint); overflow: hidden; text-overflow: ellipsis; white-space: nowrap" title={target}>{selected ? target : `Current folder: ${listing?.dir ?? ''}`}</span>
      <button class="btn" type="button" disabled={!target} onclick={() => finish(false)}>Open once</button>
      <button class="btn btn--primary" type="button" disabled={!target || isFile} title={isFile ? 'Add the folder that contains the file instead' : ''} onclick={() => finish(true)}><Plus class="icon icon--sm" />Add to libraries</button>
    </div>
  </div>
</dialog>
