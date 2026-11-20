import { describe, expect, it } from 'vitest';
import { elideLabels, filterTree, highlightParts, splitTail } from './treenav';
import type { TreeNode } from './types';

const page = (path: string, title: string, id?: string): TreeNode => ({ kind: 'page', name: path.split('/').pop()!, path, title, id, url: '/x/' + path });
const folder = (path: string, children: TreeNode[]): TreeNode => ({ kind: 'folder', name: path.split('/').pop()!, path, children, count: children.length });

describe('elideLabels', () => {
  it('cuts the words siblings share, so what differs is what shows', () => {
    const labels = elideLabels([
      page('a/i.md', 'Payments integration Phase 1 plan'),
      page('a/db.md', 'Payments database schemas'),
      page('a/dec.md', 'Payments: decisions and assumptions'),
    ]);
    expect(labels).toEqual({ 'a/i.md': 'Integration Phase 1 plan', 'a/db.md': 'Database schemas', 'a/dec.md': 'Decisions and assumptions' });
  });
  it('keeps a number with its label and leaves the index page alone', () => {
    const labels = elideLabels([
      page('b/index.md', 'Build block notes'),
      page('b/1.md', 'Payments block 1: foundation'),
      page('b/2.md', 'Payments block 2: Ledger client'),
      page('b/3.md', 'Payments block 3: sites'),
    ]);
    expect(labels).toEqual({ 'b/1.md': 'Block 1: foundation', 'b/2.md': 'Block 2: Ledger client', 'b/3.md': 'Block 3: sites' });
  });
  it('does nothing when it would not help', () => {
    expect(elideLabels([page('c/1.md', 'Install'), page('c/2.md', 'Configure'), page('c/3.md', 'Use')])).toEqual({});
    expect(elideLabels([page('c/1.md', 'Payments one'), page('c/2.md', 'Payments two')])).toEqual({}); // two pages are not a pattern
    expect(elideLabels([page('c/1.md', 'The first'), page('c/2.md', 'The second'), page('c/3.md', 'The third')])).toEqual({}); // only filler words in common
    expect(elideLabels([page('c/1.md', 'Same title'), page('c/2.md', 'Same title'), page('c/3.md', 'Same title')])).toEqual({ 'c/1.md': 'Title', 'c/2.md': 'Title', 'c/3.md': 'Title' }); // never an empty label
  });
  it('never leaves an empty label, even when one title is the shared start of the others', () => {
    const labels = elideLabels([page('d/0.md', 'Payments'), page('d/1.md', 'Payments users'), page('d/2.md', 'Payments sites')]);
    expect(labels).toEqual({});
  });
});

describe('filterTree', () => {
  const tree = [
    folder('bc', [page('bc/a.md', 'Payments block 5: users', 'OPS-0033'), page('bc/b.md', 'Payments block 6: reconcile', 'OPS-0034'), folder('bc/sub', [page('bc/sub/c.md', 'Users summary', 'OPS-0040')])]),
    page('x.md', 'Webhooks', 'OPS-0050'),
  ];
  it('matches every typed word against title, path and ID, and drops empty folders', () => {
    const r = filterTree(tree, 'users');
    expect(r.matches).toBe(2);
    expect(r.total).toBe(4);
    expect(r.nodes).toHaveLength(1);
    expect(r.nodes[0]!.children!.map((n) => n.path)).toEqual(['bc/a.md', 'bc/sub']);
    expect(filterTree(tree, 'block 6').matches).toBe(1);
    expect(filterTree(tree, 'bc block').matches).toBe(2); // folder names count
    expect(filterTree(tree, 'ops-0050').nodes.map((n) => n.path)).toEqual(['x.md']);
    expect(filterTree(tree, 'nothing').nodes).toEqual([]);
  });
  it('a short word matches only the start of a word, so a number does not match inside a date', () => {
    const t = [folder('d', [page('d/2026-10-15-a.md', 'Block 1: foundation'), page('d/2026-10-16-b.md', 'Block 5: users'), page('d/2026-10-17-c.md', 'Block 15: later')])];
    expect(filterTree(t, 'block 5').nodes[0]!.children!.map((n) => n.path)).toEqual(['d/2026-10-16-b.md']);
    expect(filterTree(t, 'ment').matches).toBe(0);
    expect(filterTree([page('x.md', 'Payments integration')], 'ment').matches).toBe(1);
    expect(filterTree([page('x.md', 'Payments integration')], 'ent').matches).toBe(1); // three characters: anywhere
    expect(filterTree([page('x.md', 'Payments integration')], 'pi').matches).toBe(0);
  });
  it('an empty query changes nothing', () => {
    const r = filterTree(tree, '  ');
    expect(r.nodes).toBe(tree);
    expect(r.matches).toBe(4);
  });
});

describe('highlightParts', () => {
  it('marks the typed words, case-insensitively, and survives special characters', () => {
    expect(highlightParts('Payments block 5', ['block', '5'])).toEqual([
      { text: 'Payments ', hit: false },
      { text: 'block', hit: true },
      { text: ' ', hit: false },
      { text: '5', hit: true },
    ]);
    expect(highlightParts('a (b) c', ['(b)']).some((p) => p.hit)).toBe(true);
    expect(highlightParts('plain', [])).toEqual([{ text: 'plain', hit: false }]);
  });
});

describe('splitTail', () => {
  it('keeps the start and the last word or two of a long title', () => {
    expect(splitTail('Integration Phase 1 design decisions')).toEqual({ head: 'Integration Phase 1 design', tail: ' decisions' });
    expect(splitTail('Integration Phase 1 open questions')).toEqual({ head: 'Integration Phase 1 open', tail: ' questions' });
    expect(splitTail('Phase 1 demo cheat sheet for the team')).toEqual({ head: 'Phase 1 demo cheat sheet for', tail: ' the team' });
  });
  it('leaves short titles and titles of one or two words whole', () => {
    expect(splitTail('Integration runbook')).toEqual({ head: 'Integration runbook', tail: '' });
    expect(splitTail('Supercalifragilisticexpialidocious Documentation')).toEqual({ head: 'Supercalifragilisticexpialidocious Documentation', tail: '' });
  });
  it('joins back to the original text', () => {
    for (const t of ['Database schemas and migrations for the reconcile job', 'x y z w v u t s r q p o n m l k']) {
      const { head, tail } = splitTail(t);
      expect(head + tail).toBe(t);
    }
  });
});
