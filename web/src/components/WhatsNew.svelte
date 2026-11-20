<script lang="ts">
  import { news } from '../lib/news.svelte';
  import { router } from '../lib/router.svelte';

  const notes = $derived(news.list(8));
  const unseen = $derived(notes.filter((n) => n.unseen).length);
  const go = (e: MouseEvent, url: string) => {
    if (e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return;
    e.preventDefault();
    router.go(url);
  };
  const kind = (n: { type?: string; service?: string }) => [n.type, n.service].filter(Boolean).join(' · ');
</script>

{#if notes.length}
  <section class="news" aria-labelledby="news-h">
    <div class="news__head">
      <h2 id="news-h">What's new {#if news.total}<span class="news__n">{news.total}</span>{/if}</h2>
      {#if news.total}<button class="btn btn--ghost btn--sm" type="button" onclick={() => news.markAll()}>Mark all as seen</button>{/if}
    </div>
    <ul class="news__list">
      {#each notes as n (n.source + n.id)}
        <li class="news__row" class:is-new={n.unseen}>
          <a href={n.url} onclick={(e) => go(e, n.url)}>
            <span class="news__id">{n.id}</span>
            <span class="news__title">{n.title}</span>
            {#if n.unseen}<span class="news__badge">New</span>{/if}
            <span class="news__meta">{kind(n)}{#if n.created} · {n.created}{/if}</span>
          </a>
        </li>
      {/each}
    </ul>
    {#if unseen === 0}<p class="news__none">You have seen all of these.</p>{/if}
  </section>
{/if}

<style>
  .news { margin: 0 0 var(--space-8); }
  .news__head { display: flex; align-items: center; justify-content: space-between; gap: var(--space-3); margin-bottom: var(--space-3); }
  .news h2 { margin: 0; padding: 0; border: 0; font-size: var(--text-reading); }
  .news__n { margin-left: var(--space-1); padding: 0 var(--space-2); border-radius: var(--radius-full); background: var(--accent-soft); color: var(--accent-strong); font-size: var(--text-xs); font-weight: 650; vertical-align: 0.1em; }
  .news__list { list-style: none; margin: 0; padding: 0; border: 1px solid var(--border); border-radius: var(--radius-md); background: var(--surface); overflow: hidden; }
  .news__row + .news__row { border-top: 1px solid var(--border); }
  .news__row a { display: grid; grid-template-columns: 6rem minmax(0, 1fr) auto; grid-template-areas: "id title badge" "id meta meta"; column-gap: var(--space-3); row-gap: 1px; align-items: baseline; padding: var(--space-2) var(--space-4); color: inherit; text-decoration: none; }
  .news__row a:hover { background: var(--surface-2); }
  .news__row a:focus-visible { outline: 2px solid var(--focus-ring); outline-offset: -2px; }
  .news__id { grid-area: id; font-family: var(--font-mono); font-size: var(--text-xs); color: var(--text-muted); }
  .news__title { grid-area: title; font-size: var(--text-ui); overflow-wrap: anywhere; color: var(--text-muted); }
  .news__row.is-new .news__title { color: var(--text); font-weight: 650; }
  .news__badge { grid-area: badge; padding: 0 var(--space-2); border-radius: var(--radius-full); background: var(--accent); color: var(--accent-contrast); font-size: var(--text-2xs); font-weight: 650; letter-spacing: 0.03em; text-transform: uppercase; }
  .news__meta { grid-area: meta; font-size: var(--text-xs); color: var(--text-faint); }
  .news__none { margin: var(--space-2) 0 0; font-size: var(--text-xs); color: var(--text-faint); }
  @media (max-width: 40rem) { .news__row a { grid-template-columns: minmax(0, 1fr) auto; grid-template-areas: "id id" "title badge" "meta meta"; } }
</style>
