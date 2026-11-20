import { api } from './api';
import { copyText } from './format';
import { sectionMarkdown } from './sections';
import { toast } from './toast.svelte';
import { ui } from './ui.svelte';
import { softBreakCode } from './tables';

const EXPAND_ICON = '<svg class="icon icon--sm" viewBox="0 0 24 24"><path d="M15 3h6v6"/><path d="m21 3-7 7"/><path d="m3 21 7-7"/><path d="M9 21H3v-6"/></svg>';
const WRAP_ICON = '<svg class="icon icon--sm" viewBox="0 0 24 24"><line x1="3" x2="21" y1="6" y2="6"/><path d="M3 12h15a3 3 0 1 1 0 6h-4"/><polyline points="16 16 14 18 16 20"/><line x1="3" x2="10" y1="18" y2="18"/></svg>';
const COPY_ICON = '<svg class="icon icon--sm" viewBox="0 0 24 24"><rect width="14" height="14" x="8" y="8" rx="2" ry="2"/><path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2"/></svg>';

/**
 * Adds the behaviour the server's static HTML cannot have: copy buttons on code, external links that open in a new tab,
 * and scrolling to the #section in the address.
 */
export function enhance(root: HTMLElement, page?: { source: string; path: string }) {
  for (const code of root.querySelectorAll<HTMLElement>('.code')) {
    if (code.querySelector('.copy')) continue;
    const btn = document.createElement('button');
    btn.type = 'button';
    btn.className = 'btn btn--ghost btn--sm btn--icon copy';
    btn.title = 'Copy code';
    btn.setAttribute('aria-label', 'Copy code');
    btn.innerHTML = COPY_ICON;
    btn.addEventListener('click', async () => {
      const text = code.querySelector('pre')?.innerText ?? '';
      toast.show((await copyText(text)) ? 'Code copied' : 'Could not copy');
    });
    const wrap = document.createElement('button');
    wrap.type = 'button';
    wrap.className = 'btn btn--ghost btn--sm btn--icon wrapbtn';
    wrap.innerHTML = WRAP_ICON;
    wrap.addEventListener('click', () => {
      code.dataset.wrap = isWrapped(code) ? 'off' : 'on'; // this block only; the reading setting sets the default for all of them
      syncCode(code.parentElement ?? code);
    });
    code.querySelector('.code__bar')?.append(wrap, btn);
  }
  syncCode(root)
  enhanceTables(root);
  // task-list checkboxes are read out as their item ("Write docs, not done"), and code that scrolls sideways can be scrolled from the keyboard
  for (const box of root.querySelectorAll<HTMLInputElement>('input[type="checkbox"]')) {
    const text = (box.closest('li')?.textContent ?? '').replace(/\s+/g, ' ').trim().slice(0, 80);
    box.setAttribute('aria-label', `${text || 'Task'}, ${box.checked ? 'done' : 'not done'}`);
  }
  for (const a of root.querySelectorAll<HTMLAnchorElement>('a[href^="http"], a[href^="//"]')) {
    a.classList.add('external');
    a.target = '_blank';
    a.rel = 'noopener noreferrer';
  }
  if (page) addSectionCopy(root, page);
  if (location.hash.length > 1) {
    const id = decodeURIComponent(location.hash.slice(1));
    requestAnimationFrame(() => document.getElementById(id)?.scrollIntoView());
  }
}

/** A small button after each h2 to h4 that copies that section as Markdown. The page's source is only fetched when you click. */
function addSectionCopy(root: HTMLElement, page: { source: string; path: string }) {
  const seen = new Map<string, number>();
  for (const h of root.querySelectorAll<HTMLElement>(':scope > h2, :scope > h3, :scope > h4')) {
    const text = (h.textContent ?? '').replace(/^#/, '').trim();
    const occurrence = seen.get(text) ?? 0;
    seen.set(text, occurrence + 1);
    if (h.querySelector('.section-copy')) continue;
    const btn = document.createElement('button');
    btn.type = 'button';
    btn.className = 'section-copy';
    btn.title = 'Copy this section as Markdown';
    btn.setAttribute('aria-label', 'Copy this section as Markdown');
    btn.innerHTML = COPY_ICON;
    btn.addEventListener('click', async () => {
      try {
        const section = sectionMarkdown(await api.raw(page.source, page.path), text, occurrence);
        if (!section) return toast.show('Could not find that section in the source');
        toast.show((await copyText(section)) ? 'Section copied as Markdown' : 'Could not copy');
      } catch {
        toast.show('Could not copy');
      }
    });
    h.append(btn);
  }
}

/** The heading a table sits under, to name it in the enlarged view. */
function headingBefore(el: Element): string {
  for (let n: Element | null = el; n; n = n.previousElementSibling ?? n.parentElement) {
    let p: Element | null = n.previousElementSibling;
    while (p) {
      if (/^H[1-6]$/.test(p.tagName)) return (p.textContent ?? '').replace(/^#/, '').trim();
      p = p.previousElementSibling;
    }
    if (n.classList?.contains('prose')) break;
  }
  return '';
}

/**
 * Tables: long code may wrap, and a table with three or more columns gets an "enlarge" button that opens it in a big view
 * (every column, a header and first column that stay in view, copy as Markdown). The table sits in a box so the page can give it more room.
 */
function enhanceTables(root: HTMLElement) {
  for (const wrap of root.querySelectorAll<HTMLElement>('.table')) {
    const table = wrap.querySelector('table');
    if (!table || wrap.parentElement?.classList.contains('tablebox')) continue;
    softBreakCode(table);
    const box = document.createElement('div');
    box.className = 'tablebox';
    wrap.replaceWith(box);
    box.append(wrap);
    const columns = table.rows[0]?.cells.length ?? 0;
    if (columns < 3) continue;
    const btn = document.createElement('button');
    btn.type = 'button';
    btn.className = 'btn btn--ghost btn--sm tablebox__expand';
    btn.title = 'Open this table in a big view';
    btn.setAttribute('aria-label', 'Enlarge this table');
    btn.innerHTML = `${EXPAND_ICON}<span>Enlarge</span>`;
    btn.addEventListener('click', () => {
      ui.table = { html: table.outerHTML, title: headingBefore(box) || 'Table' };
    });
    box.append(btn);
  }
}

/**
 * Works out how each table sits: one that is wider than its box scrolls sideways (and can be reached by keyboard), and the page says whether any table is wider
 * than the text column, so the layout can give tables the full width. Safe to call again when the window changes size.
 */
export function layoutTables(root: HTMLElement): boolean {
  // the width of the text column: what a paragraph gets
  const probe = document.createElement('div');
  probe.style.cssText = 'position:absolute;visibility:hidden;height:0;width:var(--reading-w);max-width:100%';
  (root.closest('article') ?? root).append(probe);
  const textWidth = probe.offsetWidth;
  probe.remove();
  let wide = false;
  for (const wrap of root.querySelectorAll<HTMLElement>('.table')) {
    const table = wrap.querySelector('table');
    if (!table) continue;
    table.style.width = 'max-content'; // its natural width, if no cell had to wrap
    const natural = table.offsetWidth;
    table.style.width = 'min-content'; // and the least it can be squeezed to
    const least = table.offsetWidth;
    table.style.width = '';
    // Wide means the text column is too narrow for it: it cannot be squeezed into it, or it has many columns and would have to be squeezed a lot.
    // A table of two or three columns of sentences is long, not wide: wrapping its cells is the right thing to do.
    const columns = table.rows[0]?.cells.length ?? 0;
    const direct = wrap.parentElement?.parentElement === root; // tables inside a list or a callout stay in their place
    const breaksOut = direct && (least > textWidth + 1 || (columns >= 4 && natural > textWidth + 1));
    wrap.parentElement?.classList.toggle('tablebox--wide', breaksOut);
    if (breaksOut) wide = true;
    const scrolls = table.offsetWidth > wrap.clientWidth + 1;
    wrap.classList.toggle('is-scroll', scrolls);
    if (scrolls) {
      wrap.tabIndex = 0; // a table that scrolls sideways can be scrolled from the keyboard
      wrap.setAttribute('role', 'region');
      wrap.setAttribute('aria-label', `${headingBefore(wrap) || 'Table'}, scrolls sideways`);
    } else {
      wrap.removeAttribute('tabindex');
      wrap.removeAttribute('role');
      wrap.removeAttribute('aria-label');
    }
  }
  return wide;
}

const isWrapped = (code: Element) => {
  const pre = code.querySelector('pre');
  return !!pre && getComputedStyle(pre).whiteSpace.startsWith('pre-wrap');
};

/** Brings the wrap buttons and the keyboard focus of code blocks in step with what is showing (the reading setting, or this block's own switch). */
export function syncCode(root: ParentNode) {
  const blocks = root instanceof Element && root.matches('.code') ? [root] : [...root.querySelectorAll('.code')];
  for (const code of blocks) {
    const wrapped = isWrapped(code);
    const btn = code.querySelector<HTMLButtonElement>('.wrapbtn');
    if (btn) {
      btn.setAttribute('aria-pressed', String(wrapped));
      const label = wrapped ? 'Long lines wrap. Switch to scrolling sideways' : 'Wrap long lines';
      btn.title = label;
      btn.setAttribute('aria-label', 'Wrap long lines');
    }
    const pre = code.querySelector('pre');
    if (pre) {
      if (pre.scrollWidth > pre.clientWidth || pre.scrollHeight > pre.clientHeight) pre.tabIndex = 0; // code that scrolls can be scrolled from the keyboard
      else pre.removeAttribute('tabindex');
    }
  }
}

/** A reading setting changed: every block follows it again, whatever its own switch said. */
export function resetCodeWrap(root: ParentNode) {
  root.querySelectorAll('.code[data-wrap]').forEach((c) => c.removeAttribute('data-wrap'));
  syncCode(root);
}
