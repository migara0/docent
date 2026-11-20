<script lang="ts">
  import { toast } from '../lib/toast.svelte';
</script>

<div class="toast" class:is-on={toast.on} class:toast--action={!!toast.action} role="status" aria-live="polite">
  <span>{toast.message}</span>
  {#if toast.action && toast.on}
    <button class="toast__action" type="button" onclick={() => { const a = toast.action; toast.hide(); a?.run(); }}>{toast.action.label}</button>
  {/if}
</div>
