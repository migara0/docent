<script lang="ts">
  import FileText from '@lucide/svelte/icons/file-text';
  import Folder from '@lucide/svelte/icons/folder';
  import Overview from './Overview.svelte';
  import RecentNotes from './RecentNotes.svelte';
  import { app } from '../lib/app.svelte';
  import { folderUrl } from '../lib/router.svelte';
  import type { TreeNode } from '../lib/types';

  let { src, node, title }: { src: string; node: TreeNode; title: string } = $props();
  const children = $derived(node.children ?? []);
  const dev = $derived(app.lib(src)?.profile === 'devnotes');
  // a service, feature or technology folder shows its notes grouped by type; the library root and the features/technologies roots list their folders
  const isGroupRoot = $derived(node.path === '' || node.path === 'features' || node.path === 'technologies');
  const showTables = $derived(dev && !isGroupRoot);
</script>

<div class="home folder">
  <h1>{title}</h1>
  <p class="sub">{node.count ?? 0} {(node.count ?? 0) === 1 ? 'page' : 'pages'}</p>
  {#if showTables}
    <Overview {src} folder={node.path} />
  {:else if children.length}
    <ul class="recent">
      {#each children as c (c.path)}
        <li>
          {#if c.kind === 'folder'}
            <a href={folderUrl(src, c.path)}><Folder class="icon icon--sm" /><span class="t">{c.name}</span><span class="m">{c.count} pages</span></a>
          {:else}
            <a href={c.url}><FileText class="icon icon--sm" /><span class="t">{c.title || c.name}<small>{c.path}</small></span></a>
          {/if}
        </li>
      {/each}
    </ul>
  {:else}
    <p class="sub">There is nothing to show here yet.</p>
  {/if}
</div>

{#if dev && node.path === ''}<RecentNotes {src} />{/if}
