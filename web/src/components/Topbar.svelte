<script lang="ts">
  import Menu from '@lucide/svelte/icons/menu';
  import Search from '@lucide/svelte/icons/search';
  import Copy from '@lucide/svelte/icons/copy';
  import Download from '@lucide/svelte/icons/download';
  import Printer from '@lucide/svelte/icons/printer';
  import Pencil from '@lucide/svelte/icons/pencil';
  import Focus from '@lucide/svelte/icons/focus';
  import TableOfContents from '@lucide/svelte/icons/table-of-contents';
  import Type from '@lucide/svelte/icons/type';
  import Sun from '@lucide/svelte/icons/sun';
  import Moon from '@lucide/svelte/icons/moon';
  import Monitor from '@lucide/svelte/icons/monitor';
  import Smartphone from '@lucide/svelte/icons/smartphone';
  import CircleHelp from '@lucide/svelte/icons/circle-question-mark';
  import FolderTree from '@lucide/svelte/icons/folder-tree';
  import ChevronRight from '@lucide/svelte/icons/chevron-right';
  import ReadingPopover from './ReadingPopover.svelte';
  import { prefs } from '../lib/prefs.svelte';
  import { ui } from '../lib/ui.svelte';
  import { app } from '../lib/app.svelte';
  import { copyMarkdown, downloadMarkdown, editorUrl, openHelp } from '../lib/actions';
  import { copyText, str } from '../lib/format';
  import { toast } from '../lib/toast.svelte';
  import type { Crumb, PageData } from '../lib/types';

  let { crumbs, page }: { crumbs: Crumb[]; page?: PageData } = $props();
  let scrolled = $state(false);
  const edit = $derived(page ? editorUrl(page.file, prefs.editor) : undefined);
  const pageId = $derived(str(page?.meta?.id));

  async function copyId() {
    toast.show((await copyText(pageId)) ? `${pageId} copied` : 'Could not copy');
  }
</script>

<svelte:window onscroll={() => (scrolled = scrollY > 4)} />

<header class="topbar" class:is-scrolled={scrolled}>
  <button class="btn btn--ghost btn--icon btn--sm only-mobile" type="button" aria-label="Open navigation" onclick={() => (ui.navOpen = true)}><Menu class="icon" /></button>

  {#if page && ui.titleOut}
    <!-- the page's title has scrolled away: keep it, and the note ID, in sight -->
    <div class="pagetitle">
      {#if pageId}<button class="chip chip--id" type="button" title="Copy ID" onclick={copyId}><Copy class="icon icon--sm" />{pageId}</button>{/if}
      <button class="pagetitle__text" type="button" title="Back to the top" onclick={() => scrollTo({ top: 0, behavior: matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth' })}>{page.title}</button>
    </div>
  {:else}
    <nav class="crumbs" aria-label="Breadcrumb">
      {#each crumbs as c, i (i)}
        {#if i > 0}<ChevronRight class="icon icon--sm" />{/if}
        {#if i === crumbs.length - 1}<span aria-current="page">{c.name}</span>
        {:else if c.url}<a href={c.url}>{c.name}</a>
        {:else}<span>{c.name}</span>{/if}
      {/each}
    </nav>
  {/if}

  <div class="topbar__actions">
    <button class="btn btn--ghost btn--icon btn--sm only-mobile" type="button" aria-label="Search" onclick={() => (ui.paletteOpen = true)}><Search class="icon" /></button>
    {#if page}
      <button class="btn btn--ghost btn--icon btn--sm hide-mobile" type="button" title="Copy page as Markdown" aria-label="Copy page as Markdown" onclick={() => copyMarkdown(page)}><Copy class="icon" /></button>
      <button class="btn btn--ghost btn--icon btn--sm hide-mobile" type="button" title="Download as Markdown" aria-label="Download as Markdown" onclick={() => downloadMarkdown(page)}><Download class="icon" /></button>
      <button class="btn btn--ghost btn--icon btn--sm hide-mobile" type="button" title="Print or save as PDF" aria-label="Print or save as PDF" onclick={() => (ui.printOpen = true)}><Printer class="icon" /></button>
      <button class="btn btn--ghost btn--icon btn--sm hide-mobile" type="button" title={prefs.toc ? 'Hide the contents list (On this page)' : 'Show the contents list (On this page)'} aria-label="Contents list" aria-pressed={prefs.toc} onclick={() => prefs.set('toc', !prefs.toc)}><TableOfContents class="icon" /></button>
      {#if edit}
        <a class="btn btn--ghost btn--icon btn--sm hide-mobile" href={edit} title="Edit in your editor" aria-label="Edit in your editor"><Pencil class="icon" /></a>
      {/if}
    {/if}
    <button class="btn btn--ghost btn--icon btn--sm" type="button" title="Browse files and folders (b)" aria-label="Browse files and folders" onclick={() => (ui.navigatorOpen = true)}><FolderTree class="icon" /></button>
    {#if !app.readOnly}
      <button class="btn btn--ghost btn--icon btn--sm hide-mobile" type="button" title="Open on your phone (QR code)" aria-label="Open on your phone" onclick={() => (ui.shareOpen = true)}><Smartphone class="icon" /></button>
    {/if}
    <button class="btn btn--ghost btn--icon btn--sm hide-mobile" type="button" title="Focus mode (hide the sidebars)" aria-label="Focus mode" aria-pressed={prefs.focus} onclick={() => prefs.set('focus', !prefs.focus)}><Focus class="icon" /></button>
    <span style="position: relative; display: inline-flex">
      <button class="btn btn--ghost btn--icon btn--sm" type="button" title="Reading controls" aria-label="Reading controls" data-menu-toggle aria-haspopup="true" aria-expanded={ui.readingOpen}
        onclick={() => (ui.readingOpen = !ui.readingOpen)}><Type class="icon" /></button>
      {#if ui.readingOpen}<ReadingPopover />{/if}
    </span>
    <button class="btn btn--ghost btn--icon btn--sm hide-mobile" type="button" title="Help: the Docent guide (?)" aria-label="Help: the Docent guide" onclick={openHelp}><CircleHelp class="icon" /></button>
    <button class="btn btn--ghost btn--icon btn--sm" type="button" title="Theme: {prefs.theme}" aria-label="Change theme, now {prefs.theme}" onclick={() => prefs.cycleTheme()}>
      {#if prefs.theme === 'light'}<Sun class="icon" />{:else if prefs.theme === 'dark'}<Moon class="icon" />{:else}<Monitor class="icon" />{/if}
    </button>
  </div>
</header>
