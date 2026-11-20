<script lang="ts">
  import NoteTable from './NoteTable.svelte';
  import { api } from '../lib/api';
  import { app } from '../lib/app.svelte';
  import type { NoteRow } from '../lib/types';

  let { src }: { src: string } = $props();
  let notes = $state<NoteRow[]>([]);

  $effect(() => {
    app.version;
    api.recent(src, 10).then((r) => (notes = r.notes)).catch(() => {});
  });
</script>

{#if notes.length}
  <aside class="recent-notes" aria-label="Recently updated">
    <h2>Recently updated</h2>
    <NoteTable {notes} withType />
  </aside>
{/if}

<style>
  .recent-notes { max-width: var(--reading-w); margin: var(--space-12) auto 0; padding: 0 var(--space-8); }
  h2 { margin: 0 0 var(--space-3); font-size: var(--text-sm); font-weight: 600; letter-spacing: 0.04em; text-transform: uppercase; color: var(--text-faint); }
</style>
