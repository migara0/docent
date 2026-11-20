import { test as base, expect } from '@playwright/test';

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

const LIB = '/doc-notes';

test.describe('developer notes', () => {
  test('a service folder is an overview of its notes grouped by type, in a sensible order', async ({ page }) => {
    await page.goto(`${LIB}/billing-api/`);
    await expect(page.locator('h1')).toHaveText('billing-api');
    const headings = await page.locator('.folder h2').allTextContents();
    expect(headings.map((h) => h.replace(/\d+$/, '').trim())).toEqual(['Module', 'Implementation', 'Bugfix', 'Commits']);
    const row = page.locator('tr', { hasText: 'Null profile fix' });
    await expect(row).toContainText('DOC-0002');
    await expect(row).toContainText('2026-10-05');
    await row.getByRole('link', { name: 'Null profile fix' }).click();
    await expect(page.locator('.pagehead h1')).toHaveText('Null profile fix');
  });

  test('features and technologies are overviews too, and their roots list their folders', async ({ page }) => {
    await page.goto(`${LIB}/features/badge/`);
    await expect(page.locator('tr', { hasText: 'Badge QA' })).toContainText('DOC-0005');
    await page.goto(`${LIB}/features/`);
    await expect(page.locator('.recent a', { hasText: 'badge' })).toBeVisible();
  });

  test('the library front shows the folders and the recently updated notes, newest first', async ({ page }) => {
    await page.goto(`${LIB}/`);
    await expect(page.locator('.recent-notes h2')).toHaveText('Recently updated');
    const dates = await page.locator('.recent-notes tbody tr td:nth-child(5)').allTextContents();
    expect(dates.length).toBeGreaterThan(3);
    expect([...dates].sort().reverse()).toEqual(dates);
  });

  test('a note shows where it lives, its kind of note and its tags (which lead to the tag view)', async ({ page }) => {
    await page.goto(`${LIB}/features/badge/qa/2026-10-11-badge-qa`);
    await expect(page.locator('.meta')).toContainText('Feature badge');
    await expect(page.locator('.meta')).toContainText('Type qa');
    await expect(page.locator('.chip--id')).toHaveText('DOC-0005');
    await expect(page.locator('.meta a.chip', { hasText: /^badge$/ })).toHaveCount(1); // the note lists its tags twice: each is shown once, and the page still opens
    await page.locator('.meta a.chip', { hasText: 'badge' }).click();
    await expect(page).toHaveURL(`${LIB}/_tags/badge`);
    await expect(page.locator('h1')).toHaveText('Tag: badge');
    await expect(page.locator('tbody tr')).toHaveCount(3); // the implementation, the QA note and the plan
  });

  test('a technology guide shows its versions and when it was verified', async ({ page }) => {
    await page.goto(`${LIB}/technologies/kafkajs/guide/2026-10-12-kafkajs`);
    await expect(page.locator('.meta')).toContainText('Technology kafkajs');
    await expect(page.locator('.meta')).toContainText('Versions 2.2.4');
    await expect(page.locator('.meta')).toContainText('Verified 2026-10-12');
  });

  test('a note ID in the text is a link to that note; an ID that does not exist is a warning', async ({ page }) => {
    await page.goto(`${LIB}/billing-api/implementation/2026-10-01-profile-events`);
    await expect(page.locator('.prose a.note-ref')).toHaveCount(3); // DOC-0002, DOC-0003 and DOC-0004 (under "Related notes")
    await expect(page.locator('.prose code', { hasText: 'DOC-0002' })).toBeVisible(); // in code: plain
    await expect(page.locator('.callout--warn', { hasText: 'DOC-0099' })).toBeVisible();
    await page.locator('.prose a.note-ref', { hasText: 'DOC-0002' }).click();
    await expect(page).toHaveURL(`${LIB}/billing-api/bugfix/2026-10-05-null-profile`);
    await expect(page.locator('.related')).toContainText('Profile events'); // it mentions DOC-0002: a backlink by ID
  });

  test('a module note whose code changed after it was updated carries a notice naming the files', async ({ page }) => {
    await page.goto(`${LIB}/billing-api/module/2026-10-02-profiles`);
    const notice = page.locator('.callout--warn', { hasText: 'Code changed since this note was last updated' });
    await expect(notice).toBeVisible();
    await expect(notice).toContainText('billing-api/src/profiles');
    await expect(notice).toContainText('last changed 2026-11-04');
    await page.goto(`${LIB}/billing-api/bugfix/2026-10-05-null-profile`);
    await expect(page.locator('.callout--warn', { hasText: 'Code changed' })).toHaveCount(0); // only module notes are checked
  });

  test('the tag view lists every tag with its count, and the palette can open it', async ({ page }) => {
    await page.goto(`${LIB}/billing-api/`);
    await page.keyboard.press('Control+k');
    await page.getByRole('textbox', { name: 'Search' }).fill('tags');
    await page.locator('.palette__item', { hasText: 'Browse tags' }).click();
    await expect(page).toHaveURL(`${LIB}/_tags`);
    await expect(page.locator('.meta a.chip', { hasText: 'billing-api' })).toContainText('4');
    await page.locator('.meta a.chip', { hasText: 'billing-api' }).click();
    await expect(page.locator('tbody tr')).toHaveCount(4);
  });

  test('a plain library has no developer-note extras', async ({ page }) => {
    await page.goto('/e2e-notes/guide/alpha');
    await expect(page.locator('.meta')).not.toContainText('Service');
    await page.goto('/e2e-notes/guide/');
    await expect(page.locator('.folder table')).toHaveCount(0);
  });

  test('typing a note ID in the palette jumps to the note, in any form', async ({ page }) => {
    for (const typed of ['DOC-0005', 'doc-5', 'doc 5', '5']) {
      await page.goto(`${LIB}/billing-api/`);
      await page.keyboard.press('Control+k');
      await expect(page.getByRole('dialog')).toBeVisible();
      await page.keyboard.type(typed);
      const first = page.locator('.palette [role="option"]').first();
      await expect(first).toContainText('Badge QA');
      await expect(first).toContainText('DOC-0005');
      await page.keyboard.press('Enter');
      await expect(page).toHaveURL(`${LIB}/features/badge/qa/2026-10-11-badge-qa`);
    }
  });

  test('a part of an ID lists the matching notes, and /_id/<ID> opens one', async ({ page }) => {
    await page.goto(`${LIB}/billing-api/`);
    await page.keyboard.press('Control+k');
    await expect(page.getByRole('dialog')).toBeVisible();
    await page.keyboard.type('DOC-000');
    await expect(page.locator('.palette [role="option"]').first()).toContainText('DOC-0001');
    expect(await page.locator('.palette [role="option"]').count()).toBeGreaterThan(3);
    await page.keyboard.press('Escape');
    await page.goto('/_id/doc-5');
    await expect(page).toHaveURL(`${LIB}/features/badge/qa/2026-10-11-badge-qa`);
    expect((await page.request.get('/_id/DOC-9999', { maxRedirects: 0 })).status()).toBe(404);
  });
});
