import { test, expect } from '@playwright/test';
import { rmSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';

// Task items with bold, inline code and a nested list used to be laid out as flex rows, which put every run of text in its own narrow column.
const DOC = `# Checklist

- [ ] **Action sync is switched on** for the test organization, as in the handover (steps A and B): \`service.actions.payments.enabled: true\`, and the other organizations of the tenant excluded.
- [x] **A clean start:** for the test organization, \`PAYMENTS_ENABLED\` and \`PAYMENTS_ACTION_SYNC\` are **not set** in the config provider, and \`GET …/integration-action-rules/system\` answers \`[]\`.
- [ ] **Access** to the routes:
  - the connector's log
  - the tenant's database
`;
let file = '';
test.describe.configure({ mode: 'serial' });
test.use({ viewport: { width: 1280, height: 800 } });

test.beforeAll(async ({ browser }) => {
  const page = await browser.newPage();
  const libs = (await (await page.request.get('/_api/libraries')).json()) as { libraries: { id: string; root: string }[] };
  file = join(libs.libraries.find((l) => l.id === 'e2e-notes')!.root, 'task-list.md');
  writeFileSync(file, DOC);
  await page.close();
});
test.afterAll(() => rmSync(file, { force: true }));

test('task items flow as text beside their box, not as columns', async ({ page }) => {
  await page.goto('/e2e-notes/task-list');
  const items = page.locator('.prose li:has(> input[type="checkbox"])');
  await expect(items).toHaveCount(3);
  for (let i = 0; i < 3; i++) {
    const item = items.nth(i);
    const box = (await item.locator('> input').boundingBox())!;
    const bold = (await item.locator('> strong').first().boundingBox())!;
    // the bold lead-in starts on the first line, right of the box, and is not squeezed into a column
    expect(bold.x).toBeGreaterThan(box.x + box.width);
    expect(Math.abs(bold.y + bold.height / 2 - (box.y + box.height / 2))).toBeLessThan(6); // box centred on the first line
    expect(bold.height).toBeLessThan(box.height * 2.5); // one line, not a word per line
  }
  // inline code keeps its words together on one line
  const code = (await items.nth(0).locator('code').boundingBox())!;
  expect(code.height).toBeLessThan(40);
  // the text that follows the bold run continues on the same line
  const first = items.nth(0);
  const lines = await first.evaluate((li) => Math.round(li.getBoundingClientRect().height / parseFloat(getComputedStyle(li).lineHeight)));
  expect(lines).toBeLessThanOrEqual(3);
  // the nested list sits under its item's text, not beside it
  const nested = (await items.nth(2).locator('ul').boundingBox())!;
  const lead = (await items.nth(2).locator('> strong').boundingBox())!;
  expect(nested.y).toBeGreaterThan(lead.y + lead.height - 1);
});
