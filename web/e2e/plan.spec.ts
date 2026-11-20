import { test as base, expect, type Page } from '@playwright/test';
import { readFileSync, writeFileSync } from 'node:fs';

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

// The fixture plan is written by server.mjs with dates relative to today. The tests change it, so it is put back after them.
let original = '';
let file = '';

async function openPlan(page: Page) {
  const plans = (await (await page.request.get('/_api/plans')).json()) as { plans: { path: string; url: string; title: string }[] };
  const row = plans.plans.find((p) => p.title === 'Badge rollout')!;
  if (!file) {
    const view = (await (await page.request.get(`/_api/page?src=doc-notes&path=${encodeURIComponent(row.path)}`)).json()) as { file: string };
    file = view.file;
    original = readFileSync(file, 'utf8');
  }
  await page.goto(row.url);
  await expect(page.getByRole('progressbar', { name: 'Plan progress' })).toBeVisible();
  return file;
}

test.describe.configure({ mode: 'serial' });

test.describe('plan notes', () => {
  test.afterAll(() => {
    if (file) writeFileSync(file, original);
  });

  test('the board shows the whole situation: progress, stages, now, next and what is waited for', async ({ page }) => {
    await openPlan(page);
    const bar = page.getByRole('progressbar', { name: 'Plan progress' });
    await expect(bar).toHaveAttribute('aria-valuetext', '3 of 9 steps done');
    const stages = page.getByRole('list', { name: 'Stages' }).getByRole('button');
    await expect(stages).toHaveCount(4);
    await expect(stages.nth(0)).toContainText('Design');
    await expect(stages.nth(0)).toContainText('2/2');
    await expect(stages.nth(1)).toHaveAttribute('aria-current', 'step'); // the first stage that is not done
    await expect(stages.nth(3)).toContainText('waiting'); // Release is held up by the printer firmware
    // the open panel is the current stage's, with its steps
    const panel = page.getByRole('group', { name: 'Steps of Develop' });
    await expect(panel.getByText('Print endpoint')).toBeVisible();
    await expect(page.locator('.nn--now')).toContainText('Print endpoint');
    await expect(page.locator('.nn', { hasText: 'Next' })).toContainText('Validation');
    const waiting = page.locator('.plan__wait li');
    await expect(waiting).toHaveCount(2);
    await expect(waiting.first()).toContainText('Printer firmware update');
    await expect(waiting.first()).toContainText('Facilities');
    await expect(waiting.first()).toContainText('3 days');
    await expect(waiting.first()).toContainText('holds Release');
  });

  test('another stage opens on a click and the keyboard reaches it', async ({ page }) => {
    await openPlan(page);
    await page.getByRole('list', { name: 'Stages' }).getByRole('button', { name: /^Design/ }).click();
    const panel = page.getByRole('group', { name: 'Steps of Design' });
    await expect(panel).toContainText('Scope');
    await expect(panel).toContainText('Review');
    await page.getByRole('list', { name: 'Stages' }).getByRole('button', { name: /^Testing/ }).focus();
    await page.keyboard.press('Enter');
    await expect(page.getByRole('group', { name: 'Steps of Testing' })).toContainText('QA pass');
  });

  test('ticking a step asks once, then writes the note and shows the new board', async ({ page }) => {
    await openPlan(page);
    await page.evaluate(() => localStorage.removeItem('docent.planWrite'));
    await page.reload();
    await page.getByRole('button', { name: /^Mark as done: Print endpoint/ }).click();
    const ask = page.getByRole('group', { name: 'Confirm editing this note' });
    await expect(ask).toContainText('changes this note on disk');
    await expect(ask.getByRole('button', { name: 'Allow and continue' })).toBeFocused();
    expect(readFileSync(file, 'utf8')).toContain('{step: Print endpoint, state: now}'); // nothing is written before the answer
    await ask.getByRole('button', { name: 'Allow and continue' }).click();
    await expect(page.getByRole('progressbar', { name: 'Plan progress' })).toHaveAttribute('aria-valuetext', '4 of 9 steps done');
    await expect(ask).toHaveCount(0);
    const text = readFileSync(file, 'utf8');
    expect(text).toContain('{step: Print endpoint, state: done}');
    expect(text).toContain('The goal, the plan and the decisions go here'); // the rest of the note is untouched
    expect(text).toContain('author: "Test"');
    // asked once: the next tick goes straight through (and can be undone)
    await page.getByRole('button', { name: /^Mark as to do: Print endpoint/ }).click();
    await expect(page.getByRole('progressbar', { name: 'Plan progress' })).toHaveAttribute('aria-valuetext', '3 of 9 steps done');
    await expect(page.getByRole('group', { name: 'Confirm editing this note' })).toHaveCount(0);
    expect(readFileSync(file, 'utf8')).toContain('{step: Print endpoint, state: todo}');
  });

  test('a step can be started, and a waiting item cleared once it is done', async ({ page }) => {
    await openPlan(page);
    await page.evaluate(() => localStorage.setItem('docent.planWrite', '1'));
    await page.reload();
    await page.getByRole('button', { name: 'Start Validation' }).click({ force: true });
    await expect(page.locator('.nn--now')).toContainText('Validation');
    expect(readFileSync(file, 'utf8')).toContain('{step: Validation, state: now}');
    await page.getByRole('button', { name: 'No longer waiting: QA environment access' }).click();
    await expect(page.locator('.plan__wait li')).toHaveCount(1);
    expect(readFileSync(file, 'utf8')).not.toContain('QA environment access');
  });

  test('the home page lists the plans in progress, with the stages as a strip', async ({ page }) => {
    await page.goto('/');
    const section = page.getByRole('region', { name: /Active plans/ });
    await expect(section).toBeVisible();
    const row = section.locator('li.plan-row', { hasText: 'Badge rollout' });
    await expect(row).toContainText('DOC-0009');
    await expect(row.getByRole('list', { name: 'Stages' }).locator('li')).toHaveCount(4);
    await expect(row).toContainText('%');
    await row.getByRole('link', { name: /Badge rollout/ }).click();
    await expect(page.locator('.pagehead h1')).toHaveText('Badge rollout');
  });

  test('other notes have no board', async ({ page }) => {
    await page.goto('/doc-notes/features/badge/qa/2026-10-11-badge-qa');
    await expect(page.locator('.pagehead h1')).toHaveText('Badge QA');
    await expect(page.getByRole('progressbar', { name: 'Plan progress' })).toHaveCount(0);
  });
});
