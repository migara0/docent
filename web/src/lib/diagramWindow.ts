import { resolveColor } from './color';
import { toast } from './toast.svelte';

/**
 * Opens a diagram in its own browser window (src/diagram-window.ts). The page is a normal file of this app, so it works under the same
 * Content-Security-Policy; the drawing and the colours travel over postMessage once the new window says it is ready.
 */
export function openDiagramWindow(svg: string, title: string) {
  const win = window.open('/diagram.html', '_blank', 'popup=yes,width=1280,height=860');
  if (!win) {
    toast.show('The browser blocked the pop-up: allow pop-ups for this site', 4000);
    return;
  }
  const payload = {
    type: 'docent-diagram',
    svg,
    title,
    bg: resolveColor('var(--bg)'),
    fg: resolveColor('var(--text)'),
    line: resolveColor('var(--border-strong)'),
    accent: resolveColor('var(--accent)'),
  };
  const onMessage = (e: MessageEvent) => {
    if (e.source !== win || e.origin !== location.origin || (e.data as { type?: string })?.type !== 'docent-diagram-ready') return;
    win.postMessage(payload, location.origin);
    removeEventListener('message', onMessage);
  };
  addEventListener('message', onMessage);
  setTimeout(() => removeEventListener('message', onMessage), 20_000);
}
