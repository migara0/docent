import { test, expect, type Page } from '@playwright/test';

// Visual regression: screenshots of the main views against baselines in e2e/visual.spec.ts-snapshots/ (Linux, the bundled fonts).
// After an intended design change: `pnpm exec playwright test e2e/visual.spec.ts --update-snapshots`, then look at the new images before keeping them.
const LIB = '/e2e-notes';
const DEV = '/doc-notes';

const views: { name: string; url: string; prepare?: (p: Page) => Promise<void>; unmasked?: boolean }[] = [
  {
    name: 'home',
    url: '/',
    // earlier tests leave one-off libraries and recents behind, and the path holds a random temporary folder name: show only the two permanent libraries
    prepare: (p) =>
      p.evaluate(() => {
        document.querySelectorAll('.libcard').forEach((c) => /Opened once|Dropped copies/.test(c.textContent ?? '') && c.remove());
        document.querySelectorAll('h2, ul.recent, .news').forEach((e) => e.remove());
      }),
  },
  { name: 'index-page', url: `${LIB}/index` },
  { name: 'code-and-tables', url: `${LIB}/guide/alpha` },
  { name: 'long-page', url: `${LIB}/long` },
  { name: 'diagrams', url: `${LIB}/diagrams`, prepare: (p) => p.locator('figure.diagram svg').last().waitFor() },
  { name: 'folder', url: `${LIB}/guide/` },
  { name: 'not-found', url: `${LIB}/nope/nothing` },
  {
    name: 'devnotes-overview',
    url: `${DEV}/billing-api/`,
    // "Updated" falls back to the file's modified time, so its text ("today", "5 days ago") changes the column widths from day to day: pin it
    prepare: async (p) => {
      await p.locator('.table td:nth-child(4)').first().waitFor();
      await p.locator('.table td:nth-child(4)').evaluateAll((cells) => {
        for (const c of cells) c.textContent = 'today';
      });
    },
  },
  { name: 'devnotes-note', url: `${DEV}/features/badge/qa/2026-10-11-badge-qa` },
  { name: 'devnotes-tags', url: `${DEV}/_tags` },
  // the plan note's file name holds a date relative to the day the fixture was made, so it is opened by its ID
  { name: 'plan-note', url: '/_id/DOC-0009', prepare: (p) => p.getByRole('progressbar', { name: 'Plan progress' }).waitFor() },
  {
    name: 'browse-popup',
    url: `${LIB}/guide/alpha`,
    unmasked: true, // the popup covers the page, so the dates underneath are hidden instead of masked (a mask would be drawn over the popup)
    prepare: async (p) => {
      await p.addStyleTag({ content: '.pagehead .meta, .libswitch__meta, .libcard .meta { visibility: hidden !important; }' });
      await p.keyboard.press('b');
      await p.getByRole('dialog', { name: 'Browse files' }).waitFor();
      await p.locator('.preview .excerpt').waitFor({ state: 'attached' });
      // earlier tests leave one-off libraries behind (with random folder names): show only the two permanent ones
      await p.evaluate(() => document.querySelectorAll('.col[aria-label="Libraries"] .row').forEach((r) => !/E2E notes|doc-notes/.test(r.textContent ?? '') && r.remove()));
      await p.waitForTimeout(400);
    },
  },
  {
    name: 'search',
    url: `${LIB}/index`,
    prepare: async (p) => {
      await p.keyboard.press('Control+k');
      await p.getByRole('dialog').waitFor();
      await p.keyboard.type('alpha');
      await p.waitForTimeout(300);
    },
  },
];

const sizes = [
  { tag: 'desktop', width: 1280, height: 800 },
  { tag: 'mobile', width: 390, height: 844 },
];

for (const size of sizes) {
  for (const scheme of ['light', 'dark'] as const) {
    test.describe(`${size.tag} ${scheme}`, () => {
      test.use({ viewport: { width: size.width, height: size.height }, colorScheme: scheme, reducedMotion: 'reduce' });
      for (const v of views) {
        // the search palette is the same component on mobile and the library home has no layout of its own there: one size is enough
        if (size.tag === 'mobile' && ['long-page', 'devnotes-tags'].includes(v.name)) continue;
        test(v.name, async ({ page }) => {
          await page.goto(v.url);
          await page.waitForLoadState('networkidle');
          await page.evaluate(() => document.fonts.ready);
          await v.prepare?.(page);
          await expect(page).toHaveScreenshot(`${v.name}-${size.tag}-${scheme}.png`, {
            maxDiffPixels: 60, // tight on purpose: a new icon is only a few hundred pixels
            animations: 'disabled',
            // dates shown relative to today ("2 days ago"), the modified date a note falls back to (the "Updated" column) and file sizes change from run to run
            mask: v.unmasked ? [] : [page.locator('.pagehead .meta'), page.locator('.recent-notes'), page.locator('.libcard .meta'), page.locator('.libswitch__meta'), page.locator('.libcard__path'), page.locator('.folder tbody td:nth-child(4)')],
          });
        });
      }
    });
  }
}
