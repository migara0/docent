<script lang="ts">
  import Hourglass from '@lucide/svelte/icons/hourglass';
  import { api } from '../lib/api';
  import { app } from '../lib/app.svelte';
  import { router } from '../lib/router.svelte';
  import { relativeTime } from '../lib/format';
  import type { PlanRow } from '../lib/types';

  let plans = $state<PlanRow[]>([]);
  // loads on arrival and again whenever the server reports a change (a plan was ticked or edited)
  $effect(() => {
    void app.version;
    api
      .plans()
      .then((r) => (plans = r.plans))
      .catch(() => {});
  });

  const stageText: Record<string, string> = { todo: 'to do', active: 'in progress', done: 'done', blocked: 'blocked' };
  const go = (e: MouseEvent, url: string) => {
    if (e.metaKey || e.ctrlKey || e.shiftKey || e.button !== 0) return;
    e.preventDefault();
    router.go(url);
  };
</script>

{#if plans.length}
  <section class="plans" aria-labelledby="plans-h">
    <h2 id="plans-h">Active plans <span class="plans__n">{plans.length}</span></h2>
    <ul class="plans__list">
      {#each plans as p (p.source + p.path)}
        {@const b = p.board}
        <li class="plan-row">
          <a class="plan-row__main" href={p.url} onclick={(e) => go(e, p.url)}>
            <span class="plan-row__title">{p.title}</span>
            <span class="plan-row__meta">{#if p.id}<span class="plan-row__id">{p.id}</span>{/if}{p.service}{#if b.updated} · updated {relativeTime(b.updated)}{/if}</span>
          </a>
          <ol class="strip" aria-label="Stages">
            {#each b.stages as st (st.name)}
              <li class="strip__seg strip__seg--{st.state}" class:strip__seg--current={st.current}>
                <span class="strip__bar"></span>
                <span class="strip__name">{st.name}</span>
                <span class="sr-only">: {stageText[st.state]}</span>
              </li>
            {/each}
          </ol>
          <div class="plan-row__now">
            <span class="plan-row__pct"><b>{b.percent}%</b> · {b.done} of {b.total}</span>
            <span class="plan-row__text">{b.now || b.next || 'Not started'}</span>
            {#if b.waiting.length}<span class="plan-row__wait" title="{b.waiting.length} waiting on others"><Hourglass class="icon icon--sm" aria-hidden="true" />{b.waiting.length}<span class="sr-only"> waiting on others</span></span>{/if}
          </div>
        </li>
      {/each}
    </ul>
  </section>
{/if}

<style>
  .plans { margin: 0 0 var(--space-8); }
  .plans h2 { margin: 0 0 var(--space-3); padding: 0; border: 0; font-size: var(--text-reading); }
  .plans__n { margin-left: var(--space-1); padding: 0 var(--space-2); border-radius: var(--radius-full); background: var(--surface-3); font-size: var(--text-xs); font-weight: 600; vertical-align: 0.1em; }
  .plans__list { list-style: none; margin: 0; padding: 0; display: grid; gap: var(--space-2); }
  .plan-row { display: grid; grid-template-columns: minmax(0, 1.1fr) minmax(0, 1.6fr) minmax(0, 1fr); gap: var(--space-4); align-items: center; padding: var(--space-3) var(--space-4); border: 1px solid var(--border); border-radius: var(--radius-md); background: var(--surface); }
  .plan-row:hover { border-color: var(--border-strong); }
  .plan-row__main { display: flex; flex-direction: column; gap: 2px; min-width: 0; color: inherit; text-decoration: none; }
  .plan-row__main:focus-visible { outline: 2px solid var(--focus-ring); outline-offset: 3px; border-radius: var(--radius-sm); }
  .plan-row__title { font-weight: 650; font-size: var(--text-ui); overflow-wrap: anywhere; }
  .plan-row__main:hover .plan-row__title { color: var(--accent-strong); text-decoration: underline; }
  .plan-row__meta { font-size: var(--text-xs); color: var(--text-muted); overflow-wrap: anywhere; }
  .plan-row__id { font-family: var(--font-mono); margin-right: var(--space-2); }

  .strip { list-style: none; margin: 0; padding: 0; display: flex; gap: 3px; min-width: 0; }
  .strip__seg { flex: 1 1 0; min-width: 0; display: flex; flex-direction: column; gap: 3px; }
  .strip__bar { height: 0.4rem; border-radius: var(--radius-full); background: var(--surface-3); }
  .strip__seg--done .strip__bar { background: var(--c-tip); }
  .strip__seg--active .strip__bar { background: var(--accent); }
  .strip__seg--blocked .strip__bar { background: repeating-linear-gradient(45deg, var(--c-warn) 0 4px, color-mix(in oklab, var(--c-warn) 40%, var(--surface)) 4px 8px); }
  .strip__name { font-size: var(--text-2xs); color: var(--text-muted); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .strip__seg--current .strip__name { color: var(--text); font-weight: 650; }

  .plan-row__now { display: flex; flex-wrap: wrap; align-items: baseline; gap: 2px var(--space-3); min-width: 0; font-size: var(--text-sm); }
  .plan-row__pct { color: var(--text-muted); font-size: var(--text-xs); white-space: nowrap; }
  .plan-row__pct b { color: var(--text); }
  .plan-row__text { flex: 1 1 8rem; min-width: 0; font-weight: 550; overflow-wrap: anywhere; }
  .plan-row__wait { display: inline-flex; align-items: center; gap: 3px; padding: 0 var(--space-2); border-radius: var(--radius-full); background: color-mix(in oklab, var(--c-warn) 24%, var(--surface)); font-size: var(--text-xs); font-weight: 600; }
  @media (max-width: 52rem) { .plan-row { grid-template-columns: minmax(0, 1fr); gap: var(--space-2); } }
</style>
