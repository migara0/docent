<script lang="ts">
  import Plus from '@lucide/svelte/icons/plus';
  import X from '@lucide/svelte/icons/x';
  import FileText from '@lucide/svelte/icons/file-text';
  import Folder from '@lucide/svelte/icons/folder';
  import { app } from '../lib/app.svelte';
  import { ui } from '../lib/ui.svelte';
  import { router } from '../lib/router.svelte';
  import { initials, relativeTime } from '../lib/format';
  import { openPath, removeLibrary } from '../lib/actions';
  import PlanList from './PlanList.svelte';
  import WhatsNew from './WhatsNew.svelte';
  import { news } from '../lib/news.svelte';
</script>

<div class="home">
  <PlanList />
  <WhatsNew />

  <h1>Your libraries</h1>
  <p class="home__sub">Folders of Markdown you have added. Open any other folder or file whenever you like.</p>

  <div class="cards">
    {#each app.listed as l (l.id)}
      <div class="card libcard" role="link" tabindex="0" onclick={() => router.go(`/${encodeURIComponent(l.id)}/`)} onkeydown={(e) => e.key === 'Enter' && router.go(`/${encodeURIComponent(l.id)}/`)}>
        <div class="libcard__top">
          <span class="libswitch__mark">{initials(l.name)}</span>
          <div style="min-width: 0">
            <div class="card__title">{l.name}</div>
            <div class="libcard__path" title={l.root}>{l.root}</div>
          </div>
        </div>
        {#if l.error}
          <div class="libcard__stats pill-error">{l.error}</div>
        {:else if l.indexing}
          <div class="libcard__stats">Indexing…</div>
        {:else}
          <div class="libcard__stats"><span><b>{l.pages}</b> {l.pages === 1 ? 'page' : 'pages'}</span><span><b>{l.folders}</b> {l.folders === 1 ? 'folder' : 'folders'}</span><span>Updated <b>{relativeTime(l.updated)}</b></span></div>
        {/if}
        <span style="display: flex; gap: var(--space-2); flex-wrap: wrap; justify-self: start">
          <span class="chip">{l.kind === 'library' ? (l.profile === 'devnotes' ? 'Developer notes' : 'Plain Markdown') : l.kind === 'dropped' ? 'Dropped copies' : 'Opened once'}</span>
          {#if news.count(l.id)}<span class="chip chip--new"><span class="newdot" aria-hidden="true"></span>{news.count(l.id)} new</span>{/if}
        </span>
        {#if l.kind === 'library' && !app.readOnly}
          <button class="btn btn--ghost btn--icon btn--sm remove" type="button" title="Remove from libraries" aria-label="Remove {l.name} from libraries"
            onclick={(e) => { e.stopPropagation(); removeLibrary(l.id, l.name); }}><X class="icon icon--sm" /></button>
        {/if}
      </div>
    {/each}

    {#if !app.readOnly}
      <button class="card opencard" type="button" onclick={() => (ui.pickerOpen = true)}>
        <span class="opencard__plus"><Plus class="icon" /></span>
        <b>Open a folder or file</b>
        <span style="font-size: var(--text-sm)">Add it to your libraries, or just open it once. You can also drop .md files anywhere in this window.</span>
      </button>
    {/if}
  </div>

  <p class="home__sub" style="margin-top: var(--space-6)">New here? <a href="/help/guide">Read the Docent guide</a> (or press <kbd>?</kbd> any time).</p>

  {#if app.recents.length}
    <h2>Recently opened</h2>
    <ul class="recent">
      {#each app.recents.slice(0, 8) as r (r.path)}
        <li>
          <a href="/" onclick={(e) => { e.preventDefault(); if (!app.readOnly) openPath(r.path); }}>
            {#if r.kind === 'folder'}<Folder class="icon icon--sm" />{:else}<FileText class="icon icon--sm" />{/if}
            <span class="t">{r.name}</span><span class="m" title={r.path}>{relativeTime(r.openedAt)}</span>
          </a>
        </li>
      {/each}
    </ul>
  {/if}

  {#if app.loaded && !app.listed.length && !app.error}
    <p class="home__sub" style="margin-top: var(--space-8)">Nothing here yet. Add a folder of Markdown with <b>Open a folder or file</b>, or from a terminal: <code>docent add ~/notes</code>.</p>
  {/if}
</div>
