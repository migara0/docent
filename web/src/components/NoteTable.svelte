<script lang="ts">
  import { stamp } from '../lib/format';
  import type { NoteRow } from '../lib/types';

  let { notes, withType = false }: { notes: NoteRow[]; withType?: boolean } = $props();
</script>

<div class="table">
  <table>
    <thead><tr><th>Note</th><th>ID</th>{#if withType}<th>Type</th>{/if}<th>Created</th><th>Updated</th><th>Status</th></tr></thead>
    <tbody>
      {#each notes as n (n.path)}
        {@const c = stamp(n.created, n.createdAt)}
        {@const u = stamp(n.updated, n.updatedAt)}
        <tr>
          <td><a href={n.url}>{n.title}</a></td>
          <td>{#if n.id}<code>{n.id}</code>{/if}</td>
          {#if withType}<td>{n.type ?? ''}</td>{/if}
          <td class="when" title={c.title || undefined}>{c.text}</td><td class="when" title={u.title || undefined}>{u.text}</td><td>{n.status ?? ''}</td>
        </tr>
      {/each}
    </tbody>
  </table>
</div>

<style>
  .when { white-space: nowrap; }
</style>
