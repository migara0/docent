import AxeBuilder from '@axe-core/playwright';
import { test as base, expect, type Page } from '@playwright/test';
import { mkdirSync, rmSync, writeFileSync } from 'node:fs';
import { dirname, join } from 'node:path';

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

const today = () => {
  const d = new Date();
  const p = (n: number) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`;
};

// Notes are added while Docent is open by writing files, as a Claude session or `docent new` does. They are removed afterwards: other tests count the notes.
const created: string[] = [];
test.afterAll(() => created.forEach((f) => rmSync(f, { force: true })));

async function libraryRoot(page: Page): Promise<string> {
  const plans = (await (await page.request.get('/_api/plans')).json()) as { plans: { path: string }[] };
  const view = (await (await page.request.get(`/_api/page?src=doc-notes&path=${encodeURIComponent(plans.plans[0]!.path)}`)).json()) as { file: string };
  return dirname(dirname(dirname(dirname(view.file)))); // .../features/badge/plan/<file>.md -> the library folder
}

function addNote(root: string, id: string, title: string, type = 'adr') {
  const dir = join(root, 'billing-api', type);
  mkdirSync(dir, { recursive: true });
  const file = join(dir, `${today()}-${title.toLowerCase().replace(/[^a-z0-9]+/g, '-')}.md`);
  writeFileSync(file, `---\ntitle: "${title}"\nid: ${id}\ndate: ${today()}\ntags:\n  - billing-api\n  - ${type}\n---\n\n# ${title}\n\nBody.\n`);
  created.push(file);
  return file;
}

test.describe.configure({ mode: 'serial' });

test.describe('new notes', () => {
  test('a note added while Docent is open shows a toast, a dot in the tree, a count and a place on the home page; opening it clears them', async ({ page }) => {
    const root = await libraryRoot(page);
    const first = page.waitForResponse('**/_api/latest');
    await page.goto('/doc-notes/features/badge/qa/2026-10-11-badge-qa');
    await first;
    await expect(page.locator('.tree .newdot')).toHaveCount(0); // what was there at the first look counts as seen

    addNote(root, 'DOC-0010', 'Paging cursor decision');
    const toast = page.getByRole('status').filter({ hasText: 'New note DOC-0010' });
    await expect(toast).toContainText('Paging cursor decision');
    await expect(toast.getByRole('button', { name: 'Open' })).toBeVisible();
    // the folder that holds it is closed, so the dot is on the folder
    await expect(page.locator('.tree button.tree__row', { hasText: 'billing-api' }).locator('.newdot')).toHaveCount(1);

    // the library menu and the home page say how many are new
    await page.goto('/');
    await expect(page.locator('.chip--new', { hasText: '1 new' })).toBeVisible();
    const news = page.getByRole('region', { name: /What's new/ });
    await expect(news.locator('li.is-new')).toHaveCount(1);
    await expect(news.locator('li.is-new')).toContainText('Paging cursor decision');
    await expect(news.locator('li.is-new')).toContainText('DOC-0010');

    // opening it marks it seen, and that is remembered after a reload
    await news.getByRole('link', { name: /Paging cursor decision/ }).click();
    await expect(page.locator('.pagehead h1')).toHaveText('Paging cursor decision');
    await expect(page.locator('.tree .newdot')).toHaveCount(0);
    await page.reload();
    await expect(page.locator('.pagehead h1')).toBeVisible();
    await page.goto('/');
    await expect(page.locator('.chip--new')).toHaveCount(0);
    await expect(news.locator('li.is-new')).toHaveCount(0);
    await expect(news).toContainText('You have seen all of these');
  });

  test('the toast button opens the note, and N opens the newest note you have not seen', async ({ page }) => {
    const root = await libraryRoot(page);
    const first = page.waitForResponse('**/_api/latest');
    await page.goto('/doc-notes/features/badge/qa/2026-10-11-badge-qa');
    await first;

    addNote(root, 'DOC-0011', 'Older new note');
    await page.getByRole('status').filter({ hasText: 'DOC-0011' }).waitFor();
    addNote(root, 'DOC-0012', 'Newest new note', 'runbook');
    const toast = page.getByRole('status').filter({ hasText: 'DOC-0012' });
    await expect(toast).toBeVisible();
    await toast.getByRole('button', { name: 'Open' }).click();
    await expect(page.locator('.pagehead h1')).toHaveText('Newest new note');

    await page.goto('/');
    await page.keyboard.press('n'); // the one still unseen
    await expect(page.locator('.pagehead h1')).toHaveText('Older new note');
    await page.goto('/');
    await page.keyboard.press('n');
    await expect(page.getByRole('status').filter({ hasText: 'No new notes' })).toBeVisible();
  });

  test('the search box shows the new notes before anything is typed, and "Mark all as seen" clears them', async ({ page }) => {
    const root = await libraryRoot(page);
    const first = page.waitForResponse('**/_api/latest');
    await page.goto('/');
    await first;
    addNote(root, 'DOC-0013', 'Palette new note');
    await expect(page.locator('.chip--new', { hasText: '1 new' })).toBeVisible();
    await page.keyboard.press('Control+k');
    const palette = page.getByRole('dialog');
    await expect(palette.getByText('New notes', { exact: true })).toBeVisible();
    await expect(palette.getByRole('option', { name: /Palette new note/ })).toBeVisible();
    await page.keyboard.press('Escape');

    await page.getByRole('button', { name: 'Mark all as seen' }).click();
    await expect(page.locator('.chip--new')).toHaveCount(0);
    await page.keyboard.press('Control+k');
    await expect(page.getByRole('dialog').getByText('New notes', { exact: true })).toHaveCount(0);
  });

  for (const scheme of ['light', 'dark'] as const) {
    test(`the new-note markers, the list and the toast are accessible (${scheme})`, async ({ page }) => {
      await page.emulateMedia({ colorScheme: scheme });
      const root = await libraryRoot(page);
      const first = page.waitForResponse('**/_api/latest');
      await page.goto('/doc-notes/features/badge/qa/2026-10-11-badge-qa');
      await first;
      addNote(root, `DOC-00${scheme === 'light' ? 20 : 21}`, `Accessible new note ${scheme}`);
      await page.getByRole('status').filter({ hasText: 'Accessible new note' }).waitFor();
      await page.locator('.tree .newdot').first().waitFor();
      const check = async (where: string) => {
        const result = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa']).analyze();
        const bad = result.violations.filter((x) => x.impact === 'serious' || x.impact === 'critical');
        expect(bad.map((x) => `${x.id}: ${x.nodes.slice(0, 3).map((n) => n.target.join(' ')).join(' | ')}`), where).toEqual([]);
      };
      await check('a note page with a new-note dot and the toast');
      await page.goto('/');
      await expect(page.locator('.news li.is-new')).toBeVisible();
      await check('the home page with the what\'s-new list and a count chip');
    });
  }

  test('the first look at a library sets the line: nothing that was already there is new', async ({ page }) => {
    await page.goto('/');
    await expect(page.getByRole('region', { name: /Active plans/ })).toBeVisible();
    await expect(page.locator('.chip--new')).toHaveCount(0);
    await expect(page.locator('.libswitch .newdot')).toHaveCount(0);
  });
});
