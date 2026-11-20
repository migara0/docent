import { test as base, expect, type Page } from '@playwright/test';

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
const popup = (page: Page) => page.getByRole('dialog', { name: 'Browse files' });
const selected = (page: Page) => page.locator('.navdialog .col [aria-selected="true"] .t');
const press = (page: Page, key: string) => page.getByRole('textbox', { name: /Find a page/ }).press(key);

async function openIt(page: Page, url = `${LIB}/guide/alpha`) {
  await page.goto(url);
  await page.keyboard.press('b');
  await expect(popup(page)).toBeVisible();
  await expect(page.getByRole('textbox', { name: /Find a page|Find a page in/ })).toBeFocused();
}

test.describe('Browse popup', () => {
  test('opens from the toolbar, the b key and the command palette, at the page you are reading, and closes three ways', async ({ page }) => {
    await page.goto(`${LIB}/guide/alpha`);
    await page.getByRole('button', { name: 'Browse files and folders' }).click();
    await expect(popup(page)).toBeVisible();
    await expect(page.locator('.nav__crumbs .crumb')).toHaveText(['Libraries', 'E2E notes', 'guide', 'Alpha guide']);
    await expect(page.locator('.preview h3')).toHaveText('Alpha guide');
    await page.getByRole('button', { name: 'Close' }).click();
    await expect(popup(page)).toBeHidden();

    await page.keyboard.press('b');
    await expect(popup(page)).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(popup(page)).toBeHidden();

    await page.keyboard.press('Control+k');
    await expect(page.getByRole('dialog', { name: 'Search' })).toBeVisible();
    await page.keyboard.type('browse');
    await page.keyboard.press('Enter');
    await expect(popup(page)).toBeVisible();
    await page.mouse.click(5, 5); // the backdrop
    await expect(popup(page)).toBeHidden();
    await expect(page).toHaveURL(`${LIB}/guide/alpha`);
  });

  test('the arrow keys move through the columns, and Enter opens the page', async ({ page }) => {
    await openIt(page);
    await expect(selected(page)).toHaveText(['E2E notes', 'guide', 'Alpha guide']);
    await press(page, 'ArrowDown');
    await expect(selected(page)).toHaveText(['E2E notes', 'guide', 'Beta guide']);
    await expect(page.locator('.preview h3')).toHaveText('Beta guide');
    await press(page, 'ArrowLeft'); // back to the folder
    await expect(selected(page)).toHaveText(['E2E notes', 'guide']);
    await expect(page.locator('.preview h3')).toHaveText('guide');
    await expect(page.locator('.preview')).toContainText('2 pages in this folder');
    await press(page, 'ArrowDown'); // the next folder in that column
    await expect(selected(page)).toHaveText(['E2E notes', 'notes']);
    await press(page, 'ArrowRight');
    await expect(selected(page)).toHaveText(['E2E notes', 'notes', 'Café menu']);
    await press(page, 'Enter');
    await expect(popup(page)).toBeHidden();
    await expect(page).toHaveURL(/\/e2e-notes\/notes\/caf/);
    await expect(page.locator('.pagehead h1')).toContainText('Café');
  });

  test('a page shows a preview of its beginning, and Home and End jump to the ends of a column', async ({ page }) => {
    await openIt(page);
    await expect(page.locator('.preview .excerpt')).toContainText('This page has bold');
    await press(page, 'End');
    await expect(selected(page).last()).toHaveText('Beta guide');
    await press(page, 'Home');
    await expect(selected(page).last()).toHaveText('Alpha guide');
  });

  test('clicking selects and opens the next column, double-clicking opens, and the path bar goes back', async ({ page }) => {
    await openIt(page);
    await page.locator('.navdialog .col').first().getByRole('option', { name: /E2E notes/ }).click();
    await expect(page.locator('.nav__crumbs .crumb')).toHaveText(['Libraries', 'E2E notes']);
    await page.locator('.navdialog .col').nth(1).getByRole('option', { name: /notes/ }).first().click();
    await expect(page.locator('.navdialog .col').nth(2).getByRole('option', { name: /Café menu/ })).toBeVisible();
    await page.getByRole('button', { name: 'Libraries', exact: true }).click(); // the first crumb
    await expect(page.locator('.navdialog .col')).toHaveCount(1);
    await page.locator('.navdialog .col').first().getByRole('option', { name: /E2E notes/ }).dblclick();
    await expect(popup(page)).toBeHidden();
    await expect(page).toHaveURL(`${LIB}/`);
  });

  test('Shift+Enter opens a folder or library itself', async ({ page }) => {
    await openIt(page);
    await press(page, 'ArrowLeft');
    await expect(selected(page)).toHaveText(['E2E notes', 'guide']);
    await press(page, 'Shift+Enter');
    await expect(page).toHaveURL(`${LIB}/guide/`);
    await expect(page.locator('.folder h1, h1').first()).toContainText('guide');
  });

  test('typing finds pages in the library, Enter opens one, and Escape clears the search before it closes the popup', async ({ page }) => {
    await openIt(page);
    await page.keyboard.type('alpha');
    await expect(page.locator('.nav__count')).toHaveText('1 page');
    await expect(page.locator('.results .row')).toHaveCount(1);
    await expect(page.locator('.results mark')).toHaveText('alpha', { ignoreCase: true } as never);
    await page.keyboard.press('Escape');
    await expect(popup(page)).toBeVisible(); // only the search was cleared
    await expect(page.getByRole('textbox', { name: /Find a page/ })).toHaveValue('');
    await page.keyboard.type('long');
    await page.keyboard.press('Enter');
    await expect(popup(page)).toBeHidden();
    await expect(page).toHaveURL(`${LIB}/long`);
    await expect.poll(() => page.evaluate(() => document.activeElement?.tagName)).toBe('BODY'); // the closed popup has given the focus back
    await page.keyboard.press('b');
    await expect(page.getByRole('textbox', { name: /Find a page/ })).toBeFocused();
    await page.keyboard.type('zzzz');
    await expect(page.locator('.navdialog .empty')).toContainText('No page matches');
  });

  test('from the home page it starts with the libraries and loads one when chosen', async ({ page }) => {
    await page.goto('/');
    await page.keyboard.press('b');
    await expect(popup(page)).toBeVisible();
    await expect(page.locator('.navdialog .col')).toHaveCount(1);
    await expect(page.locator('.navdialog .col').first().getByRole('option', { name: /doc-notes/ })).toBeVisible();
    await page.getByRole('textbox', { name: /Find a page/ }).press('ArrowDown'); // selects the first library
    await page.getByRole('textbox', { name: /Find a page/ }).press('ArrowRight');
    await expect(page.locator('.navdialog .col')).toHaveCount(2); // the libraries, and the first item inside the chosen one (a page here)
    await expect(page.locator('.preview h3')).not.toBeEmpty();
  });

  test('on a phone it shows one column at a time, and Back goes to the list before it', async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 800 });
    await page.goto(`${LIB}/guide/alpha`);
    await page.getByRole('button', { name: 'Browse files and folders' }).click();
    await expect(popup(page)).toBeVisible();
    const shown = page.locator('.navdialog .col.is-shown');
    await expect(shown).toHaveCount(1);
    await expect(shown.getByRole('option', { name: /Alpha guide/ })).toBeVisible();
    await page.getByRole('button', { name: 'Back' }).click(); // the list that holds the guide folder
    await expect(shown.getByRole('option', { name: /^guide/ })).toBeVisible();
    await shown.getByRole('option', { name: /^guide/ }).click(); // choosing it shows what is inside again
    await expect(shown.getByRole('option', { name: /Beta guide/ })).toBeVisible();
    await page.getByRole('button', { name: 'Open folder' }).click();
    await expect(page).toHaveURL(`${LIB}/guide/`);
  });
});
