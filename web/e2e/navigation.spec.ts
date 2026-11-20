import { test as base, expect, type Page } from '@playwright/test';
import { mkdtempSync, mkdirSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

const test = base.extend<{ errors: string[] }>({
  errors: [
    async ({ page }, use) => {
      const errors: string[] = [];
      page.on('pageerror', (e) => errors.push(`pageerror: ${e.message}`));
      page.on('console', (m) => m.type() === 'error' && errors.push(`console: ${m.text()}`));
      await use(errors);
      expect(errors, 'the browser reported errors').toEqual([]);
    },
    { auto: true },
  ],
});

// A folder like the Payments notes: many pages whose titles start the same, one of them an index page.
const titles = [
  'Payments integration Phase 1 design decisions',
  'Payments integration Phase 1 open questions',
  'Payments integration Phase 1 plan',
  'Payments integration API reference',
  'Payments integration QA guide',
  'Payments integration runbook',
  'Payments database schemas and migrations',
  'Payments: decisions, assumptions and open items',
  'Payments block 5: users',
  'Payments block 6: reconcile',
];

let dir = '';
let src = '';

test.beforeAll(async ({ request }) => {
  dir = mkdtempSync(join(tmpdir(), 'docent-nav-'));
  mkdirSync(join(dir, 'payments'));
  writeFileSync(join(dir, 'index.md'), '# Navigation fixture\n');
  writeFileSync(join(dir, 'payments', 'index.md'), '# Payments integration\n');
  titles.forEach((t, i) => writeFileSync(join(dir, 'payments', `2026-10-${String(10 + i)}-p${i}.md`), `---\ntitle: "${t}"\n---\n\n# ${t}\n\nBody ${i}.\n`));
  const res = await request.post('/_api/open', { data: { path: dir } });
  src = ((await res.json()) as { id: string }).id;
});
test.afterAll(() => rmSync(dir, { recursive: true, force: true }));

const open = async (page: Page, i = 2) => {
  await page.goto(`/${src}/payments/2026-10-${10 + i}-p${i}`);
  await expect(page.locator('.tree a[aria-current="page"]')).toBeVisible();
};

test.describe('page tree navigation', () => {
  test('titles that start the same are shown from where they differ, with the full title as the tooltip', async ({ page }) => {
    await open(page);
    const row = (name: string) => page.locator('.tree a', { has: page.locator(`.label:has-text("${name}")`) });
    await expect(row('Block 5: users')).toBeVisible(); // "Payments block 5: users" without the shared word
    await expect(row('Block 5: users')).toHaveAttribute('title', 'Payments block 5: users');
    await expect(row('Database schemas')).toHaveAttribute('title', 'Payments database schemas and migrations');
    await expect(row('Decisions, assumptions')).toBeVisible(); // "Payments:" shares the word whatever follows it
    // the index page is not part of the pattern and keeps its title
    await expect(page.locator('.tree a', { hasText: 'Payments integration' }).first()).toBeVisible();
    // and nothing about the pages themselves changed
    await expect(page.locator('.pagehead h1')).toHaveText('Payments integration Phase 1 plan');
  });

  test('a long title keeps its last words visible, so look-alike pages can be told apart', async ({ page }) => {
    await page.setViewportSize({ width: 1100, height: 800 });
    await open(page);
    const a = page.locator('.tree a[title^="Payments integration Phase 1 design"] .tail');
    const b = page.locator('.tree a[title^="Payments integration Phase 1 open"] .tail');
    await expect(a).toHaveText('decisions');
    await expect(b).toHaveText('questions');
    for (const l of [a, b]) {
      const box = await l.boundingBox();
      const rail = await page.locator('.rail').boundingBox();
      expect(box!.x + box!.width, 'the tail stays inside the column').toBeLessThanOrEqual(rail!.x + rail!.width);
    }
  });

  test('the filter narrows the tree to matching pages, opens their folders, marks the words, and Escape clears it', async ({ page }) => {
    await open(page);
    const filter = page.getByRole('textbox', { name: /Filter pages/ });
    await filter.fill('block users');
    await expect(page.locator('.treefilter__count')).toHaveText('1 of 12 pages');
    await expect(page.locator('.tree a')).toHaveCount(1);
    await expect(page.locator('.tree a')).toContainText('Payments block 5: users'); // the full title while filtering
    await expect(page.locator('.tree mark')).toHaveText(['block', 'users']); // each typed word is marked in the title
    await filter.fill('5');
    await expect(page.locator('.tree a')).toHaveCount(1); // a number matches the start of a word, not the "5" in a date
    await filter.fill('zzzz');
    await expect(page.locator('.rail__empty')).toContainText('No page matches');
    await filter.press('Escape');
    await expect(filter).toHaveValue('');
    await expect(page.locator('.treefilter__count')).toHaveCount(0);
    await expect(page.locator('.tree a').first()).toBeVisible();
  });

  test('clicking a filtered result opens the page, and the clear button empties the filter', async ({ page }) => {
    await open(page);
    await page.getByRole('textbox', { name: /Filter pages/ }).fill('reconcile');
    await page.locator('.tree a').click();
    await expect(page.locator('.pagehead h1')).toHaveText('Payments block 6: reconcile');
    await page.getByRole('button', { name: 'Clear the filter' }).click();
    await expect(page.getByRole('textbox', { name: /Filter pages/ })).toHaveValue('');
  });

  test('the column can be dragged wider, resized from the keyboard, reset by double-click, and the width is remembered', async ({ page }) => {
    await open(page);
    const rail = page.locator('.rail');
    const handle = page.getByRole('separator', { name: 'Resize the page tree' });
    expect((await rail.boundingBox())!.width).toBeCloseTo(272, 0);
    const box = (await handle.boundingBox())!;
    await page.mouse.move(box.x + box.width / 2, 300);
    await page.mouse.down();
    await page.mouse.move(440, 300, { steps: 6 });
    await page.mouse.up();
    expect((await rail.boundingBox())!.width).toBeCloseTo(440, 0);
    await page.reload();
    expect((await rail.boundingBox())!.width, 'remembered after a reload').toBeCloseTo(440, 0);
    await handle.focus();
    await page.keyboard.press('ArrowLeft');
    expect((await rail.boundingBox())!.width).toBeCloseTo(424, 0);
    await page.keyboard.press('Home');
    expect((await rail.boundingBox())!.width).toBeCloseTo(224, 0); // the narrowest allowed
    await page.keyboard.press('End');
    expect((await rail.boundingBox())!.width).toBeCloseTo(560, 0); // the widest allowed
    await handle.dblclick();
    expect((await rail.boundingBox())!.width).toBeCloseTo(272, 0);
  });

  test('a small library has no filter box, and on a phone there is no resize handle', async ({ page }) => {
    await page.goto('/e2e-notes/index');
    await expect(page.getByRole('textbox', { name: /Filter pages/ })).toHaveCount(0);
    await page.setViewportSize({ width: 390, height: 800 });
    await page.goto(`/${src}/payments/2026-10-12-p2`);
    await expect(page.locator('.pagehead h1')).toBeVisible();
    await expect(page.getByRole('separator', { name: 'Resize the page tree' })).toBeHidden(); // the tree is a drawer there
  });
});
