import { describe, expect, it } from 'vitest';
import { baseName, parentOf } from './paths';

describe('paths', () => {
  it('baseName works with either separator', () => {
    expect(baseName('/home/a/notes')).toBe('notes');
    expect(baseName('/home/a/notes/')).toBe('notes');
    expect(baseName('C:\\Users\\a\\notes')).toBe('notes');
    expect(baseName('C:\\Users\\a\\notes\\')).toBe('notes');
  });

  it('parentOf keeps roots and drives', () => {
    expect(parentOf('/home/a/x.md')).toBe('/home/a');
    expect(parentOf('/x.md')).toBe('/');
    expect(parentOf('C:\\Users\\a\\x.md')).toBe('C:\\Users\\a');
    expect(parentOf('C:\\x.md')).toBe('C:\\');
    expect(parentOf('D:/notes/x.md')).toBe('D:/notes');
    expect(parentOf('x.md')).toBe('x.md');
  });
});
