<script lang="ts">
  import X from '@lucide/svelte/icons/x';
  import Smartphone from '@lucide/svelte/icons/smartphone';
  import Copy from '@lucide/svelte/icons/copy';
  import { api } from '../lib/api';
  import { copyText } from '../lib/format';
  import { toast } from '../lib/toast.svelte';
  import { ui } from '../lib/ui.svelte';
  import type { ShareInfo } from '../lib/types';

  let dialog: HTMLDialogElement | undefined = $state();
  let info = $state<ShareInfo | null>(null);
  let error = $state('');
  let busy = $state(false);
  let minutes = $state(60);
  let which = $state(0);
  let path = $state('/');

  $effect(() => {
    if (ui.shareOpen && dialog && !dialog.open) {
      path = location.pathname + location.search; // the page being read now: the phone opens the same page
      error = '';
      info = null;
      which = 0;
      dialog.showModal();
      void load();
    }
    if (!ui.shareOpen && dialog?.open) dialog.close();
  });

  async function load() {
    try {
      info = await api.share();
    } catch (e) {
      error = e instanceof Error ? e.message : 'Could not reach Docent';
    }
  }

  async function set(on: boolean) {
    busy = true;
    error = '';
    try {
      info = await api.setShare(on, minutes);
      which = 0;
    } catch (e) {
      error = e instanceof Error ? e.message : 'Could not change sharing';
    } finally {
      busy = false;
    }
  }

  const base = $derived(info?.active ? (info.urls[which] ?? info.urls[0] ?? '') : '');
  const full = $derived(base ? base + path : '');
  const until = $derived(info?.until ? new Date(info.until).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) : '');
  const port = $derived(base ? new URL(base).port : '');

  async function copy() {
    toast.show((await copyText(full)) ? 'Address copied' : 'Could not copy');
  }
</script>

<dialog bind:this={dialog} aria-label="Open on your phone" onclose={() => { if (!dialog?.open) ui.shareOpen = false; }} onclick={(e) => { if (e.target === dialog) ui.shareOpen = false; }}>
  <div class="sheet" style="width: min(30rem, calc(100vw - 2rem))">
    <div class="sheet__head">
      <Smartphone class="icon" style="color: var(--accent-strong)" />
      <h2>Open on your phone</h2>
      <button class="btn btn--ghost btn--icon btn--sm" type="button" aria-label="Close" onclick={() => (ui.shareOpen = false)}><X class="icon" /></button>
    </div>

    <div class="sheet__body" aria-live="polite">
      {#if error}
        <p class="callout callout--warn" style="padding-left: 1.1em">{error}</p>
      {/if}

      {#if !info && !error}
        <p style="color: var(--text-muted)">Checking…</p>
      {:else if info && !info.available}
        <p>Sharing is controlled from the computer Docent runs on.</p>
      {:else if info && info.active}
        <div class="qr">
          <img src="/_api/share/qr?url={encodeURIComponent(full)}" alt="QR code for {full}" width="220" height="220" />
        </div>
        <p style="margin: var(--space-3) 0 var(--space-2); text-align: center">Scan with your phone's camera. Phone and computer must be on the same network.</p>
        <div class="addr">
          <code>{full}</code>
          <button class="btn btn--sm btn--icon btn--ghost" type="button" aria-label="Copy address" onclick={copy}><Copy class="icon icon--sm" /></button>
        </div>
        {#if info.urls.length > 1}
          <div class="seg" role="group" aria-label="Network address" style="margin-top: var(--space-2)">
            {#each info.urls as u, i (u)}
              <button type="button" aria-pressed={which === i} onclick={() => (which = i)}>{new URL(u).hostname}</button>
            {/each}
          </div>
        {/if}
        <p class="small">Read-only. Sharing stops at <b>{until}</b>, when you press Stop, or when Docent quits. Anyone on your network with this address can read what is open in Docent until then.</p>
        <p class="small">If the phone cannot open it: check it is on the same Wi-Fi (guest networks often block this), and that your computer's firewall allows port {port}.</p>
      {:else if info}
        {#if info.noNetwork}
          <p class="callout callout--warn" style="padding-left: 1.1em">This computer is not connected to a local network (Wi-Fi or cable), so a phone cannot reach it.</p>
        {:else}
          <p>Show a QR code that opens <b>the page you are reading</b> on your phone. Docent will listen on your local network, read-only, until you stop it.</p>
          <p class="small">Anyone on the same network who has the address can read what is open in Docent while it is on. Use it on a network you trust. Nobody can change anything.</p>
          <label class="row" style="margin-top: var(--space-3)">
            <span>Stop sharing after</span>
            <select class="field" style="width: auto" bind:value={minutes}>
              <option value={15}>15 minutes</option>
              <option value={60}>1 hour</option>
              <option value={240}>4 hours</option>
            </select>
          </label>
        {/if}
      {/if}
    </div>

    <div class="sheet__foot">
      <span class="grow"></span>
      {#if info?.active}
        <button class="btn" type="button" disabled={busy} onclick={() => set(false)}>Stop sharing</button>
      {:else if info?.available && !info.noNetwork}
        <button class="btn btn--primary" type="button" disabled={busy} onclick={() => set(true)}>Start sharing</button>
      {/if}
    </div>
  </div>
</dialog>

<style>
  .qr { display: grid; place-items: center; }
  .qr img { background: #fff; padding: 8px; border-radius: var(--radius-md); border: 1px solid var(--border); }
  .addr { display: flex; align-items: center; gap: var(--space-2); justify-content: center; }
  .addr code { overflow-wrap: anywhere; font-size: var(--text-sm); }
  .small { font-size: var(--text-sm); color: var(--text-muted); margin: var(--space-3) 0 0; }
  .row { display: flex; align-items: center; justify-content: space-between; gap: var(--space-3); font-size: var(--text-ui); }
</style>
