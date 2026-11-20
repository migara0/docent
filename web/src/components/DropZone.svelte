<script lang="ts">
  import FileDown from '@lucide/svelte/icons/file-down';
  import { app } from '../lib/app.svelte';
  import { uploadFiles } from '../lib/actions';
  import { toast } from '../lib/toast.svelte';

  let dragging = $state(false);
  let depth = 0; // dragenter and dragleave fire for every element the pointer crosses

  const hasFiles = (e: DragEvent) => [...(e.dataTransfer?.types ?? [])].includes('Files');

  function enter(e: DragEvent) {
    if (!hasFiles(e)) return;
    e.preventDefault();
    depth++;
    dragging = !app.readOnly;
  }
  function over(e: DragEvent) {
    if (!hasFiles(e)) return;
    e.preventDefault(); // without this the browser would open the file in this tab and the app would be gone
    if (e.dataTransfer) e.dataTransfer.dropEffect = app.readOnly ? 'none' : 'copy';
  }
  function leave(e: DragEvent) {
    if (!hasFiles(e)) return;
    depth = Math.max(0, depth - 1);
    if (!depth) dragging = false;
  }
  function drop(e: DragEvent) {
    if (!hasFiles(e)) return;
    e.preventDefault();
    depth = 0;
    dragging = false;
    if (app.readOnly) {
      toast.show('Dropping files is off while sharing on the network (read-only)', 4000);
      return;
    }
    const items = [...(e.dataTransfer?.items ?? [])];
    const folders = items.some((i) => i.webkitGetAsEntry?.()?.isDirectory);
    void uploadFiles([...(e.dataTransfer?.files ?? [])].filter((f) => f.type !== '' || f.size > 0 || /\.(md|markdown)$/i.test(f.name)), folders);
  }
</script>

<svelte:window ondragenter={enter} ondragover={over} ondragleave={leave} ondrop={drop} />

{#if dragging}
  <div class="dropzone" role="presentation" aria-hidden="true">
    <div class="dropzone__box">
      <FileDown class="icon" style="width: 2rem; height: 2rem; color: var(--accent-strong)" />
      <b>Drop Markdown files to read them</b>
      <span>They open as temporary copies. Your originals are not touched or watched.</span>
    </div>
  </div>
{/if}

<style>
  .dropzone { position: fixed; inset: 0; z-index: 80; display: grid; place-items: center; background: color-mix(in oklab, var(--bg) 70%, transparent); backdrop-filter: blur(3px); pointer-events: none; }
  .dropzone__box { display: grid; gap: var(--space-2); justify-items: center; text-align: center; padding: var(--space-10) var(--space-12); border: 2px dashed var(--accent); border-radius: var(--radius-xl); background: var(--surface); box-shadow: var(--shadow-lg); max-width: min(30rem, calc(100vw - 2rem)); }
  .dropzone__box b { font-size: 1.1rem; }
  .dropzone__box span { color: var(--text-muted); font-size: var(--text-sm); }
</style>
