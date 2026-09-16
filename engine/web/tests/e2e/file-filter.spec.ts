import { test, expect, editor, fileIs } from './fixtures';
import { mkdir, readFile, writeFile } from 'node:fs/promises';
import { join } from 'node:path';

test('filter reveals nested paths and clearing restores the original folder layout', async ({ page, appURL, workspace }) => {
  await mkdir(join(workspace, 'drafts/deep'), { recursive: true });
  await writeFile(join(workspace, 'drafts/deep/weekend.txt'), 'Weekend plans');
  await page.goto(appURL);
  await page.locator('#files-toggle').click();
  await page.locator('.tree summary').filter({ hasText: 'inner' }).click();
  const expansion = await page.locator('.tree details').evaluateAll(nodes => nodes.map(node => (node as HTMLDetailsElement).open));
  const filter = page.getByRole('searchbox', { name: 'Filter filenames and paths' });
  await filter.fill('DRAFTS/DEEP/WEEK');
  await expect(page.locator('.tree a:visible')).toHaveCount(1);
  await expect(page.locator('.tree a:visible')).toHaveText('weekend.txt');
  await expect(page.locator('#file-filter-status')).toHaveText('1 matching file.');
  await filter.fill('no such filename');
  await expect(page.locator('.tree a:visible')).toHaveCount(0);
  await expect(page.locator('#file-filter-status')).toHaveText('No matching filenames or paths.');
  await filter.press('Enter');
  await filter.press('Escape');
  await expect(filter).toBeFocused();
  expect(await page.locator('.tree details').evaluateAll(nodes => nodes.map(node => (node as HTMLDetailsElement).open))).toEqual(expansion);
  await expect(page.locator('#file-filter-status')).toBeHidden();
  await filter.fill('weekend');
  await page.getByRole('button', { name: 'Clear file filter' }).click();
  expect(await page.locator('.tree details').evaluateAll(nodes => nodes.map(node => (node as HTMLDetailsElement).open))).toEqual(expansion);
});

test('Enter on a filtered subproject file uses existing save-before-navigation', async ({ page, appURL, workspace }) => {
  await page.goto(appURL + '/?file=notes.txt');
  await editor(page).fill('Before filtered project switch');
  await page.locator('#files-toggle').click();
  await page.locator('#file-filter').fill('inner/README.md');
  await expect(page.locator('.tree a:visible')).toHaveCount(1);
  await page.locator('#file-filter').press('Enter');
  await expect(page).toHaveURL(/jade=inner/);
  expect(await readFile(join(workspace, 'notes.txt'), 'utf8')).toBe('Before filtered project switch');
});

test('filtered navigation preserves unsaved edits when saving fails', async ({ page, appURL }) => {
  await page.goto(appURL + '/?file=notes.txt');
  await page.route('**/save', route => route.fulfill({ status: 503, body: 'Disk unavailable' }));
  await editor(page).fill('Keep my unsaved edit');
  await page.locator('#files-toggle').click();
  await page.locator('#file-filter').fill('code.py');
  await page.locator('#file-filter').press('Enter');
  await expect(page.locator('#save-status')).toContainText('Not saved');
  await fileIs(page, 'notes.txt');
  await expect(editor(page)).toHaveText('Keep my unsaved edit');
  await page.unroute('**/save');
  await page.locator('#file-filter').press('Enter');
  await fileIs(page, 'code.py');
  await expect(editor(page)).toBeFocused();
});

test('filter fits a narrow sidebar and does not match file contents', async ({ page, appURL }, info) => {
  await page.setViewportSize({ width: 390, height: 640 });
  await page.goto(appURL);
  await page.locator('#files-toggle').click();
  await page.locator('#file-filter').fill('Original note');
  await expect(page.locator('#file-filter-status')).toHaveText('No matching filenames or paths.');
  await page.locator('#file-filter').fill('README');
  const box = (await page.locator('.file-filter').boundingBox())!;
  expect(box.x).toBeGreaterThanOrEqual(0);
  expect(box.x + box.width).toBeLessThanOrEqual(390);
  await page.screenshot({ path: info.outputPath('file-filter-narrow.png') });
});

test('quick open focuses and selects the filename filter', async ({ page, appURL }) => {
  await page.goto(appURL + '/?file=notes.txt');
  await editor(page).press('ControlOrMeta+p');
  const filter=page.getByRole('searchbox',{name:'Filter filenames and paths'});
  await expect(filter).toBeFocused();
  await filter.fill('code.py');
  await filter.press('Enter');
  await fileIs(page,'code.py');
});

test('terminal waits for a successful save and preserves failed edits', async ({ page, appURL, workspace }) => {
  await page.goto(appURL + '/?file=notes.txt');
  let opened=0;
  await page.route('**/terminal', async route=>{
    expect(await readFile(join(workspace,'notes.txt'),'utf8')).toBe('Saved before terminal');
    opened++;await route.fulfill({json:{message:'Terminal opened'}});
  });
  await page.route('**/save',route=>route.fulfill({status:503,body:'Unavailable'}));
  await editor(page).fill('Saved before terminal');
  await page.getByRole('button',{name:'Open terminal',exact:true}).click();
  await expect(page.locator('#terminal-notice')).toContainText('Save or resolve');
  expect(opened).toBe(0);
  await expect(editor(page)).toHaveText('Saved before terminal');
  await page.unroute('**/save');
  await page.getByRole('button',{name:'Open terminal',exact:true}).click();
  await expect(page.locator('#terminal-notice')).toContainText('Terminal opened');
  expect(opened).toBe(1);
});
