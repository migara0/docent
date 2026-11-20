import { test as base, expect, type Page } from '@playwright/test';
import { rmSync, writeFileSync } from 'node:fs';
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

const pad = (n: number) => String(n).padStart(2, '0');
const local = (d: Date) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
const TODAY = local(new Date());
const STATUS = 'Phase 1 built and tested; webhooks built and run in R&D; the sandbox rehearsal passed; the handover to QA is written; not yet run in a deployed environment';
const filler = Array.from({ length: 40 }, (_, i) => `Paragraph ${i + 1} of the text, long enough to scroll the page.`).join('\n\n');
// written by the test, so the file's own clock says today: the note created today gets its time of day
const FRESH = `---\ntitle: Fresh zebra note\ndate: ${TODAY}\nupdated: ${TODAY}\nstatus: "${STATUS}"\n---\n\n# Fresh zebra note\n\n${filler}\n`;
// its front matter says 2020: the file's clock disagrees, so only the day is known
const OLD = `---\ntitle: Old zebra note\ndate: 2020-01-02\nupdated: 2020-01-03\nstatus: draft\n---\n\n# Old zebra note\n\nText.\n`;

const files: string[] = [];
test.describe.configure({ mode: 'serial' });
test.use({ viewport: { width: 1280, height: 800 } });

test.beforeAll(async ({ browser }) => {
  const page = await browser.newPage();
  const libs = (await (await page.request.get('/_api/libraries')).json()) as { libraries: { id: string; root: string }[] };
  const root = libs.libraries.find((l) => l.id === 'e2e-notes')!.root;
  for (const [name, text] of [['fresh-zebra.md', FRESH], ['old-zebra.md', OLD]] as const) {
    files.push(join(root, name));
    writeFileSync(join(root, name), text);
  }
  await page.close();
});
test.afterAll(() => files.forEach((f) => rmSync(f, { force: true })));

const chip = (page: Page, label: string) => page.locator('.pagehead .chip', { hasText: label });

test.describe('created and updated', () => {
  test('show the time of day in this browser’s time zone when the file recorded it, else the day', async ({ page }) => {
    await page.goto('/e2e-notes/fresh-zebra');
    await expect(chip(page, 'Created')).toHaveText(new RegExp(`Created ${TODAY} \\d{2}:\\d{2}$`));
    await expect(chip(page, 'Updated')).toHaveText(new RegExp(`Updated ${TODAY} \\d{2}:\\d{2}$`));
    const zone = await page.evaluate(() => Intl.DateTimeFormat().resolvedOptions().timeZone);
    await expect(chip(page, 'Created')).toHaveAttribute('title', new RegExp(`\\d{2}:\\d{2}:\\d{2} \\(${zone.replace('/', '\\/')}\\)`));

    await page.goto('/e2e-notes/old-zebra');
    await expect(chip(page, 'Created')).toHaveText('Created 2020-01-02');
    await expect(chip(page, 'Updated')).toHaveText('Updated 2020-01-03');
    await expect(chip(page, 'Created')).toHaveAttribute('title', /records the day, not the time/);
  });

  test('follow the browser’s time zone', async ({ browser }) => {
    const ctx = await browser.newContext({ timezoneId: 'Pacific/Kiritimati' }); // UTC+14: a different hour from any test machine's zone
    const page = await ctx.newPage();
    const api = (await (await page.request.get('/_api/page?src=e2e-notes&path=fresh-zebra.md')).json()) as { createdAt: string };
    await page.goto('/e2e-notes/fresh-zebra');
    const want = await page.evaluate((at) => {
      const t = new Date(at);
      const p = (n: number) => String(n).padStart(2, '0');
      return `${t.getFullYear()}-${p(t.getMonth() + 1)}-${p(t.getDate())} ${p(t.getHours())}:${p(t.getMinutes())}`;
    }, api.createdAt);
    await expect(chip(page, 'Created')).toHaveText(`Created ${want}`);
    await ctx.close();
  });
});

test('a status written as a sentence wraps on its own line instead of running off the page', async ({ page }) => {
  await page.goto('/e2e-notes/fresh-zebra');
  const status = page.locator('.pagehead .chip--longstatus');
  await expect(status).toContainText('not yet run in a deployed environment');
  const [box, head] = [(await status.boundingBox())!, (await page.locator('.pagehead').boundingBox())!];
  expect(box.x + box.width).toBeLessThanOrEqual(head.x + head.width + 1);
  expect(box.height).toBeGreaterThan(30); // more than one line
  await page.goto('/e2e-notes/old-zebra');
  await expect(page.locator('.pagehead .chip--longstatus')).toHaveCount(0); // a short status stays a chip
});

test.describe('the title in the top bar', () => {
  test('appears once the title scrolls away, and goes back to the breadcrumb at the top', async ({ page }) => {
    await page.goto('/e2e-notes/fresh-zebra');
    const bar = page.locator('.topbar');
    await expect(bar.getByRole('navigation', { name: 'Breadcrumb' })).toBeVisible();
    await expect(bar.locator('.pagetitle')).toHaveCount(0);
    await page.mouse.wheel(0, 900);
    await expect(bar.locator('.pagetitle__text')).toHaveText('Fresh zebra note');
    await expect(bar.getByRole('navigation', { name: 'Breadcrumb' })).toHaveCount(0);
    await bar.locator('.pagetitle__text').click(); // back to the top
    await expect(bar.getByRole('navigation', { name: 'Breadcrumb' })).toBeVisible();
    await expect.poll(() => page.evaluate(() => scrollY)).toBeLessThan(5);
  });

  test('shows the note ID, which copies it', async ({ page, context }) => {
    await context.grantPermissions(['clipboard-read', 'clipboard-write']);
    await page.setViewportSize({ width: 1280, height: 420 });
    await page.goto('/doc-notes/features/badge/qa/2026-10-11-badge-qa');
    const id = (await page.locator('.pagehead .chip--id').innerText()).trim();
    await page.mouse.wheel(0, 600);
    const btn = page.locator('.topbar .pagetitle .chip--id');
    await expect(btn).toHaveText(id);
    await btn.click();
    await expect(page.getByRole('status').filter({ hasText: `${id} copied` })).toBeVisible();
    expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(id);
  });
});

test.describe('search by the day a page was created', () => {
  const open = async (page: Page) => {
    await page.goto('/e2e-notes/index');
    await page.keyboard.press('Control+k');
    await expect(page.getByRole('dialog', { name: 'Search' })).toBeVisible();
  };
  const titles = (page: Page) => page.locator('.palette__item .tt').allInnerTexts();

  test('Today lists the pages created today, newest first, and a search narrows them', async ({ page }) => {
    await open(page);
    const filter = page.getByRole('group', { name: 'Created' });
    await expect(filter.getByRole('button', { name: 'Any time' })).toHaveAttribute('aria-pressed', 'true');
    await filter.getByRole('button', { name: 'Today' }).click();
    await expect(filter.getByRole('button', { name: 'Today' })).toHaveAttribute('aria-pressed', 'true');
    await expect(page.locator('.palette__group').first()).toHaveText(/Created today/i);
    await expect.poll(() => titles(page)).toContain('Fresh zebra note');
    expect(await titles(page)).not.toContain('Old zebra note');
    await expect(page.getByRole('searchbox').or(page.getByRole('textbox', { name: 'Search' }))).toBeFocused();
    await page.keyboard.type('zebra');
    await expect.poll(() => titles(page)).toEqual(['Fresh zebra note']);
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(/\/e2e-notes\/fresh-zebra$/);
  });

  test('a chosen date lists the pages created that day', async ({ page }) => {
    await open(page);
    await page.getByLabel('Created on').fill('2020-01-02');
    await expect.poll(() => titles(page)).toEqual(['Old zebra note']);
    await expect(page.locator('.palette__group').first()).toHaveText(/Created on 2020-01-02/i);
    await page.getByLabel('Created on').fill('2020-01-05');
    await expect(page.locator('.palette__empty')).toHaveText('No pages were created on 2020-01-05');
    await page.getByRole('button', { name: 'Any time' }).click();
    await expect(page.locator('.palette__group').first()).not.toHaveText(/Created/i); // back to recent pages and actions
  });
});
