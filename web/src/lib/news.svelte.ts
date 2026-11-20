// What is new: notes you have not opened yet (see seen.ts), the toast when one appears while Docent is open, and the "open the newest note" shortcut.

import { api } from './api';
import { router } from './router.svelte';
import { toast } from './toast.svelte';
import { appeared, byNewest, idNum, isUnseen, markAllSeen, markOpened, parseStore, whatsNew, type NewNote, type SeenStore } from './seen';
import type { LatestSource } from './types';

const KEY = 'docent:seen';
const RECENT_DAYS = 7;

class News {
  sources = $state<LatestSource[]>([]);
  private store = $state<SeenStore>(load());
  private known: Set<string> | null = null; // the notes the last fetch listed, to tell what appeared since
  private loaded = false;

  async refresh() {
    let sources: LatestSource[];
    try {
      sources = (await api.latest()).sources;
    } catch {
      return;
    }
    // the first look at a library sets the line: everything already there counts as seen
    let changed = false;
    for (const s of sources) {
      if (!this.store[s.source]) {
        this.store[s.source] = markAllSeen(s.maxId);
        changed = true;
      }
    }
    if (changed) this.save();
    const fresh = appeared(this.known, sources, this.store);
    this.known = new Set(sources.flatMap((s) => s.notes.map((n) => `${s.source}:${n.id}`)));
    this.sources = sources;
    this.loaded = true;
    if (fresh.length) this.announce(fresh);
  }

  private announce(fresh: { source: string; note: { id: string; title: string; url: string } }[]) {
    const first = fresh[0]?.note;
    if (!first) return;
    const more = fresh.length > 1 ? ` and ${fresh.length - 1} more` : '';
    toast.show(`New note ${first.id}: ${first.title}${more}`, 9000, { label: 'Open', run: () => router.go(first.url) });
  }

  isNew(source: string, id: string | undefined): boolean {
    return !!id && isUnseen(this.store[source], idNum(id));
  }

  /** Opening a note marks it seen. */
  open(source: string, id: string | undefined) {
    const rec = this.store[source];
    if (!id || !rec) return;
    const next = markOpened(rec, idNum(id));
    if (next !== rec) {
      this.store[source] = next;
      this.save();
    }
  }

  /** How many notes of a library are new (the newest 40 are looked at). */
  count(source: string): number {
    const s = this.sources.find((x) => x.source === source);
    return s ? s.notes.filter((n) => isUnseen(this.store[source], n.num)).length : 0;
  }

  get total(): number {
    return this.sources.reduce((sum, s) => sum + this.count(s.source), 0);
  }

  /** The new and the recent notes, newest first. */
  list(limit: number): NewNote[] {
    return whatsNew(this.sources, this.store, RECENT_DAYS, limit);
  }

  /** The newest note that has not been opened. */
  newest(): NewNote | undefined {
    const unseen: NewNote[] = [];
    for (const s of this.sources) for (const n of s.notes) if (isUnseen(this.store[s.source], n.num)) unseen.push({ ...n, source: s.source, library: s.name, unseen: true });
    return unseen.sort(byNewest)[0];
  }

  markAll(source?: string) {
    for (const s of this.sources) if (!source || s.source === source) this.store[s.source] = markAllSeen(s.maxId);
    this.save();
  }

  /** The `n` key: open the newest note you have not seen. */
  async openNewest() {
    if (!this.loaded) await this.refresh(); // pressed before the first list arrived
    const n = this.newest();
    if (n) router.go(n.url);
    else toast.show('No new notes. You have seen them all.');
  }

  private save() {
    try {
      localStorage.setItem(KEY, JSON.stringify(this.store));
    } catch {
      /* no storage (a private window): it works for this visit only */
    }
  }
}

function load(): SeenStore {
  try {
    return parseStore(localStorage.getItem(KEY));
  } catch {
    return {};
  }
}

export const news = new News();
