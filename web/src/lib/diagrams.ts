import { resolveColor } from './color';
import { ui } from './ui.svelte';
import { openDiagramWindow } from './diagramWindow';

type Mermaid = (typeof import('mermaid'))['default'];
let loading: Promise<Mermaid> | undefined;
let counter = 0;
let configuredFor = '';

/** Mermaid is large, so it is only fetched when a page actually has a diagram. */
function loadMermaid(): Promise<Mermaid> {
  loading ??= import('mermaid').then((m) => m.default);
  return loading;
}

/** Colours for diagrams, taken from the current design tokens so they match the page in light and dark. */
function themeVariables() {
  const text = resolveColor('var(--text)');
  const bg = resolveColor('var(--bg)');
  const surface = resolveColor('var(--surface)');
  const node = resolveColor('color-mix(in oklab, var(--accent) 11%, var(--surface))');
  const accent = resolveColor('color-mix(in oklab, var(--accent) 55%, var(--surface))');
  const line = resolveColor('var(--text-faint)');
  const muted = resolveColor('var(--text-muted)');
  const border = resolveColor('var(--border-strong)');
  const soft = resolveColor('var(--surface-2)');
  return {
    background: bg, mainBkg: node, primaryColor: node, primaryBorderColor: accent, primaryTextColor: text, nodeBorder: accent, nodeTextColor: text,
    secondaryColor: soft, secondaryBorderColor: border, secondaryTextColor: text, tertiaryColor: surface, tertiaryBorderColor: border, tertiaryTextColor: text,
    lineColor: line, textColor: text, titleColor: text, edgeLabelBackground: bg, clusterBkg: soft, clusterBorder: border,
    noteBkgColor: soft, noteTextColor: text, noteBorderColor: border, actorBkg: node, actorBorder: accent, actorTextColor: text, actorLineColor: line,
    signalColor: muted, signalTextColor: text, labelBoxBkgColor: soft, labelBoxBorderColor: border, labelTextColor: text, loopTextColor: text,
    fontFamily: getComputedStyle(document.body).fontFamily, fontSize: '14px',
  };
}

function configure(m: Mermaid, themeKey: string) {
  if (configuredFor === themeKey) return;
  m.initialize({ startOnLoad: false, securityLevel: 'strict', theme: 'base', themeVariables: themeVariables(), flowchart: { htmlLabels: true, useMaxWidth: true }, sequence: { useMaxWidth: true } });
  configuredFor = themeKey;
}

const TOOL_ICONS = {
  enlarge: '<svg class="icon icon--sm" viewBox="0 0 24 24"><path d="M8 3H5a2 2 0 0 0-2 2v3"/><path d="M21 8V5a2 2 0 0 0-2-2h-3"/><path d="M3 16v3a2 2 0 0 0 2 2h3"/><path d="M16 21h3a2 2 0 0 0 2-2v-3"/></svg>',
  window: '<svg class="icon icon--sm" viewBox="0 0 24 24"><path d="M15 3h6v6"/><path d="M10 14 21 3"/><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"/></svg>',
};

function tool(kind: 'enlarge' | 'window', label: string, onClick: () => void): HTMLButtonElement {
  const b = document.createElement('button');
  b.type = 'button';
  b.className = 'btn btn--ghost btn--icon btn--sm';
  b.title = label;
  b.setAttribute('aria-label', label);
  b.innerHTML = TOOL_ICONS[kind];
  b.addEventListener('click', (e) => {
    e.stopPropagation();
    onClick();
  });
  return b;
}

/** Wraps a <pre class="mermaid"> in the diagram frame (tools row plus a body that will hold the drawing). */
function frame(pre: HTMLElement): { fig: HTMLElement; body: HTMLElement; source: string } {
  const source = pre.textContent ?? '';
  const fig = document.createElement('figure');
  fig.className = 'diagram';
  fig.dataset.source = source;
  const tools = document.createElement('div');
  tools.className = 'diagram__tools';
  const body = document.createElement('div');
  body.className = 'diagram__body';
  body.innerHTML = '<div class="diagram__loading">Drawing diagram…</div>';
  const title = () => document.title.replace(/ · Docent$/, '') || 'Diagram';
  const open = () => {
    const svg = body.querySelector('svg');
    if (svg) ui.diagram = { svg: svg.outerHTML, title: title() };
  };
  tools.append(
    tool('enlarge', 'Enlarge', open),
    tool('window', 'Open in a new window', () => {
      const svg = body.querySelector('svg');
      if (svg) openDiagramWindow(svg.outerHTML, title());
    }),
  );
  body.addEventListener('click', () => {
    if (!String(getSelection())) open();
  });
  fig.append(tools, body);
  pre.replaceWith(fig);
  return { fig, body, source };
}

async function draw(m: Mermaid, body: HTMLElement, source: string) {
  const id = `docent-mmd-${++counter}`;
  try {
    const { svg } = await m.render(id, source);
    body.innerHTML = svg;
  } catch (e) {
    const msg = e instanceof Error ? e.message : String(e);
    body.innerHTML = '';
    const box = document.createElement('div');
    box.className = 'diagram__error';
    box.innerHTML = '<b>This diagram could not be drawn.</b>';
    const detail = document.createElement('pre');
    detail.textContent = msg.split('\n').slice(0, 6).join('\n');
    detail.tabIndex = 0; // it scrolls sideways, so it must be reachable from the keyboard
    box.append(detail);
    body.append(box);
  } finally {
    document.getElementById(`d${id}`)?.remove(); // Mermaid leaves a scratch element behind after an error
  }
}

/** Draws every Mermaid block under root, and redraws existing diagrams when the theme changed. */
export async function renderDiagrams(root: HTMLElement, themeKey: string) {
  const fresh = [...root.querySelectorAll<HTMLElement>('pre.mermaid')];
  const existing = [...root.querySelectorAll<HTMLElement>('figure.diagram')];
  if (!fresh.length && !existing.length) return;
  const m = await loadMermaid();
  configure(m, themeKey);
  for (const pre of fresh) {
    const { body, source } = frame(pre);
    await draw(m, body, source);
  }
  for (const fig of existing) {
    if (fig.dataset.theme === themeKey) continue;
    const body = fig.querySelector<HTMLElement>('.diagram__body');
    if (body && fig.dataset.source) await draw(m, body, fig.dataset.source);
  }
  for (const fig of root.querySelectorAll<HTMLElement>('figure.diagram')) fig.dataset.theme = themeKey;
}
