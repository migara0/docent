<script lang="ts">
  import NoteTable from './NoteTable.svelte';
  import { api } from '../lib/api';
  import { app } from '../lib/app.svelte';
  import { typeLabel, typeRank } from '../lib/noteorder';
  import type { NoteRow } from '../lib/types';

  let { src, folder }: { src: string; folder: string } = $props();
  let notes = $state<NoteRow[]>([]);
  let loaded = $state(false);

  $effect(() => {
    app.version; // reload when notes change
    const f = folder;
    api.overview(src, f).then((r) => { notes = r.notes; loaded = true; }).catch(() => (loaded = true));
  });

  const groups = $derived.by(() => {
    const by = new Map<string, NoteRow[]>();
    for (const n of notes) by.set(n.type ?? '', [...(by.get(n.type ?? '') ?? []), n]);
    return [...by.entries()]
      .sort((a, b) => typeRank(a[0]) - typeRank(b[0]) || a[0].localeCompare(b[0]))
      .map(([type, rows]) => ({ type, rows: [...rows].sort((a, b) => b.updated.localeCompare(a.updated) || b.path.localeCompare(a.path)) }));
  });
</script>

{#if !loaded}
  <div class="skeleton" style="height: 6rem"></div>
{:else if !notes.length}
  <p class="sub">No notes here yet.</p>
{:else}
  {#each groups as g (g.type)}
    <h2>{typeLabel(g.type)} <small style="font-weight: 400; color: var(--text-faint)">{g.rows.length}</small></h2>
    <NoteTable notes={g.rows} />
  {/each}
{/if}

<style>
  h2 { margin: var(--space-8) 0 var(--space-3); font-size: var(--text-sm); font-weight: 600; letter-spacing: 0.04em; text-transform: uppercase; color: var(--text-faint); }
</style>
