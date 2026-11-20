// The stand-alone diagram viewer that opens in its own window: zoom, pan, pinch, fit modes, search, keyboard, and save as SVG.
import './diagram-window.css';

interface Payload {
  type: 'docent-diagram';
  svg: string;
  title: string;
  bg: string;
  fg: string;
  line: string;
  accent: string;
}

const $ = <T extends HTMLElement>(id: string) => document.getElementById(id) as T;

function start(p: Payload) {
  document.title = `${p.title} · diagram`;
  const root = document.documentElement.style;
  root.setProperty('--bg', p.bg);
  root.setProperty('--fg', p.fg);
  root.setProperty('--line', p.line);
  root.setProperty('--accent', p.accent);
  document.body.innerHTML = `
    <div id="bar">
      <button type="button" data-act="out" title="Zoom out (-)">−</button><span id="zoom">100%</span><button type="button" data-act="in" title="Zoom in (+)">+</button>
      <button type="button" data-act="fit" title="Whole diagram in the window (0)">Fit</button>
      <button type="button" data-act="width" title="Fit to the window width and scroll down (w, or double-click)">Width</button>
      <button type="button" data-act="actual" title="Actual size (1)">100%</button>
      <input id="search" type="search" placeholder="Find (/)" aria-label="Find in diagram" autocomplete="off" />
      <button type="button" data-act="prev" title="Previous match (Shift+Enter)">↑</button><button type="button" data-act="next" title="Next match (Enter)">↓</button><span id="count"></span>
      <span class="sep"></span><button type="button" data-act="save" title="Save as an SVG file">Save SVG</button>
    </div>
    <div id="view"><div id="stage"></div></div>`;
  const view = $('view'), stage = $('stage'), zoomLabel = $('zoom'), search = $<HTMLInputElement>('search'), count = $('count');
  stage.innerHTML = p.svg;
  const svg = stage.querySelector('svg')!;

  const box = (svg.getAttribute('viewBox') ?? '').split(/[ ,]+/).map(Number);
  const w = box[2] || svg.getBoundingClientRect().width || 800;
  const h = box[3] || svg.getBoundingClientRect().height || 600;
  svg.removeAttribute('style');
  svg.setAttribute('width', String(w));
  svg.setAttribute('height', String(h));

  const st = { x: 0, y: 0, s: 1 };
  let touched = false;
  const apply = () => {
    stage.style.transform = `translate(${st.x}px,${st.y}px) scale(${st.s})`;
    zoomLabel.textContent = `${Math.round(st.s * 100)}%`;
  };
  const fit = (mode?: 'contain' | 'width') => {
    const r = view.getBoundingClientRect();
    const byWidth = mode === 'width' || (!mode && h / w > 1.4); // a tall diagram opens at a readable width and scrolls down
    const s = byWidth ? (r.width - 40) / w : Math.min((r.width - 40) / w, (r.height - 40) / h);
    st.s = Math.min(Math.max(s, 0.05), 2);
    st.x = (r.width - w * st.s) / 2;
    st.y = byWidth ? 20 : (r.height - h * st.s) / 2;
    apply();
  };
  const zoomAt = (factor: number, cx: number, cy: number) => {
    touched = true;
    const next = Math.min(12, Math.max(0.05, st.s * factor)), k = next / st.s;
    st.x = cx - (cx - st.x) * k;
    st.y = cy - (cy - st.y) * k;
    st.s = next;
    apply();
  };
  const zoomCentre = (f: number) => {
    const r = view.getBoundingClientRect();
    zoomAt(f, r.width / 2, r.height / 2);
  };
  const actual = () => {
    const r = view.getBoundingClientRect();
    touched = true;
    st.s = 1;
    st.x = (r.width - w) / 2;
    st.y = (r.height - h) / 2;
    apply();
  };

  view.addEventListener('wheel', (e) => {
    e.preventDefault();
    const r = view.getBoundingClientRect();
    zoomAt(e.deltaY < 0 ? 1.15 : 1 / 1.15, e.clientX - r.left, e.clientY - r.top);
  }, { passive: false });
  view.addEventListener('dblclick', () => { touched = false; fit(); });

  // one pointer pans; two pinch (and pan)
  const pointers = new Map<number, { x: number; y: number }>();
  let last: { d: number; cx: number; cy: number } | null = null;
  const pair = () => (pointers.size === 2 ? [...pointers.values()] as [{ x: number; y: number }, { x: number; y: number }] : null);
  view.addEventListener('pointerdown', (e) => {
    pointers.set(e.pointerId, { x: e.clientX, y: e.clientY });
    try { view.setPointerCapture(e.pointerId); } catch { /* synthetic pointers cannot be captured */ }
    view.classList.add('dragging');
    const pr = pair();
    last = pr ? { d: Math.hypot(pr[0].x - pr[1].x, pr[0].y - pr[1].y), cx: (pr[0].x + pr[1].x) / 2, cy: (pr[0].y + pr[1].y) / 2 } : null;
  });
  view.addEventListener('pointermove', (e) => {
    const pt = pointers.get(e.pointerId);
    if (!pt) return;
    const r = view.getBoundingClientRect();
    if (pointers.size === 1) {
      touched = true;
      st.x += e.clientX - pt.x;
      st.y += e.clientY - pt.y;
      apply();
      pt.x = e.clientX;
      pt.y = e.clientY;
      return;
    }
    pt.x = e.clientX;
    pt.y = e.clientY;
    const pr = pair();
    if (!pr || !last) return;
    const d = Math.hypot(pr[0].x - pr[1].x, pr[0].y - pr[1].y), cx = (pr[0].x + pr[1].x) / 2, cy = (pr[0].y + pr[1].y) / 2;
    st.x += cx - last.cx;
    st.y += cy - last.cy;
    if (last.d > 0 && d > 0) zoomAt(d / last.d, cx - r.left, cy - r.top);
    else apply();
    last = { d, cx, cy };
  });
  const release = (e: PointerEvent) => {
    pointers.delete(e.pointerId);
    last = null;
    if (!pointers.size) view.classList.remove('dragging');
  };
  view.addEventListener('pointerup', release);
  view.addEventListener('pointercancel', release);

  // search: outline the components that match, dim the rest
  let groups = [...svg.querySelectorAll<SVGElement>('g.node, g.cluster, g.edgeLabel, g.actor, g.entity, g.label')];
  if (!groups.length) groups = [...svg.querySelectorAll<SVGElement>('text')];
  groups = groups.filter((g) => !groups.some((o) => o !== g && o.contains(g))); // a node and the label inside it count once
  let hits: SVGElement[] = [];
  let at = -1;
  const gotoHit = (step: number) => {
    if (!hits.length) return;
    at = (at + step + hits.length) % hits.length;
    hits.forEach((g) => g.classList.remove('current'));
    const el = hits[at]!, r = el.getBoundingClientRect(), v = view.getBoundingClientRect();
    el.classList.add('current');
    touched = true;
    st.x += v.left + v.width / 2 - (r.left + r.width / 2);
    st.y += v.top + v.height / 2 - (r.top + r.height / 2);
    apply();
    count.textContent = `${at + 1} of ${hits.length}`;
  };
  const runSearch = () => {
    const q = search.value.trim().toLowerCase();
    hits = [];
    at = -1;
    svg.classList.toggle('searching', !!q);
    for (const g of groups) {
      const match = !!q && (g.textContent ?? '').toLowerCase().includes(q);
      g.classList.toggle('hit', match);
      if (match) hits.push(g);
    }
    count.textContent = q ? `${hits.length} found` : '';
    if (hits.length) gotoHit(1);
  };
  search.addEventListener('input', runSearch);
  search.addEventListener('keydown', (e) => {
    if (e.key === 'Enter') { e.preventDefault(); gotoHit(e.shiftKey ? -1 : 1); }
    if (e.key === 'Escape') { search.value = ''; runSearch(); search.blur(); }
  });

  const save = () => {
    const a = document.createElement('a');
    a.href = URL.createObjectURL(new Blob([new XMLSerializer().serializeToString(svg)], { type: 'image/svg+xml' }));
    a.download = `${(p.title.replace(/[^a-z0-9]+/gi, '-').replace(/^-|-$/g, '') || 'diagram')}.svg`;
    document.body.append(a);
    a.click();
    a.remove();
  };
  $('bar').addEventListener('click', (e) => {
    const act = (e.target as HTMLElement).closest('button')?.getAttribute('data-act');
    if (act === 'in') zoomCentre(1.3);
    else if (act === 'out') zoomCentre(1 / 1.3);
    else if (act === 'fit') { touched = false; fit('contain'); }
    else if (act === 'width') { touched = false; fit('width'); }
    else if (act === 'actual') actual();
    else if (act === 'save') save();
    else if (act === 'next') gotoHit(1);
    else if (act === 'prev') gotoHit(-1);
  });
  document.addEventListener('keydown', (e) => {
    if (e.target === search || e.ctrlKey || e.metaKey || e.altKey) return;
    const step = 60;
    if (e.key === '+' || e.key === '=') zoomCentre(1.25);
    else if (e.key === '-' || e.key === '_') zoomCentre(0.8);
    else if (e.key === '0') { touched = false; fit('contain'); }
    else if (e.key === 'w') { touched = false; fit('width'); }
    else if (e.key === '1') actual();
    else if (e.key === '/' || e.key === 'f') { e.preventDefault(); search.focus(); }
    else if (e.key === 'ArrowLeft') { st.x += step; apply(); }
    else if (e.key === 'ArrowRight') { st.x -= step; apply(); }
    else if (e.key === 'ArrowUp') { st.y += step; apply(); }
    else if (e.key === 'ArrowDown') { st.y -= step; apply(); }
  });
  addEventListener('resize', () => { if (!touched) fit(); });
  fit();
  requestAnimationFrame(() => { if (!touched) fit(); });
}

addEventListener('message', (e: MessageEvent) => {
  if (e.origin !== location.origin || e.source !== window.opener) return;
  const d = e.data as Partial<Payload> | undefined;
  if (d?.type === 'docent-diagram' && typeof d.svg === 'string') start(d as Payload);
});
window.opener?.postMessage({ type: 'docent-diagram-ready' }, location.origin);
