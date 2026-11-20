<script lang="ts">
  import { ui } from '../lib/ui.svelte';
  import { openDiagramWindow } from '../lib/diagramWindow';

  let dialog: HTMLDialogElement | undefined = $state();
  let view: HTMLDivElement | undefined = $state();
  let x = $state(0), y = $state(0), scale = $state(1);

  $effect(() => {
    if (ui.diagram && dialog && !dialog.open) {
      x = y = 0;
      scale = 1;
      dialog.showModal();
    }
    if (!ui.diagram && dialog?.open) dialog.close();
  });

  function zoom(factor: number, cx: number, cy: number) {
    const next = Math.min(8, Math.max(0.3, scale * factor)), k = next / scale;
    x = cx - (cx - x) * k;
    y = cy - (cy - y) * k;
    scale = next;
  }
  const centre = (f: number) => {
    const r = view!.getBoundingClientRect();
    zoom(f, r.width / 2, r.height / 2);
  };
  function wheel(e: WheelEvent) {
    e.preventDefault();
    const r = view!.getBoundingClientRect();
    zoom(e.deltaY < 0 ? 1.15 : 1 / 1.15, e.clientX - r.left, e.clientY - r.top);
  }
  let drag: { x: number; y: number } | null = null;
  const down = (e: PointerEvent) => {
    drag = { x: e.clientX - x, y: e.clientY - y };
    view!.setPointerCapture(e.pointerId);
  };
  const move = (e: PointerEvent) => {
    if (drag) {
      x = e.clientX - drag.x;
      y = e.clientY - drag.y;
    }
  };
</script>

<dialog class="diagram-zoom" bind:this={dialog} aria-label="Diagram" onclose={() => { if (!dialog?.open) ui.diagram = null; }}
  onclick={(e) => { if (e.target === dialog) ui.diagram = null; }}>
  {#if ui.diagram}
    <div class="diagram-zoom__bar">
      <button class="btn btn--sm" type="button" title="Open in its own window (search, pinch zoom, save as SVG)" onclick={() => { if (ui.diagram) openDiagramWindow(ui.diagram.svg, ui.diagram.title); ui.diagram = null; }}>New window</button>
      <span class="diagram-zoom__sep"></span>
      <button class="btn btn--sm btn--icon" type="button" aria-label="Zoom in" onclick={() => centre(1.3)}>+</button>
      <button class="btn btn--sm btn--icon" type="button" aria-label="Zoom out" onclick={() => centre(1 / 1.3)}>−</button>
      <button class="btn btn--sm" type="button" onclick={() => { x = y = 0; scale = 1; }}>Fit</button>
      <button class="btn btn--sm" type="button" onclick={() => (ui.diagram = null)}>Close</button>
    </div>
    <div class="diagram-zoom__view" bind:this={view} role="img" aria-label={ui.diagram.title} onwheel={wheel} onpointerdown={down} onpointermove={move} onpointerup={() => (drag = null)} ondblclick={() => { x = y = 0; scale = 1; }}>
      <div class="diagram-zoom__stage" style="transform: translate({x}px, {y}px) scale({scale})">{@html ui.diagram.svg}</div>
    </div>
  {/if}
</dialog>

<style>
  dialog.diagram-zoom { width: min(96vw, 110rem); height: 92vh; max-width: none; max-height: none; padding: 0; border: 0; border-radius: var(--radius-lg); overflow: hidden; color: var(--text); background: var(--bg); box-shadow: var(--shadow-lg); }
  dialog.diagram-zoom[open] { display: flex; flex-direction: column; }
  .diagram-zoom__bar { display: flex; gap: var(--space-2); align-items: center; padding: var(--space-2) var(--space-3); border-bottom: 1px solid var(--border); }
  .diagram-zoom__sep { flex: 1; }
  .diagram-zoom__view { flex: 1; overflow: hidden; cursor: grab; touch-action: none; }
  .diagram-zoom__stage { transform-origin: 0 0; box-sizing: border-box; min-height: 100%; padding: var(--space-4); display: flex; align-items: center; }
  .diagram-zoom__stage :global(svg) { display: block; width: 100%; height: auto; max-width: none !important; }
</style>
