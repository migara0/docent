<script lang="ts">
  import X from '@lucide/svelte/icons/x';
  import { ui } from '../lib/ui.svelte';
  import { defaultSelection, otherOmissions, printWith, sections, type Section, type Selection } from '../lib/print';

  let dialog: HTMLDialogElement | undefined = $state();
  let list = $state<Section[]>([]);
  let extras = $state<string[]>([]);
  let sel = $state<Selection>({ meta: true, diagrams: true, code: true, includeOmitted: false, sections: {} });
  const KEY = 'docent:print';

  $effect(() => {
    if (ui.printOpen && dialog && !dialog.open) {
      list = sections();
      extras = otherOmissions();
      sel = defaultSelection();
      try {
        // remembered per page: the same choices next time you print this page
        const saved = JSON.parse(localStorage.getItem(`${KEY}:${location.pathname}`) ?? 'null') as Selection | null;
        if (saved) {
          for (const k of ['meta', 'diagrams', 'code', 'includeOmitted'] as const) if (typeof saved[k] === 'boolean') sel[k] = saved[k];
          for (const id of Object.keys(sel.sections)) if (typeof saved.sections?.[id] === 'boolean') sel.sections[id] = saved.sections[id]!;
        }
      } catch { /* ignore */ }
      dialog.showModal();
    }
    if (!ui.printOpen && dialog?.open) dialog.close();
  });

  function go() {
    try { localStorage.setItem(`${KEY}:${location.pathname}`, JSON.stringify(sel)); } catch { /* ignore */ }
    ui.printOpen = false;
    setTimeout(() => printWith($state.snapshot(sel)), 60); // let the dialog finish closing so it is not on the paper
  }
</script>

<dialog bind:this={dialog} aria-label="Choose what to print" onclose={() => { if (!dialog?.open) ui.printOpen = false; }} onclick={(e) => { if (e.target === dialog) ui.printOpen = false; }}>
  <div class="sheet" style="width: min(34rem, calc(100vw - 2rem))">
    <div class="sheet__head">
      <h2>Choose what to print</h2>
      <button class="btn btn--ghost btn--icon btn--sm" type="button" aria-label="Close" onclick={() => (ui.printOpen = false)}><X class="icon" /></button>
    </div>
    <div class="sheet__body" style="max-height: 60vh; overflow: auto">
      <fieldset class="printset"><legend>Page</legend>
        <label><input type="checkbox" bind:checked={sel.meta} /> Details under the title (ID, dates, tags)</label>
        <label><input type="checkbox" bind:checked={sel.diagrams} /> Diagrams</label>
        <label><input type="checkbox" bind:checked={sel.code} /> Code blocks</label>
      </fieldset>
      <fieldset class="printset"><legend>Sections</legend>
        {#each list as s (s.id)}
          <label><input type="checkbox" bind:checked={sel.sections[s.id]} /> {s.title}{#if !s.byDefault} <em>omitted by default</em>{/if}</label>
        {:else}
          <p class="muted">This page has no sections.</p>
        {/each}
      </fieldset>
      {#if extras.length}
        <fieldset class="printset"><legend>Also</legend>
          <label><input type="checkbox" bind:checked={sel.includeOmitted} /> Also print the other parts left out by default <em>{extras.join(', ')}</em></label>
        </fieldset>
      {/if}
    </div>
    <div class="sheet__foot">
      <span class="grow"></span>
      <button class="btn" type="button" onclick={() => (ui.printOpen = false)}>Cancel</button>
      <button class="btn btn--primary" type="button" onclick={go}>Print</button>
    </div>
  </div>
</dialog>

<style>
  .printset { margin: 0 0 var(--space-3); padding: var(--space-2) var(--space-3) var(--space-3); border: 1px solid var(--border); border-radius: var(--radius-md); }
  .printset legend { padding: 0 var(--space-1); font-size: var(--text-2xs); font-weight: 600; letter-spacing: 0.06em; text-transform: uppercase; color: var(--text-faint); }
  .printset label { display: flex; gap: var(--space-2); align-items: baseline; padding: 0.15rem 0; font-size: var(--text-sm); cursor: pointer; }
  .printset em { color: var(--text-faint); font-size: var(--text-xs); }
  .muted { color: var(--text-faint); font-size: var(--text-sm); margin: 0; }
</style>
