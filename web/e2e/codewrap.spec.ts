import AxeBuilder from '@axe-core/playwright';
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

const LIB = '/e2e-notes';
const LONG = `curl -sS -i -X POST "$HOOK_URL" -H 'content-type: application/json' -H 'User-Agent: Payments-Webhook/1.1.0' -H "X-Webhook-Id: $WH_ID" -H 'X-Webhook-Delivery-Id: 88120045' --data-binary "$BODY"`;
const DOC = `# Code

\`\`\`bash
echo short
${LONG}
\`\`\`

Some text between.

\`\`\`json
{"deliveryId":"88120045","siteId":"123","event":"users-activated","timestamp":"2026-11-10T09:15:00Z","aVeryLongValueWithoutAnySpaceInItAtAll":"abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz"}
\`\`\`
`;
let file = '';
test.describe.configure({ mode: 'serial' });
test.use({ viewport: { width: 1280, height: 800 } });

test.beforeAll(async ({ browser }) => {
  const page = await browser.newPage();
  const libs = (await (await page.request.get('/_api/libraries')).json()) as { libraries: { id: string; root: string }[] };
  file = join(libs.libraries.find((l) => l.id === 'e2e-notes')!.root, 'code-wrap.md');
  writeFileSync(file, DOC);
  await page.close();
});
test.afterAll(() => rmSync(file, { force: true }));

const open = async (page: Page) => {
  await page.goto(`${LIB}/code-wrap`);
  await expect(page.locator('.prose .code')).toHaveCount(2);
};
const scrolls = (page: Page, n: number) => page.locator('.code pre').nth(n).evaluate((e) => e.scrollWidth > e.clientWidth + 1);
const whiteSpace = (page: Page, n: number) => page.locator('.code pre').nth(n).evaluate((e) => getComputedStyle(e).whiteSpace);

test.describe('wrapping long lines of code', () => {
  test('long lines scroll sideways by default, and each block has a switch that wraps them', async ({ page }) => {
    await open(page);
    expect(await scrolls(page, 0)).toBe(true);
    await expect(page.locator('.code pre').first()).toHaveAttribute('tabindex', '0'); // scrollable: reachable from the keyboard
    const button = page.locator('.code').first().getByRole('button', { name: 'Wrap long lines' });
    await expect(button).toHaveAttribute('aria-pressed', 'false');
    await button.click();
    await expect(button).toHaveAttribute('aria-pressed', 'true');
    expect(await whiteSpace(page, 0)).toBe('pre-wrap');
    expect(await scrolls(page, 0)).toBe(false); // the whole command is in view
    await expect(page.locator('.code pre').first()).not.toHaveAttribute('tabindex'); // nothing to scroll any more
    // the other block is untouched, and the text itself did not change
    expect(await whiteSpace(page, 1)).toBe('pre');
    expect(await page.locator('.code pre').first().evaluate((e) => (e as HTMLElement).innerText)).toContain(LONG);
    await button.click();
    await expect(button).toHaveAttribute('aria-pressed', 'false');
    expect(await scrolls(page, 0)).toBe(true);
  });

  test('copying a wrapped block still gives the original lines', async ({ page }) => {
    await open(page);
    await page.locator('.code').first().getByRole('button', { name: 'Wrap long lines' }).click();
    await page.locator('.code').first().getByRole('button', { name: 'Copy code' }).click();
    expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(`echo short\n${LONG}`);
  });

  test('the reading setting wraps every block, is remembered, and a block can still choose for itself', async ({ page }) => {
    await open(page);
    await page.getByRole('button', { name: 'Reading controls' }).click();
    await page.getByRole('group', { name: 'Long lines of code' }).getByRole('button', { name: 'Wrap' }).click();
    await page.keyboard.press('Escape');
    for (const n of [0, 1]) {
      expect(await whiteSpace(page, n)).toBe('pre-wrap');
      expect(await scrolls(page, n)).toBe(false);
    }
    await expect(page.locator('.code').first().getByRole('button', { name: 'Wrap long lines' })).toHaveAttribute('aria-pressed', 'true');
    // one block opts out
    await page.locator('.code').first().getByRole('button', { name: 'Wrap long lines' }).click();
    expect(await whiteSpace(page, 0)).toBe('pre');
    expect(await whiteSpace(page, 1)).toBe('pre-wrap');
    // remembered across a reload, and for other pages
    await page.reload();
    await expect(page.locator('.prose .code')).toHaveCount(2);
    expect(await whiteSpace(page, 0)).toBe('pre-wrap');
    expect(await page.evaluate(() => document.documentElement.hasAttribute('data-wrapcode'))).toBe(true);
    // and switching back sets every block back, whatever it had chosen
    await page.getByRole('button', { name: 'Reading controls' }).click();
    await page.getByRole('group', { name: 'Long lines of code' }).getByRole('button', { name: 'Scroll' }).click();
    await page.keyboard.press('Escape');
    for (const n of [0, 1]) expect(await whiteSpace(page, n)).toBe('pre');
  });

  test('the command palette can switch it too', async ({ page }) => {
    await open(page);
    await page.keyboard.press('Control+k');
    await page.getByRole('dialog').getByRole('option', { name: /Wrap long code lines/ }).click();
    expect(await whiteSpace(page, 0)).toBe('pre-wrap');
    await page.keyboard.press('Control+k');
    await expect(page.getByRole('dialog').getByRole('option', { name: /Scroll long code lines/ })).toBeVisible();
    await page.keyboard.press('Escape');
  });

  test('on paper the lines always wrap', async ({ page }) => {
    await open(page);
    await page.emulateMedia({ media: 'print' });
    expect(await whiteSpace(page, 0)).toBe('pre-wrap');
    await page.emulateMedia({ media: 'screen' });
  });

  for (const scheme of ['light', 'dark'] as const) {
    test(`accessible, wrapped or not (${scheme})`, async ({ page }) => {
      await page.emulateMedia({ colorScheme: scheme });
      await open(page);
      for (const state of ['scrolling', 'wrapped']) {
        if (state === 'wrapped') await page.locator('.code').first().getByRole('button', { name: 'Wrap long lines' }).click();
        const result = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa']).analyze();
        const bad = result.violations.filter((x) => x.impact === 'serious' || x.impact === 'critical');
        expect(bad.map((x) => `${x.id}: ${x.nodes.slice(0, 3).map((n) => n.target.join(' ')).join(' | ')}`), `${scheme}, ${state}`).toEqual([]);
      }
    });
  }
});
