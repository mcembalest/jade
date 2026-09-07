import { test, expect, editor, fileIs } from './fixtures';
import { writeFile, unlink } from 'node:fs/promises';
import { join } from 'node:path';

test('last file, cursor, scroll and sidebar survive restart on a new port', async ({ page, app, workspace }) => {
  await writeFile(join(workspace, 'long.txt'), Array.from({length:200}, (_, i) => `Line ${i + 1}`).join('\n'));
  await page.goto(app.url + '/?file=long.txt');
  await expect(page.locator('body')).toHaveAttribute('data-drafts-ready', 'true');
  await editor(page).press('Control+End');
  // Use an explicit mouse selection, supported identically in both engines.
  await page.locator('.cm-line').last().click();
  await page.locator('#files-toggle').click();
  await page.locator('#pin-files').click();
  await page.locator('.tree summary').filter({hasText:'inner'}).click();
  await expect.poll(async () => (await (await page.request.get(app.url + '/session')).json()).positions?.['long.txt']?.head || 0).toBeGreaterThan(0);
  const state = await (await page.request.get(app.url + '/session')).json();
  await page.close();
  const newURL = await app.restart();
  const reopened = await page.context().newPage();
  await reopened.goto(newURL);
  await fileIs(reopened, 'long.txt');
  await expect(reopened.locator('#files-toggle')).toHaveAttribute('aria-expanded', 'true');
  await expect(reopened.locator('#pin-files')).toHaveAttribute('aria-pressed', 'true');
  await expect(reopened.locator('.tree details').filter({has:reopened.locator('summary').filter({hasText:'inner'})})).toHaveAttribute('open', '');
  const restored = JSON.parse(await reopened.locator('#session-state').textContent() || '{}');
  expect(restored.positions['long.txt'].head).toBe(state.positions['long.txt'].head);
  await expect.poll(() => reopened.locator('.cm-scroller').evaluate(node => node.scrollTop)).toBeGreaterThan(0);
});

test('explicit selection wins and missing remembered files fall back clearly', async ({ page, appURL, workspace }) => {
  await page.goto(appURL + '/?file=notes.txt');
  await expect.poll(async () => (await (await page.request.get(appURL + '/session')).json()).file).toBe('notes.txt');
  await page.goto(appURL + '/?file=code.py');
  await fileIs(page, 'code.py');
  await expect.poll(async () => (await (await page.request.get(appURL + '/session')).json()).file).toBe('code.py');
  await unlink(join(workspace, 'code.py'));
  await page.goto(appURL);
  await fileIs(page, 'README.md');
  await expect(page.locator('#session-notice')).toContainText('previous file is unavailable');
});

test('session preference failure does not block writing or normal saved navigation', async ({ page, appURL }) => {
  await page.route('**/session?*', route => route.fulfill({status:503, body:'Preference unavailable'}));
  await page.goto(appURL + '/?file=notes.txt');
  await expect(page.locator('#session-save-notice')).toBeVisible();
  await editor(page).fill('File saving still works');
  await page.locator('#files-toggle').click();
  await page.locator('a.file-link[data-file="code.py"]').click();
  await fileIs(page, 'code.py');
});

test('slow session writes coalesce intermediate selections before navigation', async ({ page, appURL }) => {
  let release!: () => void;
  const gate = new Promise<void>(resolve => { release = resolve; });
  let requests = 0;
  const snapshots: {file: string; filesPinned: boolean}[] = [];
  await page.route('**/session?*', async route => {
    requests++;
    snapshots.push(route.request().postDataJSON());
    if (requests === 1) await gate;
    await route.continue();
  });
  await page.goto(appURL + '/?file=notes.txt');
  await expect.poll(() => requests).toBe(1);
  await page.locator('#files-toggle').click();
  for (let i = 0; i < 3; i++) {
    await page.locator('#pin-files').click();
    // Each interaction reaches the debounce while the first request is held.
    await page.waitForTimeout(400);
  }
  expect(requests).toBe(1);
  await page.locator('a.file-link[data-file="code.py"]').click();
  release();
  await fileIs(page, 'code.py');
  expect(snapshots.filter(snapshot => snapshot.file === 'notes.txt')).toHaveLength(2);
  expect(snapshots[1].filesPinned).toBe(true);
});

test('a failed slow preference write releases navigation without draining retries', async ({ page, appURL }) => {
  let release!: () => void;
  const gate = new Promise<void>(resolve => { release = resolve; });
  let requests = 0;
  await page.route('**/session?*', async route => {
    requests++;
    if (requests === 1) await gate;
    await route.fulfill({status:503, body:'Preference unavailable'});
  });
  await page.goto(appURL + '/?file=notes.txt');
  await expect.poll(() => requests).toBe(1);
  await page.locator('#files-toggle').click();
  for (let i = 0; i < 3; i++) {
    await page.locator('#pin-files').click();
    await page.waitForTimeout(400);
  }
  await page.locator('a.file-link[data-file="code.py"]').click();
  release();
  await fileIs(page, 'code.py');
  expect(requests).toBe(1);
  await expect(page.locator('#session-save-notice')).toBeVisible();
});
