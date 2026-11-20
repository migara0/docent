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

test.describe('help', () => {
  test('the Help button opens the built-in guide, which has a table of contents and can be searched', async ({ page }) => {
    await page.goto('/e2e-notes/index');
    await page.getByRole('button', { name: 'Help: the Docent guide' }).click();
    await expect(page).toHaveURL('/help/guide');
    await expect(page.locator('.pagehead h1')).toHaveText('Docent guide');
    await expect(page.locator('.toc')).toContainText('Open a page on your phone');
    await expect(page.locator('.prose')).toContainText('Drop a file');
    // the guide is not one of the reader's libraries
    await page.goto('/');
    await expect(page.locator('.libcard', { hasText: 'Docent guide' })).toHaveCount(0);
    await expect(page.getByRole('link', { name: 'Read the Docent guide' })).toBeVisible();
    await page.getByRole('link', { name: 'Read the Docent guide' }).click();
    await expect(page).toHaveURL('/help/guide');
  });

  test('the ? key and the command palette open it too, but typing a ? does not', async ({ page }) => {
    await page.goto('/e2e-notes/guide/alpha');
    await page.keyboard.press('Control+k');
    await expect(page.getByRole('dialog')).toBeVisible();
    await page.keyboard.type('?');
    await page.keyboard.press('Escape');
    await expect(page.getByRole('dialog')).toBeHidden(); // a native dialog closes a moment after Escape
    await expect.poll(() => page.evaluate(() => document.activeElement?.tagName)).toBe('BODY'); // and gives the focus back a moment after that
    await expect(page).toHaveURL('/e2e-notes/guide/alpha');
    await page.keyboard.press('?');
    await expect(page).toHaveURL('/help/guide');
    await page.goto('/e2e-notes/index');
    await page.keyboard.press('Control+k');
    await expect(page.getByRole('dialog')).toBeVisible();
    await page.keyboard.type('Docent guide');
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL('/help/guide');
  });
});
