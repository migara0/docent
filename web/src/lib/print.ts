// Printing and PDF: leave parts out, and put the page's ID, title, date and page numbers on every printed page.

/** Headings (and bold label lines) with these names are left out by default, as are headings the author marked with { data-no-print }. */
export const DEFAULT_OMIT = ['related notes', 'related', 'see also'];

export interface Selection {
  meta: boolean;
  diagrams: boolean;
  code: boolean;
  includeOmitted: boolean;
  sections: Record<string, boolean>;
}

export interface Section {
  id: string;
  title: string;
  byDefault: boolean;
}

const norm = (t: string) => t.replace(/#$/, '').replace(/^[\d.\s]+/, '').replace(/[:.\s]+$/, '').trim().toLowerCase();
const level = (el: Element) => (/^H[1-6]$/.test(el.tagName) ? Number(el.tagName[1]) : 0);
const prose = () => document.querySelector<HTMLElement>('.prose');
const headingText = (h: Element) => (h.textContent ?? '').replace(/^#/, '').trim();
const omittedByDefault = (h: Element) => h.hasAttribute('data-no-print') || DEFAULT_OMIT.includes(norm(headingText(h)));

/** The heading and everything after it up to the next heading of the same or a higher level. */
function extent(h: Element): Element[] {
  const out: Element[] = [h];
  const n = level(h);
  for (let el = h.nextElementSibling; el && !(level(el) && level(el) <= n); el = el.nextElementSibling) out.push(el);
  return out;
}

export function sections(): Section[] {
  const root = prose();
  if (!root) return [];
  return [...root.children].filter((e) => e.tagName === 'H2').map((h, i) => ({ id: h.id || `section-${i}`, title: headingText(h), byDefault: !omittedByDefault(h) }));
}

export function defaultSelection(): Selection {
  return { meta: true, diagrams: true, code: true, includeOmitted: false, sections: Object.fromEntries(sections().map((s) => [s.id, s.byDefault])) };
}

/** Label lines such as "**Related notes:** ..." and sub-headings left out by default (not part of the section list). */
export function otherOmissions(): string[] {
  const root = prose();
  if (!root) return [];
  const out: string[] = [];
  for (const p of root.querySelectorAll(':scope > p')) {
    const first = p.firstElementChild;
    if (first?.tagName === 'STRONG' && DEFAULT_OMIT.includes(norm(first.textContent ?? ''))) out.push(norm(first.textContent ?? ''));
  }
  for (const h of root.querySelectorAll(':scope > h3, :scope > h4')) if (omittedByDefault(h)) out.push(headingText(h));
  return [...new Set(out)];
}

let hidden: Element[] = [];
let pending: Selection | null = null;
let headerStyle: HTMLStyleElement | null = null;
let info: () => { title: string; id: string; updated: string; library: string } = () => ({ title: document.title, id: '', updated: '', library: 'Docent' });

export function setPrintInfo(fn: typeof info) {
  info = fn;
}

function hide(els: Element[]) {
  for (const el of els) {
    el.classList.add('print-hidden');
    hidden.push(el);
  }
}

function clear() {
  for (const el of hidden) el.classList.remove('print-hidden', 'print-include');
  hidden = [];
  headerStyle?.remove();
  headerStyle = null;
}

function apply(sel: Selection) {
  clear();
  const root = prose();
  if (!root) return;
  if (!sel.meta) hide([...document.querySelectorAll('.pagehead .meta')]);
  if (!sel.diagrams) hide([...root.querySelectorAll('figure.diagram')]);
  if (!sel.code) hide([...root.querySelectorAll('.code')]);
  for (const h of root.children) {
    if (h.tagName === 'H2' && sel.sections[h.id || ''] === false) hide(extent(h));
  }
  if (!sel.includeOmitted) {
    for (const p of root.querySelectorAll(':scope > p')) {
      const first = p.firstElementChild;
      if (first?.tagName === 'STRONG' && DEFAULT_OMIT.includes(norm(first.textContent ?? ''))) hide([p]);
    }
    for (const h of root.querySelectorAll(':scope > h3, :scope > h4')) if (omittedByDefault(h)) hide(extent(h));
  }
}

const cssString = (s: string) => `"${s.replace(/\\/g, '\\\\').replace(/"/g, '\\"').replace(/[\r\n]+/g, ' ')}"`;

/** @page margin boxes: "ID · Title" and the date on top, the library name and "page x / y" at the bottom. */
function addHeader() {
  const i = info();
  const box = 'font: 8pt sans-serif; color: #666;';
  const title = `${i.id ? `${i.id} · ` : ''}${i.title}`.slice(0, 110);
  headerStyle = document.createElement('style');
  headerStyle.textContent =
    `@page { margin: 18mm 14mm 16mm; @top-left { content: ${cssString(title)}; ${box} }` +
    (i.updated ? ` @top-right { content: ${cssString(`Updated ${i.updated}`)}; ${box} }` : '') +
    ` @bottom-left { content: ${cssString(i.library)}; ${box} } @bottom-right { content: counter(page) " / " counter(pages); ${box} } }`;
  document.head.append(headerStyle);
}

/** Called once. Ctrl+P and the Print button both get the default omissions and the header; the chooser's selection applies to its own print only. */
export function installPrint() {
  addEventListener('beforeprint', () => {
    apply(pending ?? defaultSelection());
    addHeader();
  });
  addEventListener('afterprint', () => {
    clear();
    pending = null;
  });
}

export function printWith(sel: Selection) {
  pending = sel;
  window.print();
}
