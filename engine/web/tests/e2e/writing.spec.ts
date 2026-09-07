import { test, expect, editor, documentText, saved, revealFiles, revealPreview } from './fixtures';
import { readFile, writeFile } from 'node:fs/promises';
import { join } from 'node:path';

test('link wraps the selection, saves normally, and is one independent undo step', async ({ page, appURL, workspace }) => {
  await page.goto(appURL + '/?file=README.md');
  await expect(page.locator('body')).toHaveAttribute('data-drafts-ready', 'true');
  await editor(page).fill('Read café [notes] 😀');
  await editor(page).press('ControlOrMeta+a');
  await page.getByRole('button', { name: 'Link', exact: true }).click();
  await expect(page.getByLabel('Link text', { exact: true })).toHaveValue('Read café [notes] 😀');
  await expect(page.getByLabel('Destination', { exact: true })).toBeFocused();
  await page.getByLabel('Destination', { exact: true }).fill('https://example.com/my notes(v2)');
  await page.getByRole('button', { name: 'Insert link', exact: true }).click();
  const linked = '[Read café \\[notes\\] 😀](<https://example.com/my%20notes(v2)>)';
  expect(await documentText(page)).toBe(linked);
  await expect(editor(page)).toBeFocused();
  await editor(page).press('ControlOrMeta+z');
  expect(await documentText(page)).toBe('Read café [notes] 😀');
  await editor(page).press('ControlOrMeta+Shift+z');
  expect(await documentText(page)).toBe(linked);
  await saved(page);
  expect(await readFile(join(workspace, 'README.md'), 'utf8')).toBe(linked);
  await revealPreview(page);
  const link = page.frameLocator('#view-frame').getByRole('link', { name: 'Read café [notes] 😀', exact: true });
  await expect(link).toHaveAttribute('href', 'https://example.com/my%20notes(v2)');
});

test('keyboard entry supports cancel without changing selection, and insertion at the cursor', async ({ page, appURL }) => {
  await page.goto(appURL + '/?file=README.md');
  await expect(page.locator('body')).toHaveAttribute('data-drafts-ready', 'true');
  await editor(page).fill('before\nselected\nafter');
  await editor(page).press('ControlOrMeta+Home');
  await editor(page).press('ArrowDown');
  await editor(page).press('Home');
  await editor(page).press('Shift+End');
  await editor(page).press('ControlOrMeta+k');
  await expect(page.getByLabel('Link text', { exact: true })).toHaveValue('selected');
  await page.getByLabel('Destination', { exact: true }).press('Escape');
  await expect(editor(page)).toBeFocused();
  expect(await documentText(page)).toBe('before\nselected\nafter');
  await page.getByRole('button', { name: 'Link', exact: true }).click();
  await expect(page.getByLabel('Link text', { exact: true })).toHaveValue('selected');
  await page.getByRole('button', { name: 'Cancel', exact: true }).click();
  await editor(page).press('ControlOrMeta+End');
  await editor(page).press('ControlOrMeta+k');
  await expect(page.getByLabel('Link text', { exact: true })).toBeFocused();
  await page.getByLabel('Link text', { exact: true }).fill('Local notes');
  await page.getByLabel('Destination', { exact: true }).fill('notes.txt');
  await page.getByLabel('Destination', { exact: true }).press('Enter');
  expect(await documentText(page)).toBe('before\nselected\nafter[Local notes](<notes.txt>)');
});

test('source files have no link action and an unsafe destination is recoverable', async ({ page, appURL }) => {
  await page.goto(appURL + '/?file=README.md');
  await expect(page.locator('body')).toHaveAttribute('data-drafts-ready', 'true');
  const initial = await documentText(page);
  await page.getByRole('button', { name: 'Link', exact: true }).click();
  await page.getByLabel('Link text', { exact: true }).fill('Label');
  await page.getByLabel('Destination', { exact: true }).fill('javascript:alert(1)');
  await page.getByRole('button', { name: 'Insert link', exact: true }).click();
  await expect(page.locator('#link-error')).toContainText('relative file path');
  expect(await documentText(page)).toBe(initial);
  await page.getByRole('button', { name: 'Cancel', exact: true }).click();
  await revealFiles(page);
  await page.locator('.file-link[data-file="notes.txt"]').click();
  await expect(page.locator('#insert-link')).toBeHidden();
  await editor(page).press('ControlOrMeta+k');
  await expect(page.getByRole('dialog', { name: 'Insert Markdown link' })).not.toBeVisible();
  await revealFiles(page);
  await page.locator('.file-link[data-file="README.md"]').click();
  await expect(page.locator('#insert-link')).toBeVisible();
});

test('link command respects recovery loading and preserves CRLF files', async ({ page, appURL, workspace }) => {
  await writeFile(join(workspace, 'README.md'), 'first\r\nsecond\r\n');
  let release!: () => void;
  const gate = new Promise<void>(resolve => { release = resolve; });
  await page.route('**/drafts?**', async route => { await gate; await route.continue(); });
  await page.goto(appURL + '/?file=README.md');
  await expect(page.locator('body')).toHaveAttribute('data-drafts-ready', 'false');
  await page.getByRole('button', { name: 'Link', exact: true }).click();
  await expect(page.locator('#link-dialog')).not.toBeVisible();
  release();
  await expect(page.locator('body')).toHaveAttribute('data-drafts-ready', 'true');
  await editor(page).press('ControlOrMeta+Home');
  await editor(page).press('Shift+End');
  await editor(page).press('ControlOrMeta+k');
  await page.getByLabel('Destination', { exact: true }).fill('notes.txt');
  await page.getByRole('button', { name: 'Insert link', exact: true }).click();
  await saved(page);
  expect(await readFile(join(workspace, 'README.md'), 'utf8')).toBe('[first](<notes.txt>)\r\nsecond\r\n');
});

test('link dialog fits a narrow window and preserves literal ampersands', async ({page, appURL}, info) => {
  await page.setViewportSize({width:390,height:640});
  await page.goto(appURL + '/?file=README.md');
  await expect(page.locator('body')).toHaveAttribute('data-drafts-ready', 'true');
  await editor(page).fill('Research &amp; notes');
  await editor(page).press('ControlOrMeta+a');
  await page.locator('#insert-link').click();
  await page.locator('#link-destination').fill('notes.txt');
  for (const selector of ['#link-dialog', '#link-destination', '#link-form button[type="submit"]']) {
    const box = (await page.locator(selector).boundingBox())!;
    expect(box.x).toBeGreaterThanOrEqual(0); expect(box.x + box.width).toBeLessThanOrEqual(390);
    expect(box.y).toBeGreaterThanOrEqual(0); expect(box.y + box.height).toBeLessThanOrEqual(640);
  }
  await page.screenshot({path:info.outputPath('link-narrow.png')});
  await page.locator('#link-form button[type="submit"]').click();
  await saved(page);
  await revealPreview(page);
  await expect(page.frameLocator('#view-frame').getByRole('link', {name:'Research &amp; notes',exact:true})).toBeVisible();
});
