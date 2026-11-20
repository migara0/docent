<script lang="ts">
  import TreeNodeView from './TreeNode.svelte';
  import { TreeState } from '../lib/treestate.svelte';
  import { elideLabels, termsOf } from '../lib/treenav';
  import type { TreeNode } from '../lib/types';

  /** nodes: the tree to show (already filtered when a filter is typed); filter: what was typed, for highlighting. */
  let { src, nodes, current, filter = '' }: { src: string; nodes: TreeNode[]; current: string; filter?: string } = $props();
  const state = $derived(new TreeState(src));
  const terms = $derived(termsOf(filter));
  const labels = $derived(terms.length ? {} : elideLabels(nodes));

  // keep the page you are reading visible in the tree
  $effect(() => {
    if (current) state.reveal(current);
  });
  $effect(() => {
    if (!current) return;
    queueMicrotask(() => document.querySelector('.tree a[aria-current="page"]')?.scrollIntoView({ block: 'nearest' }));
  });
</script>

<ul class="tree" class:is-filtering={terms.length > 0} aria-label="Pages">
  {#each nodes as child (child.path)}
    <TreeNodeView node={child} {src} {state} {current} {labels} {terms} />
  {/each}
</ul>
