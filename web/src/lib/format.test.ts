import { describe, expect, it } from 'vitest';
import { createdDay, relativeTime, stamp } from './format';

describe('relativeTime', () => {
  const evening = new Date(2026, 10, 11, 22, 56); // 11 November 2026, 22:56 here
  const earlyMorning = new Date(2026, 10, 11, 0, 20);

  it('says today for a date-only value of today, at any hour of the day', () => {
    expect(relativeTime('2026-11-11', evening)).toBe('today');
    expect(relativeTime('2026-11-11', earlyMorning)).toBe('today');
    expect(relativeTime('2026-11-11', new Date(2026, 10, 11, 12, 0))).toBe('today');
  });

  it('counts calendar days, not hours', () => {
    expect(relativeTime('2026-11-10', evening)).toBe('yesterday');
    expect(relativeTime('2026-11-10', earlyMorning)).toBe('yesterday');
    expect(relativeTime('2026-11-08', evening)).toMatch(/3 days ago/);
  });

  it('reads a full timestamp in this time zone', () => {
    expect(relativeTime(new Date(2026, 10, 11, 1, 0).toISOString(), evening)).toBe('today');
    expect(relativeTime(new Date(2026, 10, 10, 23, 59).toISOString(), evening)).toBe('yesterday');
  });

  it('goes to weeks and then to a date, and is quiet about nonsense', () => {
    expect(relativeTime('2026-10-28', evening)).toMatch(/2 weeks ago/);
    expect(relativeTime('2026-01-02', evening)).toMatch(/2026/);
    expect(relativeTime('', evening)).toBe('');
    expect(relativeTime('not a date', evening)).toBe('');
    expect(relativeTime(undefined, evening)).toBe('');
  });
});

describe('stamp and createdDay', () => {
  it('shows the day and time in this time zone when the moment is known', () => {
    const at = new Date(2026, 10, 17, 21, 52, 6).toISOString(); // 17 November 2026, 21:52:06 here, sent as UTC
    const s = stamp('2026-11-17', at);
    expect(s.text).toBe('2026-11-17 21:52');
    expect(s.title).toMatch(/^2026-11-17 21:52:06 \(.+\)$/);
  });

  it('shows the day alone when only the day is known, and says so on hover', () => {
    expect(stamp('2026-11-09')).toEqual({ text: '2026-11-09', title: '2026-11-09 (the note records the day, not the time)' });
    expect(stamp('2026-11-09', 'not a date').text).toBe('2026-11-09');
    expect(stamp(undefined)).toEqual({ text: '', title: '' });
  });

  it('takes the creation day from the moment, in this time zone', () => {
    expect(createdDay({ created: '2026-11-17', createdAt: new Date(2026, 10, 18, 0, 30).toISOString() })).toBe('2026-11-18');
    expect(createdDay({ created: '2026-11-17' })).toBe('2026-11-17');
    expect(createdDay({})).toBe('');
  });
});
