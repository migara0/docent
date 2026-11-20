<script lang="ts">
  import { onMount } from 'svelte';
  import Rail from './components/Rail.svelte';
  import Content from './components/Content.svelte';
  import Sprite from './components/Sprite.svelte';
  import Toast from './components/Toast.svelte';
  import DiagramDialog from './components/DiagramDialog.svelte';
  import Palette from './components/Palette.svelte';
  import Picker from './components/Picker.svelte';
  import PrintDialog from './components/PrintDialog.svelte';
  import ShareDialog from './components/ShareDialog.svelte';
  import Navigator from './components/Navigator.svelte';
  import DropZone from './components/DropZone.svelte';
  import BackToTop from './components/BackToTop.svelte';
  import TableDialog from './components/TableDialog.svelte';
  import { installPrint } from './lib/print';
  import { invalidateSearch } from './lib/search';
  import { app } from './lib/app.svelte';
  import { connectEvents } from './lib/events';
  import { prefs } from './lib/prefs.svelte';
  import { router } from './lib/router.svelte';
  import { ui } from './lib/ui.svelte';
  import { openHelp } from './lib/actions';
  import { news } from './lib/news.svelte';

  const src = $derived(router.route.kind === 'library' ? router.route.src : '');

  onMount(() => {
    installPrint();
    app.refreshLibraries();
    void news.refresh();
    return connectEvents(async (events) => {
      for (const e of events) invalidateSearch(e.source);
      void news.refresh(); // a note may have been added (a new file can arrive empty and get its front matter in a second write, so page events count too)
      if (events.some((e) => e.type === 'source' || e.type === 'tree')) await app.refreshLibraries(); // a note added or removed changes the page counts
      const open = src;
      if (open && events.some((e) => e.source === open && (e.type === 'tree' || e.type === 'source'))) await app.loadTree(open);
      if (events.some((e) => e.source === open)) app.version++;
    });
  });

  // a phone's drawer closes when you move to another page
  $effect(() => {
    router.path;
    ui.navOpen = false;
  });

  function onClick(e: MouseEvent) {
    if (!(e.target as Element | null)?.closest?.('.popover, [data-menu-toggle]')) ui.closeMenus();
    if (e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
    const a = (e.target as Element | null)?.closest?.('a[href]') as HTMLAnchorElement | null;
    if (!a || a.target === '_blank' || a.hasAttribute('download')) return;
    if (a.classList.contains('broken') || a.classList.contains('link-file')) {
      e.preventDefault();
      return;
    }
    const href = a.getAttribute('href') ?? '';
    if (href.startsWith('#') || /^[a-z][a-z0-9+.-]*:/i.test(href)) return; // in-page anchors and other protocols belong to the browser
    if (router.go(href)) e.preventDefault();
  }

  function onKey(e: KeyboardEvent) {
    const typing = /^(INPUT|TEXTAREA|SELECT)$/.test((e.target as HTMLElement | null)?.tagName ?? '') || (e.target as HTMLElement | null)?.isContentEditable;
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
      e.preventDefault();
      ui.paletteOpen = !ui.paletteOpen;
    } else if (e.key === '/' && !typing && !document.querySelector('dialog[open]')) {
      e.preventDefault();
      ui.paletteOpen = true;
    } else if ((e.key === 'b' || e.key === 'B') && !typing && !e.ctrlKey && !e.metaKey && !e.altKey && !document.querySelector('dialog[open]')) {
      e.preventDefault();
      ui.navigatorOpen = true;
    } else if ((e.key === 'n' || e.key === 'N') && !typing && !e.ctrlKey && !e.metaKey && !e.altKey && !document.querySelector('dialog[open]')) {
      e.preventDefault();
      news.openNewest();
    } else if (e.key === '?' && !typing && !e.ctrlKey && !e.metaKey && !document.querySelector('dialog[open]')) {
      e.preventDefault();
      openHelp();
    } else if (e.key === 'Escape') {
      ui.closeMenus();
      ui.navOpen = false;
    }
  }
</script>

<svelte:window onclick={onClick} onkeydown={onKey} />
<svelte:head><title>Docent</title></svelte:head>

<div class="app" class:is-focus={prefs.focus} class:nav-open={ui.navOpen}>
  <div class="scrim" role="presentation" onclick={() => (ui.navOpen = false)}></div>
  <Rail {src} current={app.currentPath} />
  <Content />
</div>

<Palette />
<Picker />
<PrintDialog />
<ShareDialog />
<Navigator />
<DropZone />
<BackToTop />
<Sprite />
<DiagramDialog />
<TableDialog />
<Toast />
