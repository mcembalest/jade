import { readFile } from 'node:fs/promises';
import { join } from 'node:path';
import { test, expect, editor } from './fixtures';

test('companion cards, preferences and keyboard dismissal', async ({ page, appURL }, info) => {
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await page.goto(appURL);
  await page.getByRole('button', { name: 'Visit Sanjana’s corner' }).click();
  await expect(page.getByRole('heading', { name: 'A little good in the world' })).toBeVisible();
  await page.getByRole('button', { name: 'Another little thing' }).click();
  await expect(page.getByRole('heading', { name: 'Something to fall in love with' })).toBeVisible();
  await page.getByRole('button', { name: 'Another little thing' }).click();
  await expect(page.getByRole('heading', { name: 'A lovely little rabbit hole' })).toBeVisible();
  await page.getByRole('checkbox', { name: 'Still animation' }).check();
  await page.screenshot({path:info.outputPath('companion-desktop.png')});
  await page.getByRole('button', { name: 'Hide Sanjana' }).click();
  await page.reload();
  await expect(page.locator('#companion-dock')).toBeHidden();
  await page.getByRole('button', { name: 'Show Sanjana' }).click();
  await page.getByRole('button', { name: 'Visit Sanjana’s corner' }).click();
  await expect(page.getByRole('checkbox', { name: 'Still animation' })).toBeChecked();
  await page.keyboard.press('Escape');
  await expect(page.locator('#companion-card')).toBeHidden();
  await expect(page.locator('#companion-toggle')).toBeFocused();
  await page.setViewportSize({width:390,height:844});
  await page.getByRole('button', { name: 'Visit Sanjana’s corner' }).click();
  const box = (await page.locator('#companion-card').boundingBox())!;
  expect(box.x).toBeGreaterThanOrEqual(0);
  expect(box.x + box.width).toBeLessThanOrEqual(390);
  await page.screenshot({path:info.outputPath('companion-mobile.png')});
  await page.keyboard.press('Escape');
  const dock = (await page.locator('#companion-dock').boundingBox())!;
  const edit = (await editor(page).boundingBox())!;
  expect(edit.y + edit.height).toBeLessThanOrEqual(dock.y + 1);
});

test('hidden companion controls fit a narrow window', async ({page,appURL}) => {
  await page.setViewportSize({width:320,height:568});
  await page.route('**/terminals', route => route.fulfill({json:{apps:[{name:'Terminal',path:'terminal'},{name:'Ghostty',path:'ghostty'}],selected:'terminal',overridden:false}}));
  await page.goto(appURL);
  await page.locator('#companion-toggle').click();
  await page.locator('#companion-hide').click();
  for (const id of ['companion-restore','terminal-select','terminal-toggle']) {
    const box = (await page.locator('#'+id).boundingBox())!;
    expect(box.x).toBeGreaterThanOrEqual(0);
    expect(box.x + box.width).toBeLessThanOrEqual(320);
  }
});

test('returning from the companion to writing preserves focus and saves edits', async ({page,appURL,workspace}) => {
  await page.goto(appURL);
  await page.getByRole('link', {name:'notes.txt',exact:true}).click();
  await page.locator('#companion-toggle').click();
  await editor(page).click();
  await expect(page.locator('#companion-card')).toBeHidden();
  await expect(editor(page)).toBeFocused();
  await editor(page).press('ControlOrMeta+End');
  await page.keyboard.insertText('\nWriting after visiting Sanjana.');
  await editor(page).press('ControlOrMeta+s');
  await expect(page.locator('#save-status')).toHaveText('Saved');
  await expect.poll(() => readFile(join(workspace,'notes.txt'),'utf8')).toContain('Writing after visiting Sanjana.');
  await page.locator('#companion-toggle').click();
  await page.keyboard.press('Escape');
  await expect(page.locator('#companion-card')).toBeHidden();
  await expect(page.locator('#companion-toggle')).toBeFocused();
});
