<script lang="ts">
  import type { Heading } from '../lib/types';

  let { toc: tocIn }: { toc: Heading[] | null } = $props();
  const toc = $derived(tocIn ?? []);
  let active = $state('');

  // The active heading is the last one that has scrolled above the reading line (a little below the top bar), or the last one when the page end is reached.
  // Measured on scroll rather than with an observer, so it is right however long the paragraphs between headings are.
  $effect(() => {
    toc;
    const els = toc.map((h) => document.getElementById(h.id)).filter((e): e is HTMLElement => !!e);
    if (!els.length) return;
    const line = 110;
    let frame = 0;
    const compute = () => {
      frame = 0;
      let cur = els[0]!;
      for (const el of els) {
        if (el.getBoundingClientRect().top <= line) cur = el;
        else break;
      }
      if (innerHeight + scrollY >= document.documentElement.scrollHeight - 2) cur = els[els.length - 1]!;
      active = cur.id;
    };
    const schedule = () => {
      if (!frame) frame = requestAnimationFrame(compute);
    };
    compute();
    addEventListener('scroll', schedule, { passive: true });
    addEventListener('resize', schedule);
    return () => {
      removeEventListener('scroll', schedule);
      removeEventListener('resize', schedule);
      cancelAnimationFrame(frame);
    };
  });
</script>

{#if toc.length > 1}
  <nav class="toc" aria-label="On this page">
    <p class="toc__title">On this page</p>
    <ol>
      {#each toc as h (h.id)}
        <li><a href="#{h.id}" class:l3={h.level > 2} class:is-active={h.id === active}>{h.text}</a></li>
      {/each}
    </ol>
  </nav>
{/if}
