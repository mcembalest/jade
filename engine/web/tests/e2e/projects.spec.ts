import { test, expect, editor, fileIs } from './fixtures';
import { mkdir, readFile, realpath, writeFile } from 'node:fs/promises';
import { dirname, join } from 'node:path';

test('open an unrelated project after saving; recent projects restore the previous file', async ({page, appURL, workspace}) => {
  const other = join(dirname(workspace), 'other-project');
  await mkdir(other); await writeFile(join(other, 'README.md'), '# Other project');
  await page.goto(appURL + '/?file=notes.txt');
  await editor(page).fill('Saved before changing projects');
  await page.locator('#projects-toggle').click();
  await page.locator('#project-path').fill(other);
  await page.locator('#project-open').click();
  await expect(page.locator('.project')).toHaveText('Other project');
  expect(new URL(page.url()).origin).not.toBe(appURL);
  expect(await readFile(join(workspace, 'notes.txt'), 'utf8')).toBe('Saved before changing projects');
  await page.locator('#projects-toggle').click();
  await page.locator('#recent-projects').getByRole('button', {name: await realpath(workspace), exact: true}).click();
  await fileIs(page, 'notes.txt');
  await expect(editor(page)).toHaveText('Saved before changing projects');
  expect(new URL(page.url()).origin).toBe(appURL);
});

test('failed saving blocks project opening and a missing project leaves the file intact', async ({page, appURL, workspace}) => {
  await page.goto(appURL + '/?file=notes.txt');
  await page.route('**/save', route => route.fulfill({status:503,body:'Disk unavailable'}));
  await editor(page).fill('Keep this edit');
  await page.locator('#projects-toggle').click();
  await page.locator('#project-path').fill(join(workspace, 'inner'));
  await page.locator('#project-open').click();
  await expect(page.locator('#projects-error')).toContainText('Resolve the current file');
  await fileIs(page, 'notes.txt');
  await expect(editor(page)).toHaveText('Keep this edit');
  await page.unroute('**/save');
  await page.locator('#project-path').fill(join(workspace, 'missing-project'));
  await page.locator('#project-open').click();
  await expect(page.locator('#projects-error')).toContainText('no such file');
  await fileIs(page, 'notes.txt');
});

test('cancelling a slow project open keeps the current editor and returns focus', async ({page, appURL, workspace}) => {
  await page.goto(appURL + '/?file=notes.txt');
  let release!: () => void;
  const held = new Promise<void>(resolve => { release = resolve; });
  let reached!: () => void;
  const requested = new Promise<void>(resolve => { reached = resolve; });
  await page.route('**/projects', async route => {
    if (route.request().method() !== 'POST') { await route.continue(); return; }
    reached(); await held;
    await route.fulfill({json:{url:appURL + '/?file=code.py'}}).catch(() => {});
  });
  await page.locator('#projects-toggle').click();
  await page.locator('#project-path').fill(workspace);
  await page.locator('#project-open').click();
  await requested;
  await page.locator('#projects-cancel').click();
  release();
  await expect(page.locator('#projects-toggle')).toBeFocused();
  await expect(editor(page)).toHaveAttribute('contenteditable', 'true');
  await editor(page).fill('Still editing this project');
  await fileIs(page, 'notes.txt');
});

test('project dialog and long recent paths fit a narrow window', async ({page, appURL}, info) => {
  await page.setViewportSize({width:390,height:640});
  await page.goto(appURL);
  await page.route('**/projects', route => route.fulfill({json:{current:'/Users/me/notes',recent:['/Users/me/' + 'a-long-project-folder/'.repeat(7)]}}));
  await page.locator('#projects-toggle').click();
  await expect(page.locator('#recent-projects button')).toBeVisible();
  for (const selector of ['#projects-dialog', '#project-open', '#recent-projects button']) {
    const box = (await page.locator(selector).boundingBox())!;
    expect(box.x).toBeGreaterThanOrEqual(0);
    expect(box.x + box.width).toBeLessThanOrEqual(390);
  }
  await page.screenshot({path:info.outputPath('projects-narrow.png')});
  await page.locator('#project-path').press('Escape');
  await expect(page.locator('#projects-toggle')).toBeFocused();
});
