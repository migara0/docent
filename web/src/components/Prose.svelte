<script lang="ts">
  import { tick } from 'svelte';
  import { enhance, layoutTables, resetCodeWrap, syncCode } from '../lib/enhance';
  import { renderDiagrams } from '../lib/diagrams';
  import { prefs } from '../lib/prefs.svelte';

  /** onwide: told whether a table is wider than the text column, so the page can give tables the full width. */
  let { html, page, onwide }: { html: string; page?: { source: string; path: string }; onwide?: (wide: boolean) => void } = $props();
  let el: HTMLDivElement | undefined = $state();

  $effect(() => {
    html; // re-run when the page changes
    if (!el) return;
    tick().then(() => {
      if (!el) return;
      enhance(el, page);
      onwide?.(layoutTables(el));
    });
  });

  // the reading setting for long code lines changed: every block follows it
  $effect(() => {
    void prefs.wrapCode;
    if (el) tick().then(() => el && resetCodeWrap(el));
  });

  // a window that changes size changes which tables fit; fonts arriving change widths too
  $effect(() => {
    if (!el) return;
    let timer: ReturnType<typeof setTimeout> | undefined;
    const again = () => {
      clearTimeout(timer);
      timer = setTimeout(() => {
        if (!el) return;
        onwide?.(layoutTables(el));
        syncCode(el);
      }, 120);
    };
    window.addEventListener('resize', again);
    void document.fonts?.ready.then(again);
    return () => {
      clearTimeout(timer);
      window.removeEventListener('resize', again);
    };
  });

  // diagrams are drawn in the current theme and redrawn when it changes
  $effect(() => {
    html;
    const themeKey = prefs.dark ? 'dark' : 'light';
    if (!el) return;
    tick().then(() => el && renderDiagrams(el, themeKey));
  });
</script>

<div class="prose" bind:this={el}>{@html html}</div>
