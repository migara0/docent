<script lang="ts">
  import { ui } from '../lib/ui.svelte';
  import { toast } from '../lib/toast.svelte';
  import { copyText } from '../lib/format';
  import { cellsOf, toMarkdown } from '../lib/tables';

  let dialog: HTMLDialogElement | undefined = $state();
  let body: HTMLDivElement | undefined = $state();

  $effect(() => {
    if (ui.table && dialog && !dialog.open) dialog.showModal();
    if (!ui.table && dialog?.open) dialog.close();
  });

  async function copyMarkdown() {
    const table = body?.querySelector('table');
    if (!table) return;
    toast.show((await copyText(toMarkdown(cellsOf(table)))) ? 'Table copied as Markdown' : 'Could not copy');
  }
</script>

<dialog class="table-zoom" bind:this={dialog} aria-label={ui.table ? `Table: ${ui.table.title}` : 'Table'} onclose={() => { if (!dialog?.open) ui.table = null; }}
  onclick={(e) => { if (e.target === dialog) ui.table = null; }}>
  {#if ui.table}
    <div class="table-zoom__bar">
      <span class="table-zoom__title">{ui.table.title}</span>
      <button class="btn btn--sm" type="button" onclick={copyMarkdown}>Copy as Markdown</button>
      <button class="btn btn--sm" type="button" onclick={() => (ui.table = null)}>Close</button>
    </div>
    <!-- a region that scrolls must be reachable from the keyboard -->
    <!-- svelte-ignore a11y_no_noninteractive_tabindex -->
    <!-- the HTML is the table already on the page, which the server has sanitised -->
    <div class="table-zoom__body" bind:this={body} tabindex="0" role="region" aria-label="The table, scrolls in both directions"><div class="table table--zoom is-scroll">{@html ui.table.html}</div></div>
  {/if}
</dialog>

<style>
  dialog.table-zoom { width: min(96vw, 120rem); max-height: 92vh; max-width: none; padding: 0; border: 0; border-radius: var(--radius-lg); overflow: hidden; color: var(--text); background: var(--bg); box-shadow: var(--shadow-lg); }
  dialog.table-zoom[open] { display: flex; flex-direction: column; }
  .table-zoom__bar { display: flex; gap: var(--space-2); align-items: center; padding: var(--space-2) var(--space-3); border-bottom: 1px solid var(--border); }
  .table-zoom__title { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; font-weight: 650; }
  .table-zoom__body { flex: 1; min-height: 0; overflow: auto; padding: var(--space-4); }
  .table-zoom__body:focus-visible { outline: 2px solid var(--focus-ring); outline-offset: -2px; }
</style>
