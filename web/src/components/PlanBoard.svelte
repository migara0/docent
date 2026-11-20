<script lang="ts">
  import CircleCheck from '@lucide/svelte/icons/circle-check';
  import CirclePlay from '@lucide/svelte/icons/circle-play';
  import CirclePause from '@lucide/svelte/icons/circle-pause';
  import Circle from '@lucide/svelte/icons/circle';
  import Minus from '@lucide/svelte/icons/minus';
  import Hourglass from '@lucide/svelte/icons/hourglass';
  import Check from '@lucide/svelte/icons/check';
  import Play from '@lucide/svelte/icons/play';
  import { api } from '../lib/api';
  import { app } from '../lib/app.svelte';
  import { toast } from '../lib/toast.svelte';
  import type { PlanBoard, PlanEdit, PlanStage, PlanStep, StepState } from '../lib/types';

  let { board, source, path }: { board: PlanBoard; source: string; path: string } = $props();

  // What this component has just written is shown at once; when the page reloads with the file's own data, that wins again.
  let live = $state<PlanBoard | null>(null);
  $effect(() => {
    void board;
    live = null;
  });
  const b = $derived(live ?? board);

  const editable = $derived(!app.readOnly);
  const KEY = 'docent.planWrite';
  let allowed = $state(false);
  try {
    allowed = localStorage.getItem(KEY) === '1';
  } catch {
    /* no storage: ask every time */
  }
  let pending = $state<PlanEdit | null>(null);
  let busy = $state(false);
  let allowBtn: HTMLButtonElement | undefined = $state();
  $effect(() => {
    if (pending) allowBtn?.focus();
  });

  async function run(edit: PlanEdit) {
    busy = true;
    try {
      live = await api.editPlan(edit);
    } catch (e) {
      toast.show(e instanceof Error ? e.message : 'Could not save the plan', 4000);
    } finally {
      busy = false;
    }
  }
  function request(edit: PlanEdit) {
    if (busy) return;
    if (!allowed) {
      pending = edit;
      return;
    }
    void run(edit);
  }
  function allow() {
    try {
      localStorage.setItem(KEY, '1');
    } catch {
      /* fine */
    }
    allowed = true;
    const e = pending;
    pending = null;
    if (e) void run(e);
  }

  const ref = (st: PlanStage, s?: PlanStep) => (s ? `${st.name}/${s.step}` : `${st.name}/`);
  const setState = (st: PlanStage, s: PlanStep | undefined, state: StepState) => request({ source, path, op: 'state', ref: ref(st, s), state });
  const resolveWait = (index: number) => request({ source, path, op: 'resolve', index });

  // the strip shows every stage in one row; the steps of one stage are open below it (the current one, until the reader picks another)
  let picked = $state<string | null>(null);
  const shown = $derived(b.stages.find((x) => x.name === picked) ?? b.stages.find((x) => x.current) ?? b.stages[b.stages.length - 1]);

  const stateLabel: Record<string, string> = { todo: 'to do', now: 'in progress', done: 'done', blocked: 'blocked', skipped: 'skipped' };
  const stageLabel: Record<string, string> = { todo: 'to do', active: 'in progress', done: 'done', blocked: 'blocked' };
  const age = (d: number) => (d < 0 ? '' : d === 0 ? 'today' : d === 1 ? '1 day' : `${d} days`);
  // a stage without steps is one row of its own
  const rows = (st: PlanStage): PlanStep[] =>
    st.steps.length ? st.steps : [{ step: st.name, state: st.state === 'done' ? 'done' : st.state === 'active' ? 'now' : st.state === 'blocked' ? 'blocked' : 'todo' }];
</script>

<section class="plan" aria-label="Plan status">
  <div class="plan__top">
    <div
      class="plan__bar"
      role="progressbar"
      aria-label="Plan progress"
      aria-valuemin="0"
      aria-valuemax="100"
      aria-valuenow={b.percent}
      aria-valuetext="{b.done} of {b.total} steps done"
    >
      <span style="width: {b.percent}%"></span>
    </div>
    <div class="plan__count"><b>{b.done}</b> of {b.total} steps<span class="plan__pct"> · {b.percent}%</span></div>
  </div>

  {#if pending}
    <div class="plan__confirm" role="group" aria-label="Confirm editing this note">
      <p>Ticking a step changes this note on disk: only the plan block of its front matter and the <code>updated</code> date. Docent asks this once.</p>
      <div class="plan__confirm-actions">
        <button class="btn btn--sm" type="button" bind:this={allowBtn} onclick={allow}>Allow and continue</button>
        <button class="btn btn--ghost btn--sm" type="button" onclick={() => (pending = null)}>Not now</button>
      </div>
    </div>
  {/if}

  <ol class="strip" aria-label="Stages">
    {#each b.stages as st (st.name)}
      <li>
        <button
          class="chip-stage chip-stage--{st.state}"
          class:chip-stage--current={st.current}
          class:chip-stage--picked={shown?.name === st.name}
          type="button"
          aria-pressed={shown?.name === st.name}
          aria-current={st.current ? 'step' : undefined}
          onclick={() => (picked = st.name)}
        >
          <span class="chip-stage__icon chip-stage__icon--{st.state}" aria-hidden="true">
            {#if st.state === 'done'}<CircleCheck class="icon" />{:else if st.state === 'active'}<CirclePlay class="icon" />{:else if st.state === 'blocked'}<CirclePause class="icon" />{:else}<Circle class="icon" />{/if}
          </span>
          <span class="chip-stage__name">{st.name}</span>
          <span class="chip-stage__count">{#if st.total > 1}{st.done}/{st.total}{:else if st.state === 'blocked'}waiting{/if}</span>
          <span class="sr-only">: {stageLabel[st.state]}{#if st.total > 1}, {st.done} of {st.total} steps{/if}</span>
        </button>
      </li>
    {/each}
  </ol>

  {#if shown}
    <div class="detail detail--{shown.state}" aria-label="Steps of {shown.name}" role="group">
      <div class="detail__head">
        <span class="detail__name">{shown.name}</span>
        <span class="detail__state">{stageLabel[shown.state]}{#if shown.total > 1} · {shown.done} of {shown.total}{/if}{#if shown.state === 'blocked' && shown.waiting} · held up by something below{/if}</span>
      </div>
      <ul class="steps">
        {#each rows(shown) as s (s.step)}
          <li class="step step--{s.state}">
            {#if editable}
              <button
                class="step__check"
                type="button"
                disabled={busy}
                aria-label="{s.state === 'done' ? 'Mark as to do' : 'Mark as done'}: {s.step} ({stateLabel[s.state]})"
                onclick={() => setState(shown, shown.steps.length ? s : undefined, s.state === 'done' ? 'todo' : 'done')}
              >
                {@render stepIcon(s.state)}
              </button>
            {:else}
              <span class="step__check step__check--static">{@render stepIcon(s.state)}<span class="sr-only">{stateLabel[s.state]}:</span></span>
            {/if}
            <span class="step__text">{s.step}</span>
            {#if editable && (s.state === 'todo' || s.state === 'blocked')}
              <button class="step__start" type="button" disabled={busy} aria-label="Start {s.step}" title="Start this step" onclick={() => setState(shown, shown.steps.length ? s : undefined, 'now')}>
                <Play class="icon icon--sm" />
              </button>
            {/if}
          </li>
        {/each}
      </ul>
    </div>
  {/if}

  {#if b.now || b.next}
    <div class="plan__nn">
      <div class="nn nn--now"><span class="nn__label">Now</span><span class="nn__text">{b.now || 'Nothing in progress'}</span></div>
      <div class="nn"><span class="nn__label">Next</span><span class="nn__text">{b.next || '—'}</span></div>
    </div>
  {/if}

  {#if b.waiting.length}
    <div class="plan__wait">
      <h2 class="plan__wait-title">Waiting on others <span class="plan__wait-n">{b.waiting.length}</span></h2>
      <ul>
        {#each b.waiting as w, i (i + w.what)}
          <li class="wait" class:wait--old={w.days >= 5}>
            <Hourglass class="icon icon--sm wait__icon" aria-hidden="true" />
            <span class="wait__what">{w.what}</span>
            {#if w.who}<span class="wait__who">{w.who}</span>{/if}
            {#if w.blocks}<span class="wait__holds">holds {w.blocks}</span>{/if}
            {#if age(w.days)}<span class="wait__age">{age(w.days)}</span>{/if}
            {#if editable}
              <button class="wait__done" type="button" disabled={busy} aria-label="No longer waiting: {w.what}" title="No longer waiting" onclick={() => resolveWait(i + 1)}>
                <Check class="icon icon--sm" />
              </button>
            {/if}
          </li>
        {/each}
      </ul>
    </div>
  {/if}
</section>

{#snippet stepIcon(state: StepState)}
  {#if state === 'done'}<CircleCheck class="icon step__ico step__ico--done" />
  {:else if state === 'now'}<CirclePlay class="icon step__ico step__ico--now" />
  {:else if state === 'blocked'}<CirclePause class="icon step__ico step__ico--blocked" />
  {:else if state === 'skipped'}<Minus class="icon step__ico step__ico--skipped" />
  {:else}<Circle class="icon step__ico" />{/if}
{/snippet}

<style>
  .plan { margin: 0 0 var(--space-6); padding: var(--space-4) var(--space-5) var(--space-5); border: 1px solid var(--border); border-radius: var(--radius-lg); background: var(--surface); box-shadow: var(--shadow-sm); }
  .plan__top { display: flex; align-items: center; gap: var(--space-3); }
  .plan__bar { flex: 1; height: 0.5rem; border-radius: var(--radius-full); background: var(--surface-3); overflow: hidden; }
  .plan__bar > span { display: block; height: 100%; border-radius: inherit; background: var(--c-tip); transition: width var(--dur-3) var(--ease); }
  .plan__count { font-size: var(--text-sm); color: var(--text-muted); white-space: nowrap; }
  .plan__count b { color: var(--text); font-weight: 650; }

  .plan__confirm { margin-top: var(--space-3); padding: var(--space-3) var(--space-4); border-radius: var(--radius-md); background: var(--accent-soft); border: 1px solid color-mix(in oklab, var(--accent) 30%, var(--border)); }
  .plan__confirm p { margin: 0 0 var(--space-2); font-size: var(--text-sm); }
  .plan__confirm-actions { display: flex; gap: var(--space-2); }

  .strip { list-style: none; margin: var(--space-4) 0 0; padding: 0; display: grid; grid-template-columns: repeat(auto-fit, minmax(6.75rem, 1fr)); gap: var(--space-2); }
  .chip-stage { display: grid; grid-template-columns: auto minmax(0, 1fr); column-gap: var(--space-2); align-items: center; width: 100%; height: 100%; padding: var(--space-2); border: 1px solid transparent; border-radius: var(--radius-md); background: var(--surface-2); color: var(--text); font: inherit; font-size: var(--text-sm); font-weight: 600; text-align: left; cursor: pointer; }
  .chip-stage:hover { background: var(--surface-3); }
  .chip-stage:focus-visible { outline: 2px solid var(--focus-ring); outline-offset: 2px; }
  .chip-stage--current { border: 1.5px solid var(--accent); background: var(--accent-softer); }
  .chip-stage--blocked { border: 1px dashed color-mix(in oklab, var(--c-warn) 70%, var(--border-strong)); }
  .chip-stage--current.chip-stage--blocked { border-style: dashed; border-color: var(--accent); }
  .chip-stage--picked { box-shadow: 0 0 0 2px var(--surface), 0 0 0 4px var(--border-strong); }
  .chip-stage--done .chip-stage__name { color: var(--text-muted); font-weight: 550; }
  .chip-stage__name { min-width: 0; overflow-wrap: break-word; line-height: 1.25; }
  .chip-stage__count { grid-column: 2; font-size: var(--text-xs); font-weight: 550; color: var(--text-muted); white-space: nowrap; min-height: 1em; }
  .chip-stage__icon { display: inline-flex; grid-row: 1 / span 2; align-self: start; padding-top: 1px; }
  .chip-stage__icon--done { color: var(--c-tip); }
  .chip-stage__icon--active { color: var(--accent); }
  .chip-stage__icon--blocked { color: var(--c-warn); }
  .chip-stage__icon--todo { color: var(--text-faint); }

  .detail { margin-top: var(--space-2); padding: var(--space-3) var(--space-4) var(--space-3); border-radius: var(--radius-md); background: var(--surface-2); }
  .detail__head { display: flex; flex-wrap: wrap; align-items: baseline; gap: var(--space-1) var(--space-3); margin-bottom: var(--space-1); }
  .detail__name { font-weight: 650; font-size: var(--text-ui); }
  .detail__state { font-size: var(--text-xs); color: var(--text-muted); }

  .steps { list-style: none; margin: 0; padding: 0; columns: 13rem 3; column-gap: var(--space-5); }
  .step { display: flex; align-items: flex-start; gap: var(--space-1); padding: 1px 0; font-size: var(--text-sm); line-height: 1.4; break-inside: avoid; }
  .step__check, .step__start, .wait__done { display: inline-flex; align-items: center; justify-content: center; flex: none; padding: 0; border: 0; background: none; color: inherit; border-radius: var(--radius-sm); }
  .step__check { width: 1.4rem; height: 1.4rem; cursor: pointer; }
  .step__check--static { cursor: default; }
  .step__check:hover:not(.step__check--static):not(:disabled), .step__start:hover:not(:disabled), .wait__done:hover:not(:disabled) { background: var(--surface-3); }
  .step__check:focus-visible, .step__start:focus-visible, .wait__done:focus-visible { outline: 2px solid var(--focus-ring); outline-offset: 1px; }
  .step__check:disabled, .step__start:disabled, .wait__done:disabled { cursor: progress; opacity: 0.6; }
  .step__text { flex: 1; min-width: 0; padding-top: 0.2rem; overflow-wrap: anywhere; color: var(--text-muted); }
  :global(.step__ico) { width: 0.95rem; height: 0.95rem; color: var(--text-faint); }
  :global(.step__ico--done) { color: var(--c-tip); }
  :global(.step__ico--now) { color: var(--accent); }
  :global(.step__ico--blocked) { color: var(--c-warn); }
  .step--now .step__text { color: var(--text); font-weight: 650; }
  .step--done .step__text { color: var(--text-faint); }
  .step--skipped .step__text { color: var(--text-faint); text-decoration: line-through; }
  .step__start { width: 1.4rem; height: 1.4rem; color: var(--text-faint); cursor: pointer; opacity: 0; }
  .step:hover .step__start, .step:focus-within .step__start { opacity: 1; }
  @media (hover: none) { .step__start { opacity: 1; } }

  .plan__nn { display: grid; grid-template-columns: repeat(auto-fit, minmax(14rem, 1fr)); gap: var(--space-2); margin-top: var(--space-3); }
  .nn { display: flex; flex-direction: column; gap: 2px; padding: var(--space-2) var(--space-3); border-radius: var(--radius-md); background: var(--surface-2); min-width: 0; }
  .nn--now { background: var(--accent-soft); }
  .nn__label { font-size: var(--text-2xs); font-weight: 650; letter-spacing: 0.04em; text-transform: uppercase; color: var(--text-muted); }
  .nn__text { font-size: var(--text-ui); font-weight: 550; overflow-wrap: anywhere; }

  .plan__wait { margin-top: var(--space-4); }
  .plan__wait-title { margin: 0 0 var(--space-1); padding: 0; border: 0; font-size: var(--text-xs); font-weight: 650; letter-spacing: 0.04em; text-transform: uppercase; color: var(--text-muted); }
  .plan__wait-n { margin-left: var(--space-1); padding: 0 var(--space-2); border-radius: var(--radius-full); background: var(--surface-3); color: var(--text); font-size: var(--text-xs); }
  .plan__wait ul { list-style: none; margin: 0; padding: 0; }
  .wait { display: flex; flex-wrap: wrap; align-items: center; gap: var(--space-1) var(--space-3); padding: var(--space-2) 0; border-top: 1px solid var(--border); font-size: var(--text-sm); }
  :global(.wait__icon) { color: var(--c-warn); flex: none; }
  .wait__what { font-weight: 550; flex: 1 1 12rem; min-width: 0; overflow-wrap: anywhere; }
  .wait__who, .wait__holds { color: var(--text-muted); font-size: var(--text-xs); }
  .wait__age { padding: 1px var(--space-2); border-radius: var(--radius-full); background: var(--surface-3); color: var(--text); font-size: var(--text-xs); white-space: nowrap; }
  .wait--old .wait__age { background: color-mix(in oklab, var(--c-warn) 28%, var(--surface)); }
  .wait__done { width: 1.6rem; height: 1.6rem; color: var(--text-muted); cursor: pointer; }
</style>
