<script lang="ts">
  import Copy from '@lucide/svelte/icons/copy';
  import Prose from './Prose.svelte';
  import Toc from './Toc.svelte';
  import PlanBoard from './PlanBoard.svelte';
  import { copyText, readMinutes, relativeTime, stamp, str } from '../lib/format';
  import { ui } from '../lib/ui.svelte';
  import { toast } from '../lib/toast.svelte';
  import { prefs } from '../lib/prefs.svelte';
  import { app } from '../lib/app.svelte';
  import { news } from '../lib/news.svelte';
  import type { PageData } from '../lib/types';

  let { page }: { page: PageData } = $props();
  const showToc = $derived(!prefs.focus && prefs.toc);
  let wide = $state(false); // a table is wider than the text: the page gives it the full width

  const meta = $derived(page.meta ?? {});
  const id = $derived(str(meta.id));
  const status = $derived(str(meta.status));
  const created = $derived(str(meta.date) || str(meta.created));
  const updated = $derived(str(meta.updated));
  const tags = $derived(Array.isArray(meta.tags) ? (meta.tags as unknown[]).map(str).filter(Boolean).slice(0, 6) : []);
  const lede = $derived(str(meta.description));
  const release = $derived(str(meta.release));
  const versions = $derived(str(meta.versions));
  const verified = $derived(str(meta.verified));
  const author = $derived(str(meta.author));
  const tagBase = $derived(`/${encodeURIComponent(page.source)}/_tags/`);
  // the folder the page lives in, as a small label above the title
  const eyebrow = $derived(page.crumbs.slice(1, -1).map((c) => c.name).slice(-2).join(' / '));

  // reading a note marks it seen
  $effect(() => {
    news.open(page.source, id);
  });

  async function copyId() {
    toast.show((await copyText(id)) ? `${id} copied` : 'Could not copy');
  }

  // the dates with their time of day in this browser's time zone, when the file's clock recorded it
  const createdStamp = $derived(stamp(created, page.createdAt));
  const updatedStamp = $derived(updated || page.updatedAt ? stamp(updated, page.updatedAt) : { text: relativeTime(page.modified), title: '' });
  // a status written as a sentence gets a full-width line of its own under the chips, wrapped, instead of running off the page
  const longStatus = $derived(status.length > 48);

  // once the title scrolls under the top bar, the top bar shows it and the ID
  let heading: HTMLElement | undefined = $state();
  $effect(() => {
    if (!heading) return;
    const bar = parseFloat(getComputedStyle(document.documentElement).getPropertyValue('--topbar-h')) || 56;
    const obs = new IntersectionObserver(([e]) => (ui.titleOut = !!e && !e.isIntersecting && e.boundingClientRect.top < bar), { rootMargin: `-${bar}px 0px 0px 0px` });
    obs.observe(heading);
    return () => {
      obs.disconnect();
      ui.titleOut = false;
    };
  });
</script>

<div class="reader" class:reader--wide={wide} class:reader--notoc={!showToc}>
  <article>
    <header class="pagehead">
      {#if eyebrow}<div class="pagehead__eyebrow">{eyebrow}</div>{/if}
      <h1 bind:this={heading}>{page.title}</h1>
      <div class="meta">
        {#if id}<button class="chip chip--id" type="button" title="Copy ID" onclick={copyId}><Copy class="icon icon--sm" />{id}</button>{/if}
        {#if page.place}
          <span class="chip">{page.place.group === 'features' ? 'Feature' : page.place.group === 'technologies' ? 'Technology' : 'Service'} <b>{page.place.service}</b></span>
          <span class="chip">Type <b>{page.place.type}</b></span>
        {/if}
        {#if status}<span class="chip" class:chip--longstatus={longStatus}><span class="dot"></span><b>{status}</b></span>{/if}
        {#if release}<span class="chip">Release <b>{release}</b></span>{/if}
        {#if created}<span class="chip" title={createdStamp.title || undefined}>Created <b>{createdStamp.text}</b></span>{/if}
        <span class="chip" title={updatedStamp.title || undefined}>Updated <b>{updatedStamp.text}</b></span>
        {#if versions}<span class="chip" title={versions}>Versions <b>{versions.length > 28 ? versions.slice(0, 28) + '…' : versions}</b></span>{/if}
        {#if verified}<span class="chip">Verified <b>{verified}</b></span>{/if}
        {#if author}<span class="chip">{author}</span>{/if}
        {#each [...new Set(tags)] as t (t)}<a class="chip" href={tagBase + encodeURIComponent(t)} title="Notes tagged {t}">{t}</a>{/each}
        <span class="chip">{readMinutes(page.words)} min read</span>
      </div>
      {#if lede}<p class="pagehead__lede">{lede}</p>{/if}
    </header>

    {#if app.lib(page.source)?.kind === 'dropped'}
      <div class="callout callout--note" role="status" style="margin-bottom: var(--space-5)">
        <p class="callout__title">A temporary copy</p>
        <p>This is a copy of the file you dropped. Changes to the original are not shown here, and the copy is deleted when Docent quits.</p>
      </div>
    {/if}

    {#each page.notices ?? [] as n (n.title)}
      <div class="callout callout--{n.kind === 'warning' ? 'warn' : 'note'}" role="status" style="margin-bottom: var(--space-5)">
        <p class="callout__title">{n.title}</p>
        {#if n.text}<p>{n.text}</p>{/if}
        {#if n.items?.length}<ul style="margin: 0.3em 0 0; padding-left: 1.2em">{#each n.items as it (it)}<li><code>{it}</code></li>{/each}</ul>{/if}
      </div>
    {/each}

    {#if page.warnings?.length}
      <div class="callout callout--warn" role="status" style="margin-bottom: var(--space-5)">
        <p class="callout__title">This page has a problem</p>
        {#each page.warnings as w (w)}<p>{w}</p>{/each}
      </div>
    {/if}

    {#if page.plan}<PlanBoard board={page.plan} source={page.source} path={page.path} />{/if}

    <Prose html={page.html} page={{ source: page.source, path: page.path }} onwide={(w) => (wide = w)} />

    {#if page.backlinks?.length}
      <aside class="related" aria-label="Referenced by">
        <h2>Referenced by</h2>
        <div class="cards">
          {#each page.backlinks as b (b.path)}
            <a class="card" href={b.url}><span class="card__title">{b.title}</span><span class="card__meta">{b.path}</span></a>
          {/each}
        </div>
      </aside>
    {/if}

    {#if page.prev || page.next}
      <div class="pager">
        {#if page.prev}<a class="card" href={page.prev.url}><span class="card__meta">Previous</span><span class="card__title">← {page.prev.title}</span></a>{:else}<span></span>{/if}
        {#if page.next}<a class="card" href={page.next.url}><span class="card__meta">Next</span><span class="card__title">{page.next.title} →</span></a>{/if}
      </div>
    {/if}
  </article>

  {#if showToc}<Toc toc={page.toc} />{/if}
</div>
