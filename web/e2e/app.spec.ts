import { test as base, expect, type Page } from '@playwright/test';
import { mkdtempSync, mkdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { basename, join } from 'node:path';

// Every test also fails on any error the browser reports.
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

const LIB = '/e2e-notes';
const libraryRoot = async (page: Page) => {
  const res = await page.request.get('/_api/libraries');
  const libs = (await res.json()).libraries as { id: string; root: string }[];
  return libs.find((l) => l.id === 'e2e-notes')!.root;
};

test.describe('home and navigation', () => {
  test('home lists the registered library, and opening it shows its front page', async ({ page }) => {
    await page.goto('/');
    await expect(page.getByRole('heading', { name: 'Your libraries' })).toBeVisible();
    await expect(page.locator('.libcard', { hasText: 'E2E notes' })).toContainText('6 pages');
    await page.locator('.libcard', { hasText: 'E2E notes' }).click();
    await expect(page).toHaveURL(`${LIB}/`); // the library's index page is shown at the library address
    await expect(page.locator('.pagehead h1')).toHaveText('E2E library');
    await expect(page.locator('.tree a[aria-current="page"]')).toContainText('E2E library');
  });

  test('internal links navigate without a page reload; broken and file links do nothing', async ({ page }) => {
    await page.goto(`${LIB}/index`);
    await page.evaluate(() => ((window as unknown as { __kept: boolean }).__kept = true));
    await page.locator('.prose a.internal', { hasText: 'Alpha guide' }).click();
    await expect(page).toHaveURL(`${LIB}/guide/alpha`);
    await expect(page.locator('.pagehead h1')).toHaveText('Alpha guide');
    expect(await page.evaluate(() => (window as unknown as { __kept?: boolean }).__kept)).toBe(true); // same document: no reload
    await page.goBack();
    await expect(page).toHaveURL(`${LIB}/index`);
    await expect(page.locator('.pagehead h1')).toHaveText('E2E library');

    const broken = page.locator('.prose a.broken');
    await expect(broken).toHaveCount(1);
    await expect(broken).toHaveAttribute('title', /Page not found/);
    await broken.click();
    await expect(page).toHaveURL(`${LIB}/index`);
    await expect(page.locator('.prose a.link-file')).toHaveCount(1);
  });

  test('a link with a section opens the page and scrolls to it', async ({ page }) => {
    await page.goto(`${LIB}/guide/alpha`);
    await page.locator('.prose a.internal', { hasText: 'Beta guide' }).click();
    await expect(page).toHaveURL(`${LIB}/guide/beta#details`);
    await expect(page.locator('#details')).toBeInViewport();
  });

  test('unknown pages and libraries say so instead of failing', async ({ page }) => {
    await page.goto(`${LIB}/nope/nothing`);
    await expect(page.getByRole('heading', { name: 'Nothing here' })).toBeVisible();
    await page.goto('/no-such-library/');
    await expect(page.getByText('There is no open library')).toBeVisible();
  });

  test('a unicode file name works in the tree, the address bar and a reload', async ({ page }) => {
    await page.goto(`${LIB}/index`);
    await page.getByRole('button', { name: /^notes/ }).click();
    await page.getByRole('link', { name: 'Café menu' }).click();
    await expect(page.locator('.pagehead h1')).toHaveText('Café menu');
    await page.reload();
    await expect(page.locator('.pagehead h1')).toHaveText('Café menu');
    await expect(page.locator('.tree a[aria-current="page"]')).toContainText('Café menu');
  });

  test('the tree opens to the current page and remembers open folders', async ({ page }) => {
    await page.goto(`${LIB}/guide/beta`);
    await expect(page.locator('.tree a[aria-current="page"]')).toContainText('Beta guide');
    await page.reload();
    await expect(page.locator('.tree a[aria-current="page"]')).toContainText('Beta guide');
  });
});

test.describe('reading a page', () => {
  test('shows the metadata, a table of contents that follows the scroll, and rendered content', async ({ page }) => {
    await page.goto(`${LIB}/guide/alpha`);
    await expect(page.locator('.chip--id')).toHaveText('E2E-0001');
    await expect(page.locator('.meta')).toContainText('stable');
    await expect(page.locator('.pagehead__lede')).toHaveText('The first guide, used to test reading.');
    await expect(page.locator('.callout')).toHaveCount(2); // the GitHub alert and the !!! warning
    await expect(page.locator('.table table')).toHaveCount(1);
    await expect(page.locator('.code[data-lang="ts"] .name')).toHaveText('example.ts');
    await expect(page.locator('.prose input[type="checkbox"]')).toHaveCount(2);
    await expect(page.locator('.toc a')).toHaveCount(5);
    await page.goto(`${LIB}/long`);
    await page.locator('#section-6').evaluate((e) => e.scrollIntoView()); // the heading reaches the top: it is the one being read
    await expect(page.locator('.toc a.is-active')).toHaveText('Section 6');
    await page.evaluate(() => scrollTo(0, document.documentElement.scrollHeight));
    await expect(page.locator('.toc a.is-active')).toHaveText('Section 8'); // at the end of the page the last section is active
  });

  test('copy buttons work: code, ID and the whole page as Markdown', async ({ page }) => {
    await page.goto(`${LIB}/guide/alpha`);
    const clipboard = () => page.evaluate(() => navigator.clipboard.readText());
    await page.locator('.code .copy').click();
    await expect.poll(clipboard).toContain('export const answer: number = 42;');
    await page.locator('.chip--id').click();
    await expect.poll(clipboard).toBe('E2E-0001');
    await page.getByRole('button', { name: 'Copy page as Markdown' }).click();
    const raw = await (await page.request.get('/_api/raw?src=e2e-notes&path=guide/alpha.md')).text();
    await expect.poll(clipboard).toBe(raw);
  });

  test('a single section can be copied as Markdown', async ({ page }) => {
    await page.goto(`${LIB}/guide/alpha`);
    const heading = page.locator('.prose h2', { hasText: 'Endpoints' });
    await heading.hover();
    await heading.locator('.section-copy').click();
    await expect.poll(() => page.evaluate(() => navigator.clipboard.readText())).toMatch(/^## Endpoints/);
    const copied = await page.evaluate(() => navigator.clipboard.readText());
    expect(copied).toContain('| `POST /b` | Second |');
    expect(copied).not.toContain('## Code'); // it stops at the next section
  });

  test('previous and next, and who links here', async ({ page }) => {
    await page.goto(`${LIB}/guide/beta`);
    await expect(page.locator('.related')).toContainText('Alpha guide'); // alpha links to beta
    await expect(page.locator('.pager')).toContainText('Previous');
  });

  test('a page changed on disk updates live, and a new file appears in the tree', async ({ page }) => {
    const root = await libraryRoot(page);
    const file = join(root, 'guide', 'alpha.md');
    const original = readFileSync(file, 'utf8');
    try {
      await page.goto(`${LIB}/guide/alpha`);
      await expect(page.locator('.prose')).toContainText('The end of the page.');
      writeFileSync(file, original.replace('The end of the page.', 'The end, changed while you were reading.'));
      await expect(page.locator('.prose')).toContainText('changed while you were reading', { timeout: 8000 });
      writeFileSync(join(root, 'guide', 'gamma.md'), '# Gamma guide\n\nNew.\n');
      await expect(page.locator('.tree a', { hasText: 'Gamma guide' })).toBeVisible({ timeout: 8000 });
    } finally {
      writeFileSync(file, original);
      rmSync(join(root, 'guide', 'gamma.md'), { force: true });
    }
  });
});

test.describe('search', () => {
  test('Ctrl+K searches titles and body text, Enter opens the result', async ({ page }) => {
    await page.goto(`${LIB}/index`);
    await page.keyboard.press('Control+k');
    await expect(page.getByRole('dialog', { name: 'Search' })).toBeVisible();
    await page.getByRole('textbox', { name: 'Search' }).fill('alpha');
    await expect(page.locator('.palette__item').first()).toContainText('Alpha guide');
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(`${LIB}/guide/alpha`);

    await page.keyboard.press('Control+k');
    await page.getByRole('textbox', { name: 'Search' }).fill('brulee'); // typo tolerant, and found by body text
    await page.getByRole('textbox', { name: 'Search' }).fill('crème');
    await expect(page.locator('.palette__item').first()).toContainText('Café menu');
  });

  test('the slash key opens it, an empty search shows recent pages and actions, Escape closes', async ({ page }) => {
    await page.goto(`${LIB}/guide/alpha`);
    await page.keyboard.press('/');
    await expect(page.getByRole('dialog', { name: 'Search' })).toBeVisible();
    await expect(page.locator('.palette__group', { hasText: 'Actions' })).toBeVisible();
    await expect(page.locator('.palette__group', { hasText: 'Recent pages' })).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(page.getByRole('dialog', { name: 'Search' })).toBeHidden();
  });

  test('a search with no match says so', async ({ page }) => {
    await page.goto(`${LIB}/index`);
    await page.keyboard.press('Control+k');
    await page.getByRole('textbox', { name: 'Search' }).fill('zzzzqqqq');
    await expect(page.getByText('No pages match')).toBeVisible();
  });
});

test.describe('reading controls', () => {
  test('text size changes the page text, and the choice survives a reload', async ({ page }) => {
    await page.goto(`${LIB}/guide/alpha`);
    const size = () => page.locator('.prose').evaluate((e) => parseFloat(getComputedStyle(e).fontSize));
    const before = await size();
    await page.getByRole('button', { name: 'Reading controls' }).click();
    await page.getByRole('button', { name: 'A+ larger text' }).click();
    await page.getByRole('button', { name: 'A+ larger text' }).click();
    await expect(page.locator('.popover output')).toHaveText('120%');
    expect(await size()).toBeCloseTo(before * 1.2, 0);
    await page.getByRole('button', { name: 'Wide' }).click();
    await page.reload();
    expect(await size()).toBeCloseTo(before * 1.2, 0);
    expect(await page.evaluate(() => document.documentElement.dataset.width)).toBe('wide');
  });

  test('theme: dark, light, and it is remembered', async ({ page }) => {
    await page.goto(`${LIB}/guide/alpha`);
    await page.getByRole('button', { name: 'Reading controls' }).click();
    await page.getByRole('button', { name: 'Dark' }).click();
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
    expect(await page.evaluate(() => getComputedStyle(document.body).backgroundColor)).not.toBe('rgb(255, 255, 255)');
    await page.reload();
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
    await page.getByRole('button', { name: 'Reading controls' }).click();
    await page.getByRole('button', { name: 'Light' }).click();
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');
  });

  test('focus mode hides the sidebars and the contents list', async ({ page }) => {
    await page.goto(`${LIB}/guide/alpha`);
    await expect(page.locator('.rail')).toBeVisible();
    await page.getByRole('button', { name: 'Focus mode' }).click();
    await expect(page.locator('.rail')).toBeHidden();
    await expect(page.locator('.toc')).toHaveCount(0);
    await page.getByRole('button', { name: 'Focus mode' }).click();
    await expect(page.locator('.rail')).toBeVisible();
  });
});

test.describe('diagrams', () => {
  test('are drawn, errors are shown in place, and they can be enlarged and opened in a window', async ({ page, context }) => {
    await page.goto(`${LIB}/diagrams`);
    await expect(page.locator('figure.diagram')).toHaveCount(2);
    await expect(page.locator('figure.diagram svg').first()).toBeVisible({ timeout: 20_000 });
    await expect(page.locator('.diagram__error')).toHaveCount(1);
    await expect(page.locator('pre.mermaid')).toHaveCount(0);
    const good = page.locator('figure.diagram').first();

    await good.getByRole('button', { name: 'Enlarge' }).click();
    const dlg = page.locator('dialog.diagram-zoom');
    await expect(dlg).toBeVisible();
    await dlg.locator('.diagram-zoom__view').hover();
    await page.mouse.wheel(0, -300);
    await expect(dlg.locator('.diagram-zoom__stage')).toHaveAttribute('style', /scale\(1\.\d+/);
    await page.keyboard.press('Escape');
    await expect(dlg).toBeHidden();

    const [popup] = await Promise.all([context.waitForEvent('page'), good.getByRole('button', { name: 'Open in a new window' }).click()]);
    await expect(popup.locator('#stage svg')).toBeVisible();
    await popup.locator('#search').fill('browser');
    await expect(popup.locator('#count')).toHaveText(/1 of 1|1 found/);
  });

  test('are redrawn in the dark theme', async ({ page }) => {
    await page.goto(`${LIB}/diagrams`);
    const fill = () => page.locator('figure.diagram svg g.node rect, figure.diagram svg g.node polygon, figure.diagram svg g.node path').first().evaluate((e) => getComputedStyle(e).fill);
    await expect(page.locator('figure.diagram svg').first()).toBeVisible({ timeout: 20_000 });
    const light = await fill();
    await page.getByRole('button', { name: 'Reading controls' }).click();
    await page.getByRole('button', { name: 'Dark' }).click();
    await expect.poll(fill, { timeout: 10_000 }).not.toBe(light);
  });
});

test.describe('print', () => {
  test('the chooser leaves out marked and related sections by default, and only while printing', async ({ page }) => {
    await page.goto(`${LIB}/guide/beta`);
    await page.getByRole('button', { name: 'Print or save as PDF' }).click();
    const dlg = page.getByRole('dialog', { name: 'Choose what to print' });
    await expect(dlg).toBeVisible();
    await expect(dlg.locator('label', { hasText: 'Internal notes' })).toContainText('omitted by default');
    await expect(dlg.locator('label', { hasText: 'Internal notes' }).locator('input')).not.toBeChecked();
    await expect(dlg.locator('label', { hasText: 'Related notes' }).locator('input')).not.toBeChecked();
    await expect(dlg.locator('label', { hasText: 'Overview' }).locator('input')).toBeChecked();
    await dlg.getByLabel('Details', { exact: true }).uncheck();

    await page.evaluate(() => {
      const w = window as unknown as { __printed?: string[]; __header?: string };
      window.print = () => {
        window.dispatchEvent(new Event('beforeprint'));
        w.__printed = [...document.querySelectorAll('.print-hidden')].map((e) => (e.textContent ?? '').trim().slice(0, 24));
        w.__header = document.head.querySelector('style:last-of-type')?.textContent ?? '';
        window.dispatchEvent(new Event('afterprint'));
      };
    });
    await dlg.getByRole('button', { name: 'Print' }).click();
    await expect.poll(() => page.evaluate(() => (window as unknown as { __printed?: string[] }).__printed)).toBeTruthy();
    const printed = await page.evaluate(() => (window as unknown as { __printed: string[] }).__printed);
    expect(printed.join('|')).toContain('Internal notes');
    expect(printed.join('|')).toContain('Related notes');
    expect(printed.join('|')).toContain('Details');
    expect(printed.join('|')).not.toContain('Overview');
    expect(await page.evaluate(() => (window as unknown as { __header: string }).__header)).toContain('@top-left');
    await expect(page.locator('.print-hidden')).toHaveCount(0); // everything is shown again afterwards
  });
});

test.describe('opening folders', () => {
  test('open a folder once, then add another to the libraries; the added one is still there after a reload', async ({ page }) => {
    const once = mkdtempSync(join(tmpdir(), 'docent-once-'));
    const keep = mkdtempSync(join(tmpdir(), 'docent-keep-'));
    const keepName = `Kept ${basename(keep)}`; // unique, so a repeat or an earlier failure cannot confuse this test
    writeFileSync(join(once, 'one.md'), '# Opened once\n\nHello.\n');
    mkdirSync(join(keep, 'inner'));
    writeFileSync(join(keep, 'inner', 'deep.md'), '# Kept page\n\nStays.\n');
    try {
      await page.goto('/');
      await page.getByRole('button', { name: 'Open a folder or file' }).first().click();
      const dlg = page.getByRole('dialog', { name: 'Open a folder or file' });
      await dlg.getByLabel('Folder path').fill(once);
      await dlg.getByLabel('Folder path').press('Enter');
      await expect(dlg.locator('.picker__row', { hasText: 'one.md' })).toBeVisible();
      await dlg.getByRole('button', { name: 'Open once' }).click();
      await expect(page.locator('.libswitch__meta')).toContainText('opened once');
      await expect(page.locator('.tree')).toContainText('Opened once');

      await page.goto('/');
      await page.getByRole('button', { name: 'Open a folder or file' }).first().click();
      await dlg.getByLabel('Folder path').fill(keep);
      await dlg.getByLabel('Folder path').press('Enter');
      await expect(dlg.locator('.picker__row', { hasText: 'inner' })).toContainText('1 .md');
      await dlg.getByLabel('Name').fill(keepName);
      await dlg.getByRole('button', { name: 'Add to libraries' }).click();
      await expect(page.locator('.libswitch__name')).toHaveText(keepName);

      await page.goto('/');
      await page.reload();
      await expect(page.locator('.libcard', { hasText: keepName })).toBeVisible();
      await expect(page.locator('.libcard', { hasText: basename(once) })).toHaveCount(1); // the one opened once is still open in this run
      const libs = (await (await page.request.get('/_api/libraries')).json()).libraries as { kind: string; name: string }[];
      expect(libs.filter((l) => l.kind === 'library').map((l) => l.name)).toContain(keepName);
      expect(libs.find((l) => l.name === basename(once))?.kind).toBe('folder'); // not registered: it will not be there next time

      await page.locator('.libcard', { hasText: keepName }).getByRole('button', { name: /Remove/ }).click();
      await expect(page.locator('.libcard', { hasText: keepName })).toHaveCount(0);
      expect(readFileSync(join(keep, 'inner', 'deep.md'), 'utf8')).toContain('Kept page'); // the folder itself is never touched
    } finally {
      const libs = (await (await page.request.get('/_api/libraries')).json()).libraries as { id: string; name: string; kind: string }[];
      for (const l of libs.filter((x) => x.name === keepName && x.kind === 'library')) await page.request.delete(`/_api/libraries/${l.id}`);
      rmSync(once, { recursive: true, force: true });
      rmSync(keep, { recursive: true, force: true });
    }
  });

  test('a single Markdown file can be opened', async ({ page }) => {
    const dir = mkdtempSync(join(tmpdir(), 'docent-file-'));
    writeFileSync(join(dir, 'solo.md'), '# Solo page\n\nJust me.\n');
    try {
      await page.goto('/');
      await page.getByRole('button', { name: 'Open a folder or file' }).first().click();
      const dlg = page.getByRole('dialog', { name: 'Open a folder or file' });
      await dlg.getByLabel('Folder path').fill(join(dir, 'solo.md'));
      await dlg.getByLabel('Folder path').press('Enter');
      await expect(dlg.getByRole('button', { name: 'Add to libraries' })).toBeDisabled(); // only folders are added
      await dlg.getByRole('button', { name: 'Open once' }).click();
      await expect(page.locator('.pagehead h1')).toHaveText('Solo page');
    } finally {
      rmSync(dir, { recursive: true, force: true });
    }
  });

  test('a path that does not exist is reported, not a crash', async ({ page, errors }) => {
    await page.goto('/');
    await page.getByRole('button', { name: 'Open a folder or file' }).first().click();
    const dlg = page.getByRole('dialog', { name: 'Open a folder or file' });
    await dlg.getByLabel('Folder path').fill('/definitely/not/here');
    await dlg.getByLabel('Folder path').press('Enter');
    await expect(dlg).toContainText('is not a folder');
    errors.length = 0; // the 404 for the missing folder is the expected answer, and the browser logs every 404
  });
});

test.describe('phone', () => {
  test.use({ viewport: { width: 390, height: 844 } });

  test('the sidebar is a drawer, closes after choosing a page, and nothing scrolls sideways', async ({ page }) => {
    await page.goto(`${LIB}/guide/alpha`);
    await expect(page.locator('.rail')).toBeHidden();
    await page.getByRole('button', { name: 'Open navigation' }).click();
    await expect(page.locator('.rail')).toBeVisible();
    await page.locator('.tree a', { hasText: 'Beta guide' }).click();
    await expect(page.locator('.pagehead h1')).toHaveText('Beta guide');
    await expect(page.locator('.rail')).toBeHidden();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    await page.goto(`${LIB}/guide/alpha`);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true); // the table scrolls inside its own box
  });
});

test.describe('back to top', () => {
  test('a button appears after scrolling a long page and takes you back to the top', async ({ page }) => {
    await page.goto('/e2e-notes/long');
    await expect(page.getByRole('button', { name: 'Back to top' })).toHaveCount(0);
    await page.evaluate(() => scrollTo(0, 1500));
    const button = page.getByRole('button', { name: 'Back to top' });
    await expect(button).toBeVisible();
    await button.click();
    await expect.poll(() => page.evaluate(() => scrollY)).toBe(0);
    await expect(button).toHaveCount(0);
    // a short page never shows it
    await page.goto('/e2e-notes/guide/beta');
    await page.evaluate(() => scrollTo(0, 5000));
    await expect(button).toHaveCount(0);
  });
});
