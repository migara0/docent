// Reading preferences, remembered in this browser (localStorage) and applied to <html> so the CSS tokens take over.

export type Theme = 'auto' | 'light' | 'dark';
export type Width = 'narrow' | 'standard' | 'wide';
export type Editor = 'none' | 'vscode' | 'zed';

interface Stored {
  theme: Theme;
  scale: number;
  width: Width;
  focus: boolean;
  toc: boolean; // the "On this page" list beside a page
  wrapCode: boolean; // long lines of code wrap instead of scrolling sideways
  editor: Editor;
  railWidth: number; // pixels, the page-tree column
}

const KEY = 'docent:prefs';
const DEFAULTS: Stored = { theme: 'auto', scale: 1, width: 'standard', focus: false, toc: true, wrapCode: false, editor: 'none', railWidth: 272 };
export const SCALE_MIN = 0.8;
export const SCALE_MAX = 1.6;
export const RAIL_MIN = 224;
export const RAIL_MAX = 560;
export const RAIL_DEFAULT = 272;

function load(): Stored {
  try {
    const raw = JSON.parse(localStorage.getItem(KEY) ?? '{}') as Partial<Stored>;
    return {
      theme: ['auto', 'light', 'dark'].includes(raw.theme as string) ? (raw.theme as Theme) : DEFAULTS.theme,
      scale: typeof raw.scale === 'number' ? Math.min(SCALE_MAX, Math.max(SCALE_MIN, raw.scale)) : DEFAULTS.scale,
      width: ['narrow', 'standard', 'wide'].includes(raw.width as string) ? (raw.width as Width) : DEFAULTS.width,
      focus: raw.focus === true,
      toc: raw.toc !== false,
      wrapCode: raw.wrapCode === true,
      editor: ['none', 'vscode', 'zed'].includes(raw.editor as string) ? (raw.editor as Editor) : DEFAULTS.editor,
      railWidth: typeof raw.railWidth === 'number' ? Math.min(RAIL_MAX, Math.max(RAIL_MIN, Math.round(raw.railWidth))) : DEFAULTS.railWidth,
    };
  } catch {
    return { ...DEFAULTS };
  }
}

class Prefs {
  theme = $state<Theme>(DEFAULTS.theme);
  scale = $state(DEFAULTS.scale);
  width = $state<Width>(DEFAULTS.width);
  focus = $state(DEFAULTS.focus);
  toc = $state(DEFAULTS.toc);
  wrapCode = $state(DEFAULTS.wrapCode);
  editor = $state<Editor>(DEFAULTS.editor);
  railWidth = $state(DEFAULTS.railWidth);
  systemDark = $state(matchMedia('(prefers-color-scheme: dark)').matches);

  constructor() {
    Object.assign(this, load());
    this.apply();
    matchMedia('(prefers-color-scheme: dark)').addEventListener('change', (e) => (this.systemDark = e.matches));
  }

  private apply() {
    const root = document.documentElement;
    if (this.theme === 'auto') root.removeAttribute('data-theme');
    else root.dataset.theme = this.theme;
    if (this.width === 'standard') root.removeAttribute('data-width');
    else root.dataset.width = this.width;
    root.toggleAttribute('data-wrapcode', this.wrapCode);
    root.style.setProperty('--reading-scale', String(this.scale));
    root.style.setProperty('--rail-w', `${this.railWidth}px`);
  }

  private save() {
    try {
      localStorage.setItem(KEY, JSON.stringify({ theme: this.theme, scale: this.scale, width: this.width, focus: this.focus, toc: this.toc, wrapCode: this.wrapCode, editor: this.editor, railWidth: this.railWidth }));
    } catch {
      /* private mode: the preference just will not persist */
    }
  }

  set<K extends keyof Stored>(key: K, value: Stored[K]) {
    (this as Stored)[key] = value;
    this.apply();
    this.save();
  }

  setScale(v: number) {
    this.set('scale', Math.round(Math.min(SCALE_MAX, Math.max(SCALE_MIN, v)) * 10) / 10);
  }

  setRailWidth(px: number) {
    this.set('railWidth', Math.min(RAIL_MAX, Math.max(RAIL_MIN, Math.round(px))));
  }

  cycleTheme() {
    this.set('theme', ({ auto: 'light', light: 'dark', dark: 'auto' } as const)[this.theme]);
  }

  /** The theme actually showing, after resolving "auto" against the system. */
  get dark(): boolean {
    return this.theme === 'dark' || (this.theme === 'auto' && this.systemDark);
  }
}

export const prefs = new Prefs();
