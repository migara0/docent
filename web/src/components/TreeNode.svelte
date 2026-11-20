<script lang="ts">
  import TreeNodeView from './TreeNode.svelte';
  import ChevronRight from '@lucide/svelte/icons/chevron-right';
  import Folder from '@lucide/svelte/icons/folder';
  import FolderOpen from '@lucide/svelte/icons/folder-open';
  import FileText from '@lucide/svelte/icons/file-text';
  import type { TreeNode } from '../lib/types';
  import type { TreeState } from '../lib/treestate.svelte';
  import { elideLabels, highlightParts, splitTail } from '../lib/treenav';
  import { news } from '../lib/news.svelte';

  /** labels: the shortened titles for this node's siblings; terms: the words typed in the filter (then every folder is open and the full titles show). */
  let { node, src, state, current, labels = {}, terms = [] }: { node: TreeNode; src: string; state: TreeState; current: string; labels?: Record<string, string>; terms?: string[] } = $props();
  const filtering = $derived(terms.length > 0);
  const open = $derived(filtering || !!state.open[node.path]);
  const childLabels = $derived(filtering ? {} : elideLabels(node.children ?? []));
  const full = $derived(node.title || node.name);
  const shown = $derived(filtering ? full : (labels[node.path] ?? full));
  const cut = $derived(splitTail(shown));
  // a note you have not opened yet gets a dot; a closed folder gets one when something new is inside
  const holdsNew = (n: TreeNode): boolean => (n.kind === 'page' ? news.isNew(src, n.id) : (n.children ?? []).some(holdsNew));
  const isNew = $derived(node.kind === 'page' && news.isNew(src, node.id));
  const folderNew = $derived(node.kind === 'folder' && !open && holdsNew(node));
</script>

<li>
  {#if node.kind === 'folder'}
    <button class="tree__row" type="button" aria-expanded={open} title={node.path} onclick={() => !filtering && state.toggle(node.path)}>
      <ChevronRight class="icon icon--sm chev" />
      {#if open}<FolderOpen class="icon icon--sm" />{:else}<Folder class="icon icon--sm" />{/if}
      <span class="label">{#each highlightParts(node.name, terms) as p, i (i)}{#if p.hit}<mark>{p.text}</mark>{:else}{p.text}{/if}{/each}</span>
      {#if folderNew}<span class="newdot" aria-hidden="true"></span><span class="sr-only">contains new notes</span>{/if}
      <span class="count">{node.count}</span>
    </button>
    {#if open}
      <ul>
        {#each node.children ?? [] as child (child.path)}
          <TreeNodeView node={child} {src} {state} {current} labels={childLabels} {terms} />
        {/each}
      </ul>
    {/if}
  {:else}
    <a href={node.url} aria-current={node.path === current ? 'page' : undefined} data-path={node.path} title={node.id ? `${full} (${node.id})` : full}>
      <FileText class="icon icon--sm" />
      <span class="label label--cut"><span class="head">{#each highlightParts(cut.head, terms) as p, i (i)}{#if p.hit}<mark>{p.text}</mark>{:else}{p.text}{/if}{/each}</span>{#if cut.tail}<span class="tail">{#each highlightParts(cut.tail, terms) as p, i (i)}{#if p.hit}<mark>{p.text}</mark>{:else}{p.text}{/if}{/each}</span>{/if}</span>
      {#if isNew}<span class="newdot" aria-hidden="true"></span><span class="sr-only">new note</span>{/if}
      {#if filtering && node.id}<span class="count mono">{node.id}</span>{/if}
    </a>
  {/if}
</li>
