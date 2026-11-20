<script lang="ts">
  import NoteTable from './NoteTable.svelte';
  import { api } from '../lib/api';
  import { app } from '../lib/app.svelte';
  import type { TagCount } from '../lib/types';

  let { src, tag }: { src: string; tag: string } = $props();
  let tags = $state<TagCount[]>([]);
  let loaded = $state(false);

  $effect(() => {
    app.version;
    api.tags(src).then((r) => { tags = r.tags; loaded = true; }).catch(() => (loaded = true));
  });
  const current = $derived(tags.find((t) => t.name === tag));
  const base = $derived(`/${encodeURIComponent(src)}/_tags/`);
</script>

<div class="home folder">
  <h1>{tag ? `Tag: ${tag}` : 'Tags'}</h1>
  <p class="sub">{tag ? `${current?.count ?? 0} ${current?.count === 1 ? 'note' : 'notes'}` : `${tags.length} tags`}</p>

  <div class="meta" style="margin-bottom: var(--space-6)">
    {#each tags as t (t.name)}
      <a class="chip" href={base + encodeURIComponent(t.name)} aria-current={t.name === tag ? 'true' : undefined} style={t.name === tag ? 'background: var(--accent-soft); color: var(--accent-strong); border-color: transparent' : ''}>{t.name} <b>{t.count}</b></a>
    {/each}
  </div>

  {#if tag}
    {#if current}<NoteTable notes={current.pages} />{:else if loaded}<p class="sub">No notes have this tag.</p>{/if}
  {:else if loaded && !tags.length}
    <p class="sub">No note has a tag yet. Tags are the <code>tags:</code> list in a page's front matter.</p>
  {/if}
</div>
