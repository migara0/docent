import { describe, expect, it } from 'vitest';
import { breakPoints, toMarkdown } from './tables';

describe('tables', () => {
  it('writes a table as Markdown, with a header rule', () => {
    expect(toMarkdown([['Route', 'Purpose'], ['`GET /a`', 'First'], ['`POST /b`', 'Second']])).toBe('| Route | Purpose |\n| --- | --- |\n| `GET /a` | First |\n| `POST /b` | Second |');
  });

  it('escapes a pipe, flattens line breaks, and pads short rows', () => {
    expect(toMarkdown([['A', 'B', 'C'], ['x | y', 'line\nbreak'], []])).toBe('| A | B | C |\n| --- | --- | --- |\n| x \\| y | line break |  |\n|  |  |  |');
  });

  it('gives nothing for nothing', () => {
    expect(toMarkdown([])).toBe('');
    expect(toMarkdown([[]])).toBe('');
  });

  it('breaks long code after separators, and leaves short code alone', () => {
    expect(breakPoints('GET')).toBeNull();
    expect(breakPoints('POST /api/v2/organisations/{id}/members')).toEqual(['POST /', 'api/', 'v2/', 'organisations/', '{id}/', 'members']);
    expect(breakPoints('profile_event_handler_name')).toEqual(['profile_', 'event_', 'handler_', 'name']);
    expect(breakPoints('averyveryverylongwordwithnoseparators')).toBeNull(); // nothing to break at: the browser decides
  });
});
