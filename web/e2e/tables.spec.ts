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
const rows = Array.from({ length: 40 }, (_, i) => `| row ${i + 1} | value ${i + 1} | more ${i + 1} |`).join('\n');
const DOC = `# Tables

A small table that fits, and that has no reason to get more room:

| Field | Meaning |
| ----- | ------- |
| \`id\` | The order ID |
| \`status\` | Current state |

## Endpoints

| Route | Method | Auth | Request body | Response | Errors | Rate limit | Notes |
| ----- | ------ | ---- | ------------ | -------- | ------ | ---------- | ----- |
| \`/api/v2/organisations/{organisationId}/orders\` | POST | Bearer token with the orders:write scope | \`{ "customerId": string, "lines": OrderLine[], "currency": string }\` | \`201\` with the created order and a Location header | \`400\` invalid body, \`401\` no token, \`403\` missing scope, \`409\` duplicate idempotency key | 60 per minute per organisation | The idempotency key header is required; replays within 24 hours return the original response. |
| \`/api/v2/organisations/{organisationId}/orders/{orderId}\` | GET | Bearer token with the orders:read scope | none | \`200\` with the order, including its lines and payment status | \`401\` no token, \`403\` missing scope, \`404\` unknown order | 600 per minute per organisation | Supports If-None-Match for conditional reads. |
| \`/api/v2/organisations/{organisationId}/orders/export_all_pending_orders_as_csv\` | GET | Service token | none | \`200\` text/csv stream | \`401\`, \`403\` | 2 per hour | Large; use the export job instead where possible. |

## A long table

| Name | Value | Other |
| ---- | ----- | ----- |
${rows}

A table of sentences is long, not wide:

| Topic | What to know |
| ----- | ------------ |
| Idempotency | Every create call needs an idempotency key. If the same key is sent again within twenty-four hours, the original response is returned and nothing new is created, which makes retries safe. |
| Rate limits | Limits are counted per organisation, not per token, so several services sharing one organisation share one budget. Ask for a higher limit before a bulk import, not during it. |

The end.
`;

let file = '';
test.describe.configure({ mode: 'serial' });

test.beforeAll(async ({ browser }) => {
  const page = await browser.newPage();
  const libs = (await (await page.request.get('/_api/libraries')).json()) as { libraries: { id: string; root: string }[] };
  const root = libs.libraries.find((l) => l.id === 'e2e-notes')!.root;
  file = join(root, 'wide-tables.md');
  writeFileSync(file, DOC);
  await page.close();
});
test.afterAll(() => rmSync(file, { force: true }));

// The contents list is shown unless the reader hides it, and with it shown a table has the column beside it to use. These tests hide it, to look at a table with the whole page.
async function open(page: Page, contents = false) {
  await page.addInitScript((show) => localStorage.getItem('docent:prefs') === null && localStorage.setItem('docent:prefs', JSON.stringify({ toc: show })), contents); // only the first time: a reload must see what the reader chose
  await page.goto(`${LIB}/wide-tables`);
  await expect(page.locator('.prose .tablebox')).toHaveCount(4);
  await page.waitForTimeout(300); // the layout is worked out after the page has been drawn
}

test.describe('wide tables', () => {
  test.use({ viewport: { width: 1280, height: 800 } });

  test('a table wider than the text gets the whole width, and the text keeps its width', async ({ page }) => {
    await open(page);
    await expect(page.locator('.reader')).toHaveClass(/reader--wide/);
    await expect(page.locator('.toc')).toHaveCount(0); // hidden by the reader: the table has the column it would have used
    const width = (sel: string) => page.locator(sel).first().evaluate((e) => e.getBoundingClientRect().width);
    const text = await width('.prose > p');
    const wide = await page.locator('.tablebox--wide').first().evaluate((e) => e.getBoundingClientRect().width);
    const small = await page.locator('.prose > .tablebox:not(.tablebox--wide)').first().evaluate((e) => e.getBoundingClientRect().width);
    expect(wide).toBeGreaterThan(text + 150);
    expect(small).toBeLessThanOrEqual(text + 1); // a table that fits is not stretched
    // and it fits: no sideways scrolling needed to see a whole row
    const t = page.locator('.tablebox--wide .table').first();
    expect(await t.evaluate((e) => e.scrollWidth <= e.clientWidth + 1)).toBe(true);
    await expect(t).not.toHaveClass(/is-scroll/);
    await expect(t).not.toHaveAttribute('tabindex');
  });

  test('the contents list stays visible by default, and a switch in the top bar hides or shows it (and is remembered)', async ({ page }) => {
    await open(page, true);
    const toc = page.locator('.toc');
    const toggle = page.getByRole('button', { name: 'Contents list' });
    await expect(toc).toBeVisible(); // a wide table does not take it away
    await expect(toggle).toHaveAttribute('aria-pressed', 'true');
    const withToc = await page.locator('.tablebox--wide').first().evaluate((e) => e.getBoundingClientRect().width);
    await toggle.click();
    await expect(toc).toHaveCount(0);
    await expect(toggle).toHaveAttribute('aria-pressed', 'false');
    const without = await page.locator('.tablebox--wide').first().evaluate((e) => e.getBoundingClientRect().width);
    expect(without).toBeGreaterThan(withToc + 150); // the table takes the room the list gave up
    await page.reload();
    await expect(page.locator('.prose .tablebox')).toHaveCount(4);
    await expect(page.locator('.toc')).toHaveCount(0); // remembered
    await page.getByRole('button', { name: 'Contents list' }).click();
    await expect(page.locator('.toc')).toBeVisible();
    await page.reload();
    await expect(page.locator('.toc')).toBeVisible(); // and this too
  });

  test('a table of two columns of sentences wraps its cells and does not take the whole page', async ({ page }) => {
    await open(page);
    await expect(page.locator('.tablebox').nth(3)).not.toHaveClass(/tablebox--wide/);
    await expect(page.locator('.tablebox').nth(1)).toHaveClass(/tablebox--wide/);
  });

  test('long code wraps at slashes instead of widening its column', async ({ page }) => {
    await open(page);
    const code = page.locator('.tablebox--wide td code').first();
    expect(await code.evaluate((e) => e.querySelectorAll('wbr').length)).toBeGreaterThan(2);
    expect(await code.evaluate((e) => (e.textContent ?? '').includes('/api/v2/organisations/{organisationId}/orders'))).toBe(true); // the text itself is unchanged
    expect(await code.evaluate((e) => e.getBoundingClientRect().height)).toBeGreaterThan(30); // it wrapped onto several lines
  });

  test('the header stays in view while a long table scrolls past', async ({ page }) => {
    await open(page);
    const head = page.locator('.tablebox').nth(2).locator('th').first();
    await head.scrollIntoViewIfNeeded();
    await page.evaluate(() => window.scrollBy(0, 500));
    await page.waitForTimeout(150);
    const top = await head.evaluate((e) => e.getBoundingClientRect().top);
    const bar = await page.locator('.topbar').evaluate((e) => e.getBoundingClientRect().height);
    expect(top).toBeGreaterThanOrEqual(bar - 2);
    expect(top).toBeLessThan(bar + 6); // right under the top bar
    expect(await head.evaluate((e) => e.getBoundingClientRect().bottom)).toBeGreaterThan(0);
  });

  test('the enlarged view shows every column at once, with Markdown to copy', async ({ page }) => {
    await open(page);
    await expect(page.locator('.tablebox').first().getByRole('button', { name: 'Enlarge this table' })).toHaveCount(0); // two columns: no need
    const button = page.locator('.tablebox').nth(1).getByRole('button', { name: 'Enlarge this table' });
    await button.click();
    const dialog = page.getByRole('dialog', { name: 'Table: Endpoints' });
    await expect(dialog).toBeVisible();
    await expect(dialog.locator('th')).toHaveCount(8);
    const body = dialog.getByRole('region', { name: /scrolls in both directions/ });
    expect(await body.evaluate((e) => e.scrollWidth <= e.clientWidth + 1)).toBe(true);
    await dialog.getByRole('button', { name: 'Copy as Markdown' }).click();
    const md = await page.evaluate(() => navigator.clipboard.readText());
    expect(md.split('\n')[0]).toBe('| Route | Method | Auth | Request body | Response | Errors | Rate limit | Notes |');
    expect(md.split('\n')[1]).toBe('| --- | --- | --- | --- | --- | --- | --- | --- |');
    expect(md).toContain('| `/api/v2/organisations/{organisationId}/orders` | POST |');
    expect(md.split('\n')).toHaveLength(5);
    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();
    await expect(button).toBeFocused(); // back where you were
  });

  test('accessible in both themes, also with the enlarged view open', async ({ page }) => {
    for (const scheme of ['light', 'dark'] as const) {
      await page.emulateMedia({ colorScheme: scheme });
      await open(page);
      await page.locator('.tablebox').nth(1).getByRole('button', { name: 'Enlarge this table' }).click();
      await expect(page.getByRole('dialog', { name: 'Table: Endpoints' })).toBeVisible();
      for (const where of ['with the enlarged view open', 'the page']) {
        if (where === 'the page') {
          await page.keyboard.press('Escape');
          await expect(page.getByRole('dialog')).toHaveCount(0);
        }
        const result = await new AxeBuilder({ page }).withTags(['wcag2a', 'wcag2aa', 'wcag21a', 'wcag21aa']).analyze();
        const bad = result.violations.filter((x) => x.impact === 'serious' || x.impact === 'critical');
        expect(bad.map((x) => `${x.id}: ${x.nodes.slice(0, 3).map((n) => n.target.join(' ')).join(' | ')}`), `${scheme}, ${where}`).toEqual([]);
      }
    }
  });
});

test.describe('a narrow window', () => {
  test.use({ viewport: { width: 760, height: 800 } });

  test('a table that still does not fit scrolls sideways, keeps its first column in view and can be reached from the keyboard', async ({ page }) => {
    await open(page);
    const table = page.locator('.tablebox').nth(1).locator('.table');
    await expect(table).toHaveClass(/is-scroll/);
    await expect(table).toHaveAttribute('tabindex', '0');
    await expect(table).toHaveAttribute('role', 'region');
    await expect(table).toHaveAttribute('aria-label', 'Endpoints, scrolls sideways');
    await table.focus();
    await page.keyboard.press('ArrowRight');
    await page.keyboard.press('ArrowRight');
    await expect.poll(() => table.evaluate((e) => e.scrollLeft)).toBeGreaterThan(20);
    const offset = await table.evaluate((e) => e.querySelector('td')!.getBoundingClientRect().left - e.getBoundingClientRect().left);
    expect(offset).toBeLessThan(4); // the Route column did not scroll away
    const second = await table.evaluate((e) => e.querySelectorAll('td')[1]!.getBoundingClientRect().left - e.getBoundingClientRect().left);
    expect(second).toBeLessThan(await table.evaluate((e) => e.querySelectorAll('td')[1]!.offsetLeft)); // the others did
  });

  test('a window that grows brings the table back to one piece', async ({ page }) => {
    await open(page);
    const table = page.locator('.tablebox').nth(1).locator('.table');
    await expect(table).toHaveClass(/is-scroll/);
    await page.setViewportSize({ width: 1400, height: 800 });
    await expect(table).not.toHaveClass(/is-scroll/);
    await expect(page.locator('.reader')).toHaveClass(/reader--wide/);
    await page.setViewportSize({ width: 760, height: 800 });
    await expect(table).toHaveClass(/is-scroll/);
  });
});
