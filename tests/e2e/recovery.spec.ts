import { test, expect, editor, documentText, saved } from './fixtures';
import type { APIRequestContext, Page } from '@playwright/test';
import { readFile, writeFile, unlink } from 'node:fs/promises';
import { join } from 'node:path';

type Draft = { id: string; content: string; revision: string };
const drafts = async (request: APIRequestContext, url: string): Promise<Draft[]> => {
  const response = await request.get(url + '/drafts?jade=.&file=notes.txt');
  expect(response.ok()).toBeTruthy();
  return (await response.json()).drafts;
};
async function leaveDraft(page: Page, url: string, content: string) {
  await page.route('**/save', route => route.fulfill({ status: 503, body: 'Simulated disk save failure' }));
  await page.goto(url + '/?file=notes.txt');
  await editor(page).fill(content);
  await expect.poll(async () => (await drafts(page.request, url)).some(d => d.content.replace(/\r\n/g, '\n') === content)).toBe(true);
}

test('draft survives engine crash and a fresh browser, then saves explicitly', async ({ page, browser, app, workspace }) => {
  const content = 'Recovered after restart 😀\n';
  await leaveDraft(page, app.url, content);
  await page.close();
  await app.restart('SIGKILL');
  const fresh = await browser.newPage();
  try {
    await fresh.goto(app.url + '/?file=notes.txt');
    await fresh.getByRole('button', { name: 'Recover draft', exact: true }).click();
    expect(await documentText(fresh)).toBe(content);
    // A recovered draft stays pending until the user chooses to save it.
    await fresh.waitForTimeout(1100);
    expect(await readFile(join(workspace, 'notes.txt'), 'utf8')).toBe('Original note\n');
    await editor(fresh).press('ControlOrMeta+s');
    await saved(fresh);
    expect(await readFile(join(workspace, 'notes.txt'), 'utf8')).toBe(content);
    await expect.poll(async () => (await drafts(fresh.request, app.url)).length).toBe(0);
    await fresh.reload();
    await expect(fresh.getByRole('button', { name: 'Recover draft', exact: true })).not.toBeVisible();
    expect(await documentText(fresh)).toBe(content);
  } finally { await fresh.close(); }
});

test('recovered draft cannot overwrite an external edit after restart', async ({ page, browser, app, workspace }, testInfo) => {
  const content = 'My unsaved version\n';
  await leaveDraft(page, app.url, content);
  await page.close();
  await writeFile(join(workspace, 'notes.txt'), 'Agent version on disk\n');
  await app.restart('SIGKILL');
  const fresh = await browser.newPage();
  try {
    await fresh.goto(app.url + '/?file=notes.txt');
    const downloading = fresh.waitForEvent('download');
    await fresh.getByRole('button', { name: 'Download draft', exact: true }).click();
    const copy = testInfo.outputPath('recovered.txt');
    await (await downloading).saveAs(copy);
    expect(await readFile(copy, 'utf8')).toBe(content);
    await fresh.getByRole('button', { name: 'Recover draft', exact: true }).click();
    expect(await documentText(fresh)).toBe(content);
    await editor(fresh).press('ControlOrMeta+s');
    await expect(fresh.locator('#save-status')).toContainText('changed');
    expect(await readFile(join(workspace, 'notes.txt'), 'utf8')).toBe('Agent version on disk\n');
    expect(await documentText(fresh)).toBe(content);
    await fresh.getByRole('button', { name: 'Reload from disk', exact: true }).click();
    await fresh.getByRole('button', { name: 'Discard edits and reload', exact: true }).click();
    await expect(fresh.locator('#save-status')).toHaveText('Reloaded from disk');
    expect(await documentText(fresh)).toBe('Agent version on disk\n');
    await expect.poll(async () => (await drafts(fresh.request, app.url)).length).toBe(0);
  } finally { await fresh.close(); }
});

test('one tab cannot discard another tab’s draft', async ({ page, context, app }) => {
  await leaveDraft(page, app.url, 'First tab draft\n');
  const other = await context.newPage();
  await leaveDraft(other, app.url, 'Second tab draft\n');
  await expect.poll(async () => (await drafts(page.request, app.url)).length).toBe(2);
  await other.close();
  await page.reload();
  const before = await drafts(page.request, app.url);
  await page.locator('#draft-select').selectOption(before.find(d => d.content === 'First tab draft\n')!.id);
  await page.getByRole('button', { name: 'Discard draft', exact: true }).click();
  await expect.poll(async () => (await drafts(page.request, app.url)).map(d => d.content)).toEqual(['Second tab draft\n']);
  await page.getByRole('button', { name: 'Recover draft', exact: true }).click();
  expect(await documentText(page)).toBe('Second tab draft\n');
});

test('draft recovery preserves original CRLF line endings', async ({ page, browser, app, workspace }) => {
  await writeFile(join(workspace, 'notes.txt'), 'Original\r\nsecond\r\n');
  await leaveDraft(page, app.url, 'Edited\nsecond\n');
  await page.close();
  await app.restart('SIGKILL');
  const fresh = await browser.newPage();
  try {
    await fresh.goto(app.url + '/?file=notes.txt');
    await fresh.getByRole('button', { name: 'Recover draft', exact: true }).click();
    await editor(fresh).press('ControlOrMeta+s');
    await saved(fresh);
    expect(await readFile(join(workspace, 'notes.txt'), 'utf8')).toBe('Edited\r\nsecond\r\n');
  } finally { await fresh.close(); }
});

test('failed draft lookup never offers a different file’s drafts', async ({ page, app }) => {
  await leaveDraft(page, app.url, 'Draft belonging to notes\n');
  await page.reload();
  await expect(page.getByRole('button', { name: 'Recover draft', exact: true })).toBeVisible();
  await page.route('**/drafts?**', route => {
    if (route.request().method() === 'GET' && new URL(route.request().url()).searchParams.get('file') === 'code.py') {
      return route.fulfill({ status: 503, body: 'Cannot load drafts' });
    }
    return route.continue();
  });
  await page.getByRole('link', { name: 'code.py', exact: true }).click();
  await expect(page.locator('#draft-status')).toContainText('Cannot load');
  await expect(page.getByRole('button', { name: 'Recover draft', exact: true })).not.toBeVisible();
  expect(await documentText(page)).toBe('print("hello")\n');
});

test('typing during draft cleanup is saved before reporting success', async ({ page, app, workspace }) => {
  await page.goto(app.url + '/?file=notes.txt');
  let release!: () => void, started!: () => void;
  const held = new Promise<void>(resolve => { release = resolve; });
  const reached = new Promise<void>(resolve => { started = resolve; });
  let first = true;
  await page.route('**/drafts?**', async route => {
    if (route.request().method() === 'DELETE' && first) {
      first = false; started(); await held;
    }
    await route.continue();
  });
  await editor(page).fill('First saved version\n');
  await editor(page).press('ControlOrMeta+s');
  await reached;
  await editor(page).fill('Typed during cleanup\n');
  release();
  await saved(page);
  expect(await readFile(join(workspace, 'notes.txt'), 'utf8')).toBe('Typed during cleanup\n');
  await expect.poll(async () => (await drafts(page.request, app.url)).length).toBe(0);
});

test('undoing all edits clears the redundant recovery draft', async ({ page, app }) => {
  await leaveDraft(page, app.url, 'Will undo\n');
  await page.unroute('**/save');
  await editor(page).press('ControlOrMeta+z');
  expect(await documentText(page)).toBe('Original note\n');
  await editor(page).press('ControlOrMeta+s');
  await saved(page);
  await expect.poll(async () => (await drafts(page.request, app.url)).length).toBe(0);
});

test('a failed recovery backup is visible and does not block an ordinary save', async ({ page, app, workspace }) => {
  await page.route('**/drafts', route => route.fulfill({ status: 503, body: 'Recovery storage unavailable' }));
  await page.goto(app.url + '/?file=notes.txt');
  await editor(page).fill('Saved despite backup failure\n');
  await editor(page).press('ControlOrMeta+s');
  await saved(page);
  expect(await readFile(join(workspace, 'notes.txt'), 'utf8')).toBe('Saved despite backup failure\n');
  await expect(page.locator('#draft-status')).toContainText('Recovery backup unavailable');
});

test('a draft already saved to disk can be recovered and cleared without a false conflict', async ({ page, app, workspace }) => {
  await page.goto(app.url + '/?file=notes.txt');
  await page.route('**/drafts?**', route => route.request().method() === 'DELETE'
    ? route.fulfill({ status: 503, body: 'Cleanup interrupted' }) : route.continue());
  await editor(page).fill('Saved before cleanup\n');
  await editor(page).press('ControlOrMeta+s');
  await saved(page);
  expect(await readFile(join(workspace, 'notes.txt'), 'utf8')).toBe('Saved before cleanup\n');
  expect((await drafts(page.request, app.url)).length).toBe(1);
  await page.unroute('**/drafts?**');
  await page.reload();
  await page.getByRole('button', { name: 'Recover draft', exact: true }).click();
  await editor(page).press('ControlOrMeta+s');
  await saved(page);
  await expect.poll(async () => (await drafts(page.request, app.url)).length).toBe(0);
});

test('a deleted file’s draft remains downloadable after restart without recreating the file', async ({ page, browser, app, workspace }, testInfo) => {
  const content = 'Text from deleted file\n';
  await leaveDraft(page, app.url, content);
  await page.close();
  await unlink(join(workspace, 'notes.txt'));
  await app.restart('SIGKILL');
  const fresh = await browser.newPage();
  try {
    await fresh.goto(app.url + '/?file=notes.txt');
    const downloading = fresh.waitForEvent('download');
    await fresh.getByRole('button', { name: 'Download draft', exact: true }).click();
    const copy = testInfo.outputPath('deleted-file.txt');
    await (await downloading).saveAs(copy);
    expect(await readFile(copy, 'utf8')).toBe(content);
    await fresh.getByRole('button', { name: 'Recover draft', exact: true }).click();
    expect(await documentText(fresh)).toBe(content);
    await editor(fresh).press('ControlOrMeta+s');
    await expect(readFile(join(workspace, 'notes.txt'))).rejects.toThrow();
  } finally { await fresh.close(); }
});

test('recovery actions fit the minimum supported window', async ({ page, app }, testInfo) => {
  await leaveDraft(page, app.url, 'Recovery panel example\n');
  await page.setViewportSize({ width: 760, height: 520 });
  await page.reload();
  await expect(page.getByRole('button', { name: 'Recover draft', exact: true })).toBeVisible();
  for (const selector of ['#draft-select', '#recover-draft', '#download-draft', '#discard-draft', '#editor']) {
    const box = await page.locator(selector).boundingBox();
    expect(box).not.toBeNull();
    expect(box!.x).toBeGreaterThanOrEqual(0);
    expect(box!.y).toBeGreaterThanOrEqual(0);
    expect(box!.x + box!.width).toBeLessThanOrEqual(760);
    expect(box!.y + box!.height).toBeLessThanOrEqual(520);
  }
  await page.screenshot({ path: testInfo.outputPath('recovery-panel.png') });
});
