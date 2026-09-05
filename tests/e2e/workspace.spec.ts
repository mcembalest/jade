import { test, expect, editor, documentText, fileIs, saved } from './fixtures';
import { readFile, writeFile, unlink } from 'node:fs/promises';
import { join } from 'node:path';

const note = 'Original note\n';

test.beforeEach(async ({ page, appURL }) => { await page.goto(appURL + '/?file=notes.txt'); });

test('autosave reaches disk and survives reload', async ({ page, workspace }) => {
  await editor(page).fill('Autosaved 😀\n');
  await expect.poll(() => readFile(join(workspace, 'notes.txt'), 'utf8')).toBe('Autosaved 😀\n');
  await page.reload();
  expect(await documentText(page)).toBe('Autosaved 😀\n');
});

test('immediate subproject navigation saves first', async ({ page, workspace }) => {
  await editor(page).fill('Before switching\n');
  await page.locator('a[href="/?jade=inner&file=jade.md"]').click();
  await expect(page).toHaveURL(/jade=inner/);
  expect(await readFile(join(workspace, 'notes.txt'), 'utf8')).toBe('Before switching\n');
  await page.getByRole('link', { name: 'JADE', exact: true }).click();
  await expect(page.locator('body')).toHaveAttribute('data-jade', '.');
});

test('undo history stays with its file', async ({ page }) => {
  await editor(page).fill('Changed note\n');
  await page.getByRole('link', { name: 'code.py', exact: true }).click();
  await fileIs(page, 'code.py');
  await editor(page).press('ControlOrMeta+z');
  expect(await documentText(page)).toBe('print("hello")\n');
  await page.getByRole('link', { name: 'notes.txt', exact: true }).click();
  await fileIs(page, 'notes.txt');
  await editor(page).press('ControlOrMeta+z');
  expect(await documentText(page)).toBe(note);
});

test('failed save blocks navigation and can be retried', async ({ page, workspace }) => {
  await page.route('**/save', route => route.fulfill({ status: 503, body: 'Temporary write failure' }));
  await editor(page).fill('Keep this edit\n');
  await page.locator('a[href="/?jade=inner&file=jade.md"]').click();
  await expect(page.locator('#save-status')).toContainText('Not saved');
  await fileIs(page, 'notes.txt');
  expect(await documentText(page)).toBe('Keep this edit\n');
  expect(await readFile(join(workspace, 'notes.txt'), 'utf8')).toBe(note);
  await page.unroute('**/save');
  await editor(page).press('ControlOrMeta+s');
  await saved(page);
  expect(await readFile(join(workspace, 'notes.txt'), 'utf8')).toBe('Keep this edit\n');
});

test('external edit conflict preserves both versions and offers explicit recovery', async ({ page, workspace }, testInfo) => {
  let release!: () => void, started!: () => void;
  const held = new Promise<void>(resolve => { release = resolve; });
  const reached = new Promise<void>(resolve => { started = resolve; });
  await page.route('**/save', async route => { started(); await held; await route.continue(); });
  await editor(page).fill('Local version\n');
  await editor(page).press('ControlOrMeta+s');
  await reached;
  await writeFile(join(workspace, 'notes.txt'), 'Agent version\n');
  release();
  await expect(page.locator('#save-status')).toContainText('File changed');
  expect(await readFile(join(workspace, 'notes.txt'), 'utf8')).toBe('Agent version\n');
  expect(await documentText(page)).toBe('Local version\n');
  const downloading = page.waitForEvent('download');
  await page.getByRole('button', { name: 'Download my edits', exact: true }).click();
  const download = await downloading;
  const copy = testInfo.outputPath('local-version.txt');
  await download.saveAs(copy);
  expect(await readFile(copy, 'utf8')).toBe('Local version\n');
  await page.getByRole('button', { name: 'Reload from disk', exact: true }).click();
  expect(await documentText(page)).toBe('Local version\n');
  await page.getByRole('button', { name: 'Discard edits and reload', exact: true }).click();
  await expect(page.locator('#save-status')).toHaveText('Reloaded from disk');
  expect(await documentText(page)).toBe('Agent version\n');
});

test('unmodified files pick up external edits', async ({ page, workspace }) => {
  await writeFile(join(workspace, 'notes.txt'), 'Changed externally\n');
  await expect.poll(() => documentText(page)).toBe('Changed externally\n');
});

test('deleting an edited file does not recreate it', async ({ page, workspace }) => {
  let release!: () => void, started!: () => void;
  const held = new Promise<void>(resolve => { release = resolve; });
  const reached = new Promise<void>(resolve => { started = resolve; });
  await page.route('**/save', async route => { started(); await held; await route.continue(); });
  await editor(page).fill('Keep after deletion\n');
  await editor(page).press('ControlOrMeta+s');
  await reached;
  await unlink(join(workspace, 'notes.txt'));
  release();
  await expect(page.locator('#save-status')).toContainText('File changed');
  await expect(readFile(join(workspace, 'notes.txt'))).rejects.toThrow();
  expect(await documentText(page)).toBe('Keep after deletion\n');
});

for (const [name, ending] of [['LF', '\n'], ['CRLF', '\r\n']]) {
  test(`saving preserves ${name} line endings`, async ({ page, appURL, workspace }) => {
    await writeFile(join(workspace, 'endings.txt'), 'first' + ending + 'second' + ending);
    await page.goto(appURL + '/?file=endings.txt');
    await editor(page).fill('new first\nsecond\n');
    await editor(page).press('ControlOrMeta+s');
    await saved(page);
    expect(await readFile(join(workspace, 'endings.txt'), 'utf8')).toBe('new first' + ending + 'second' + ending);
  });
}

test('edits during a save are included before navigation completes', async ({ page, workspace }) => {
  let release!: () => void, started!: () => void;
  const held = new Promise<void>(resolve => { release = resolve; });
  const reached = new Promise<void>(resolve => { started = resolve; });
  let first = true;
  await page.route('**/save', async route => {
    if (first) { first = false; started(); await held; }
    await route.continue();
  });
  await editor(page).fill('First snapshot\n');
  await editor(page).press('ControlOrMeta+s');
  await reached;
  await editor(page).fill('Newer snapshot\n');
  release();
  await page.locator('a[href="/?jade=inner&file=jade.md"]').click();
  await expect(page).toHaveURL(/jade=inner/);
  expect(await readFile(join(workspace, 'notes.txt'), 'utf8')).toBe('Newer snapshot\n');
  await saved(page);
});

test('file creation, cancellation, and refresh use local files', async ({ page, workspace }) => {
  await page.getByRole('button', { name: 'New file', exact: true }).click();
  await page.getByRole('button', { name: 'Cancel', exact: true }).click();
  await expect(page.locator('#new-file-dialog')).not.toBeVisible();
  await page.getByRole('button', { name: 'New file', exact: true }).click();
  await page.getByRole('textbox', { name: 'New file path' }).fill('notes/with spaces.md');
  await page.getByRole('button', { name: 'Create file', exact: true }).click();
  await fileIs(page, 'notes/with spaces.md');
  expect(await readFile(join(workspace, 'notes/with spaces.md'), 'utf8')).toBe('');
  await writeFile(join(workspace, 'agent-added.txt'), 'new file');
  await page.getByRole('button', { name: 'Refresh files', exact: true }).click();
  await expect(page.getByRole('link', { name: 'agent-added.txt', exact: true })).toBeVisible();
});

test('error controls fit the minimum supported window width', async ({ page }) => {
  await page.setViewportSize({ width: 760, height: 520 });
  await page.route('**/save', route => route.fulfill({ status: 503, body: 'Cannot save here. Check the file and folder permissions, then try saving again.' }));
  await editor(page).fill('Unsaved');
  await editor(page).press('ControlOrMeta+s');
  await expect(page.locator('#save-status')).toContainText('Not saved');
  for (const selector of ['#editor', '#reload-file', '#save-copy', '#save-status']) {
    const box = await page.locator(selector).boundingBox();
    expect(box).not.toBeNull();
    expect(box!.x).toBeGreaterThanOrEqual(0);
    expect(box!.x + box!.width).toBeLessThanOrEqual(760);
    expect(box!.y + box!.height).toBeLessThanOrEqual(520);
  }
});

test('local SVG plots render inside the sandboxed Markdown preview', async ({ page, appURL, workspace }) => {
  await writeFile(join(workspace, 'plot.svg'), '<svg xmlns="http://www.w3.org/2000/svg" width="200" height="100"><rect width="200" height="100" fill="#50dfbd"/></svg>');
  await writeFile(join(workspace, 'jade.md'), '# Plot workspace\n\n![Measured plot](plot.svg)\n');
  await page.goto(appURL);
  const plot = page.frameLocator('#view-frame').getByRole('img', { name: 'Measured plot' });
  await expect(plot).toBeVisible();
  await expect.poll(() => plot.evaluate(image => (image as HTMLImageElement).naturalWidth)).toBe(200);
  await expect(page.locator('#view-frame')).toHaveAttribute('sandbox', 'allow-top-navigation-by-user-activation');
});
