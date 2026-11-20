import { describe, expect, it } from 'vitest';
import { appeared, idNum, isRecent, isUnseen, markAllSeen, markOpened, parseStore, whatsNew } from './seen';
import type { LatestNote } from './types';

const note = (num: number, created: string, extra: Partial<LatestNote> = {}): LatestNote => ({ id: `DOC-${String(num).padStart(4, '0')}`, num, title: `Note ${num}`, url: `/n/${num}`, path: `${num}.md`, created, ...extra });
const NOW = new Date(2026, 10, 11, 15, 0, 0); // 11 November 2026

describe('seen', () => {
  it('reads the number out of an ID', () => {
    expect(idNum('DOC-0034')).toBe(34);
    expect(idNum('OPS-12')).toBe(12);
    expect(idNum('')).toBe(0);
    expect(idNum(undefined)).toBe(0);
    expect(idNum('no id')).toBe(0);
  });

  it('a note is new when it is above the base and not opened yet', () => {
    const rec = { base: 10, opened: [12] };
    expect(isUnseen(rec, 10)).toBe(false);
    expect(isUnseen(rec, 11)).toBe(true);
    expect(isUnseen(rec, 12)).toBe(false);
    expect(isUnseen(rec, 13)).toBe(true);
    expect(isUnseen(undefined, 99)).toBe(false); // no record yet: nothing is new until the first look sets the base
  });

  it('opening a note marks it seen, and the base catches up when the notes above it are all seen', () => {
    let rec = { base: 10, opened: [] as number[] };
    rec = markOpened(rec, 12);
    expect(rec).toEqual({ base: 10, opened: [12] });
    rec = markOpened(rec, 11);
    expect(rec).toEqual({ base: 12, opened: [] });
    expect(markOpened(rec, 5)).toBe(rec); // an old note changes nothing
    expect(markOpened(rec, 12)).toBe(rec);
  });

  it('marks everything seen', () => {
    expect(markAllSeen(40)).toEqual({ base: 40, opened: [] });
  });

  it('trusts nothing it reads back', () => {
    expect(parseStore(null)).toEqual({});
    expect(parseStore('not json')).toEqual({});
    expect(parseStore('[1,2]')).toEqual({});
    expect(parseStore('{"a":{"base":"x"},"b":{"base":-1},"c":{"base":5,"opened":[3,7,"x",8.5,9]},"d":null}')).toEqual({ c: { base: 5, opened: [7, 9] } });
  });

  it('knows what recent means (today counts, a week is seven days)', () => {
    expect(isRecent('2026-11-11', 7, NOW)).toBe(true);
    expect(isRecent('2026-11-05', 7, NOW)).toBe(true);
    expect(isRecent('2026-11-04', 7, NOW)).toBe(false);
    expect(isRecent('', 7, NOW)).toBe(false);
    expect(isRecent('yesterday', 7, NOW)).toBe(false);
  });

  it('lists the unseen and the recent notes, newest first, and only that many', () => {
    const src = [
      { source: 'a', name: 'A', notes: [note(30, '2026-11-11'), note(29, '2026-11-09'), note(20, '2026-10-01'), note(19, '2026-10-01')] },
      { source: 'b', name: 'B', notes: [note(7, '2026-11-10'), note(2, '2026-09-01')] },
    ];
    const store = { a: { base: 29, opened: [] }, b: { base: 7, opened: [] } };
    const list = whatsNew(src, store, 7, 10, NOW);
    expect(list.map((n) => `${n.source}${n.num}`)).toEqual(['a30', 'b7', 'a29']);
    expect(list.map((n) => n.unseen)).toEqual([true, false, false]);
    expect(whatsNew(src, store, 7, 2, NOW)).toHaveLength(2);
    // an unseen note shows even if it is old (it was added to an old date, or dated by hand)
    expect(whatsNew([{ source: 'a', name: 'A', notes: [note(31, '2026-01-01')] }], { a: { base: 30, opened: [] } }, 7, 5, NOW)[0]?.unseen).toBe(true);
  });

  it('says which notes appeared since the last look, and stays quiet the first time', () => {
    const src = [{ source: 'a', notes: [note(31, '2026-11-11'), note(30, '2026-11-11'), note(29, '2026-11-09')] }];
    const store = { a: { base: 29, opened: [] } };
    expect(appeared(null, src, store)).toEqual([]);
    const before = new Set(['a:DOC-0029', 'a:DOC-0030']);
    expect(appeared(before, src, store).map((x) => x.note.id)).toEqual(['DOC-0031']);
    expect(appeared(before, src, { a: { base: 29, opened: [31] } })).toEqual([]); // already opened
    expect(appeared(new Set(), src, store).map((x) => x.note.id)).toEqual(['DOC-0031', 'DOC-0030']);
  });
});
