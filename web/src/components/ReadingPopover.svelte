<script lang="ts">
  import Sun from '@lucide/svelte/icons/sun';
  import Moon from '@lucide/svelte/icons/moon';
  import Monitor from '@lucide/svelte/icons/monitor';
  import { prefs, SCALE_MAX, SCALE_MIN } from '../lib/prefs.svelte';
</script>

<div class="popover" aria-label="Reading" style="position:absolute; right:0; top:calc(100% + 8px)">
  <h3>Reading</h3>
  <div class="row">
    <span>Text size</span>
    <div class="stepper">
      <button class="btn btn--sm btn--icon" type="button" aria-label="A− smaller text" disabled={prefs.scale <= SCALE_MIN} onclick={() => prefs.setScale(prefs.scale - 0.1)}>A−</button>
      <output>{Math.round(prefs.scale * 100)}%</output>
      <button class="btn btn--sm btn--icon" type="button" aria-label="A+ larger text" disabled={prefs.scale >= SCALE_MAX} onclick={() => prefs.setScale(prefs.scale + 0.1)}>A+</button>
    </div>
  </div>
  <div class="row">
    <span>Width</span>
    <div class="seg" role="group" aria-label="Page width">
      {#each [['narrow', 'Narrow'], ['standard', 'Normal'], ['wide', 'Wide']] as [w, label] (w)}
        <button type="button" aria-pressed={prefs.width === w} onclick={() => prefs.set('width', w as 'narrow' | 'standard' | 'wide')}>{label}</button>
      {/each}
    </div>
  </div>
  <div class="row">
    <span>Long code lines</span>
    <div class="seg" role="group" aria-label="Long lines of code">
      <button type="button" aria-pressed={!prefs.wrapCode} onclick={() => prefs.set('wrapCode', false)}>Scroll</button>
      <button type="button" aria-pressed={prefs.wrapCode} onclick={() => prefs.set('wrapCode', true)}>Wrap</button>
    </div>
  </div>
  <div class="row">
    <span>Theme</span>
    <div class="seg" role="group" aria-label="Theme">
      <button type="button" aria-label="Light" aria-pressed={prefs.theme === 'light'} onclick={() => prefs.set('theme', 'light')}><Sun class="icon icon--sm" /></button>
      <button type="button" aria-label="Match system" aria-pressed={prefs.theme === 'auto'} onclick={() => prefs.set('theme', 'auto')}><Monitor class="icon icon--sm" /></button>
      <button type="button" aria-label="Dark" aria-pressed={prefs.theme === 'dark'} onclick={() => prefs.set('theme', 'dark')}><Moon class="icon icon--sm" /></button>
    </div>
  </div>
  <div class="row">
    <span>Edit in</span>
    <div class="seg" role="group" aria-label="Editor for the edit button">
      {#each [['none', 'Off'], ['vscode', 'VS Code'], ['zed', 'Zed']] as [e, label] (e)}
        <button type="button" aria-pressed={prefs.editor === e} onclick={() => prefs.set('editor', e as 'none' | 'vscode' | 'zed')}>{label}</button>
      {/each}
    </div>
  </div>
</div>
