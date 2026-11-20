<script lang="ts">
  import ArrowUp from '@lucide/svelte/icons/arrow-up';
  import { prefs } from '../lib/prefs.svelte';

  const SHOW_AFTER = 480; // pixels scrolled before the button appears
  let shown = $state(false);

  function onscroll() {
    shown = scrollY > SHOW_AFTER;
  }

  function top() {
    const calm = matchMedia('(prefers-reduced-motion: reduce)').matches;
    scrollTo({ top: 0, behavior: calm ? 'auto' : 'smooth' });
    // keyboard users land at the top of the page, not on a button that has just disappeared
    document.querySelector<HTMLElement>('.main')?.focus({ preventScroll: true });
  }
</script>

<svelte:window {onscroll} />

{#if shown}
  <button class="totop btn btn--icon" class:is-focus={prefs.focus} type="button" title="Back to top" aria-label="Back to top" onclick={top}><ArrowUp class="icon" /></button>
{/if}

<style>
  .totop { position: fixed; right: var(--space-5); bottom: var(--space-5); z-index: 30; width: 2.75rem; height: 2.75rem; border-radius: var(--radius-full); background: var(--surface); border: 1px solid var(--border-strong); box-shadow: var(--shadow-md); color: var(--text); animation: totop-in var(--dur-2) var(--ease); }
  .totop:hover { background: var(--surface-2); color: var(--accent-strong); }
  @keyframes totop-in { from { opacity: 0; transform: translateY(8px); } }
  @media print { .totop { display: none; } }
  @media (max-width: 760px) { .totop { right: var(--space-4); bottom: var(--space-4); } }
</style>
