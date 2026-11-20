import AxeBuilder from '@axe-core/playwright';
import { test, expect, type Page } from '@playwright/test';

// Accessibility: every main view, in the light and the dark theme, must have no serious or critical violations (WCAG 2.1 A and AA rules, including colour contrast).
const LIB = '/e2e-notes';
const DEV = '/doc-notes';

const views: { name: string; url: string; prepare?: (p: Page) => Promise<void> }[] = [
  { name: 'libraries home', url: '/' },
  { name: 'library index page', url: `${LIB}/index` },
  { name: 'page with code and tables', url: `${LIB}/guide/alpha` },
  { name: 'long page with a table of contents', url: `${LIB}/long` },
  {
    name: 'long page scrolled (back to top button)',
    url: `${LIB}/long`,
    prepare: async (p) => {
      await p.evaluate(() => scrollTo(0, 1500));
      await p.getByRole('button', { name: 'Back to top' }).waitFor();
    },
  },
  {
    name: 'page tree with the filter in use',
    url: `${DEV}/billing-api/`,
    prepare: async (p) => {
      await p.getByRole('textbox', { name: /Filter pages/ }).fill('profile');
      await p.locator('.treefilter__count').waitFor();
    },
  },
  {
    name: 'browse popup',
    url: `${LIB}/guide/alpha`,
    prepare: async (p) => {
      await p.keyboard.press('b');
      await p.getByRole('dialog', { name: 'Browse files' }).waitFor();
      await p.locator('.preview .excerpt').waitFor();
      await p.waitForTimeout(300);
    },
  },
  {
    name: 'browse popup while searching',
    url: `${LIB}/guide/alpha`,
    prepare: async (p) => {
      await p.keyboard.press('b');
      await p.getByRole('dialog', { name: 'Browse files' }).waitFor();
      await p.keyboard.type('guide');
      await p.locator('.navdialog .results .row').first().waitFor();
      await p.waitForTimeout(300);
    },
  },
  { name: 'page with diagrams', url: `${LIB}/diagrams`, prepare: (p) => p.locator('figure.diagram svg').first().waitFor() },
  { name: 'the built-in guide', url: '/help/guide' },
  { name: 'folder listing', url: `${LIB}/guide/` },
  { name: 'not found', url: `${LIB}/nope/nothing` },
  { name: 'developer notes: service overview', url: `${DEV}/billing-api/` },
  { name: 'developer notes: a note with chips', url: `${DEV}/features/badge/qa/2026-10-11-badge-qa` },
  { name: 'developer notes: tags', url: `${DEV}/_tags` },
  { name: 'developer notes: a plan note with its status board', url: '/_id/DOC-0009', prepare: (p) => p.getByRole('progressbar', { name: 'Plan progress' }).waitFor() },
  {
    name: 'search palette',
    url: `${LIB}/index`,
    prepare: async (p) => {
      await p.keyboard.press('Control+k');
      await p.getByRole('dialog').waitFor();
    },
  },
  {
    name: 'open on your phone (before sharing)',
    url: `${LIB}/guide/alpha`,
    prepare: async (p) => {
      await p.getByRole('button', { name: 'Open on your phone' }).click();
      await p.getByRole('dialog', { name: 'Open on your phone' }).waitFor();
      await p.waitForTimeout(300);
    },
  },
  {
    name: 'reading settings',
    url: `${LIB}/guide/alpha`,
    prepare: async (p) => {
      await p.getByRole('button', { name: /reading/i }).first().click();
    },
  },
];

for (const scheme of ['light', 'dark'] as const) {
  test.describe(`accessibility, ${scheme} theme`, () => {
    test.use({ colorScheme: scheme });
    for (const v of views) {
      test(v.name, async ({ page }) => {
        await page.goto(v.url);
        await page.waitForLoadState('networkidle');
        await v.prepare?.(page);
        const result = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa']).analyze();
        const bad = result.violations.filter((x) => x.impact === 'serious' || x.impact === 'critical');
        const report = bad.map((x) => `${x.id} (${x.impact}): ${x.help}\n    ${x.nodes.slice(0, 3).map((n) => n.target.join(' ')).join('\n    ')}`);
        expect(report, `${v.name} / ${scheme}`).toEqual([]);
      });
    }
  });
}
