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

/** Drops files onto the window the way a file manager does. */
async function dropFiles(page: Page, files: { name: string; text: string; type?: string }[], hold = false) {
  await page.evaluate(
    ({ files, hold }) => {
      const dt = new DataTransfer();
      for (const f of files) dt.items.add(new File([f.text], f.name, { type: f.type ?? 'text/markdown' }));
      const fire = (type: string) => window.dispatchEvent(new DragEvent(type, { dataTransfer: dt, bubbles: true, cancelable: true }));
      fire('dragenter');
      fire('dragover');
      if (!hold) fire('drop');
    },
    { files, hold },
  );
}

test.describe('dropping files', () => {
  test('a dropped Markdown file opens as a temporary copy that says so', async ({ page }) => {
    await page.goto(`${LIB}/index`);
    await dropFiles(page, [{ name: 'dropped-one.md', text: '# A dropped page\n\nHello from a copy. See [two](dropped-two.md).\n' }, { name: 'dropped-two.md', text: '# Second dropped\n' }]);
    await expect(page).toHaveURL(/\/dropped\/dropped-one$/);
    await expect(page.locator('.pagehead h1')).toHaveText('A dropped page');
    await expect(page.locator('.callout', { hasText: 'A temporary copy' })).toBeVisible();
    await expect(page.locator('.toast')).toContainText('2 files as temporary copies');
    // the copies link to each other like any pages
    await page.locator('.prose a', { hasText: 'two' }).click();
    await expect(page.locator('.pagehead h1')).toHaveText('Second dropped');
    // it is a library of its own on the home page, labelled as dropped copies
    await page.goto('/');
    await expect(page.locator('.libcard', { hasText: 'Dropped files' })).toContainText('Dropped copies');
    // dropping the same name again replaces the copy
    await page.goto(`${LIB}/index`);
    await dropFiles(page, [{ name: 'dropped-one.md', text: '# Replaced title\n' }]);
    await expect(page.locator('.pagehead h1')).toHaveText('Replaced title');
  });

  test('an overlay shows while files are held over the window, and files that are not Markdown are refused kindly', async ({ page }) => {
    await page.goto(`${LIB}/index`);
    await dropFiles(page, [{ name: 'x.md', text: '# x' }], true);
    await expect(page.locator('.dropzone')).toBeVisible();
    await page.evaluate(() => window.dispatchEvent(new DragEvent('dragleave', { bubbles: true, cancelable: true, dataTransfer: new DataTransfer() })));
    await dropFiles(page, [{ name: 'photo.png', text: 'not markdown', type: 'image/png' }]);
    await expect(page.locator('.toast')).toContainText('Nothing to open');
    await expect(page).toHaveURL(`${LIB}/index`);
  });
});

test.describe('open on your phone', () => {
  test('start sharing shows a QR code for this page, the shared address is read-only, and stop closes it', async ({ page, request }) => {
    await page.goto(`${LIB}/guide/alpha`);
    await page.getByRole('button', { name: 'Open on your phone' }).click();
    const dialog = page.getByRole('dialog', { name: 'Open on your phone' });
    await expect(dialog).toBeVisible();
    await expect(dialog).toContainText('Anyone on the same network');
    const start = dialog.getByRole('button', { name: 'Start sharing' });
    const noNetwork = dialog.getByText('not connected to a local network');
    await expect(start.or(noNetwork)).toBeVisible();
    test.skip(await noNetwork.isVisible(), 'this machine has no local network address');

    await start.click();
    const img = dialog.locator('.qr img');
    await expect(img).toBeVisible();
    await expect.poll(() => img.evaluate((el: HTMLImageElement) => el.complete && el.naturalWidth > 0)).toBe(true);
    const address = (await dialog.locator('.addr code').textContent())!.trim();
    expect(address).toMatch(/^http:\/\/\d+\.\d+\.\d+\.\d+:\d+\/e2e-notes\/guide\/alpha$/);

    // the phone's view: the same page, readable, but nothing can be changed
    const origin = new URL(address).origin;
    expect((await request.get(`${origin}/_api/page?src=e2e-notes&path=guide/alpha.md`)).status()).toBe(200);
    expect((await request.post(`${origin}/_api/open`, { data: { path: '/tmp' } })).status()).toBe(403);
    expect((await request.post(`${origin}/_api/drop`, { data: { files: [{ name: 'a.md', text: 'x' }] } })).status()).toBe(403);

    await dialog.getByRole('button', { name: 'Stop sharing' }).click();
    await expect(dialog.getByRole('button', { name: 'Start sharing' })).toBeVisible();
    await expect.poll(async () => request.get(`${origin}/_api/health`).then(() => 'open', () => 'closed')).toBe('closed');
  });
});
