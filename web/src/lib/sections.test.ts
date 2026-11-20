import { describe, expect, test } from 'vitest';
import { sectionMarkdown } from './sections';

const doc = `---
title: T
---

# Title

## One

Text one.

### Sub

Deep.

## Two { data-no-print }

Text two.

\`\`\`md
## Not a heading
\`\`\`

## One

Second one.
`;

describe('sectionMarkdown', () => {
  test('takes the section with its sub-sections, up to the next heading of the same level', () => {
    expect(sectionMarkdown(doc, 'One', 0)).toBe('## One\n\nText one.\n\n### Sub\n\nDeep.\n');
  });
  test('a sub-section ends at the next heading of any higher or equal level', () => {
    expect(sectionMarkdown(doc, 'Sub', 0)).toBe('### Sub\n\nDeep.\n');
  });
  test('attribute lists are dropped, and code fences are not searched', () => {
    expect(sectionMarkdown(doc, 'Two', 0)).toBe('## Two\n\nText two.\n\n```md\n## Not a heading\n```\n');
    expect(sectionMarkdown(doc, 'Not a heading', 0)).toBeUndefined();
  });
  test('the same heading text twice: the occurrence decides', () => {
    expect(sectionMarkdown(doc, 'One', 1)).toBe('## One\n\nSecond one.\n');
    expect(sectionMarkdown(doc, 'One', 2)).toBeUndefined();
  });
  test('markup in the heading is ignored when matching', () => {
    expect(sectionMarkdown('## The `code` and **bold**\n\nx\n', 'The code and bold', 0)).toBe('## The `code` and **bold**\n\nx\n');
  });
});
