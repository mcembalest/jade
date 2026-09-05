import { test, expect, editor, fileIs } from './fixtures';
import { mkdir, writeFile, readFile } from 'node:fs/promises';
import { join } from 'node:path';

test.beforeEach(async ({workspace,page})=>{
  await mkdir(join(workspace,'study','report'),{recursive:true});
  await writeFile(join(workspace,'README.md'),'# Project\n\n[Study](study/)\n\n[Outside](//example.test/article)\n');
  await writeFile(join(workspace,'study','README.md'),'# Study\n\n[Report](report/notes.md)\n\n[Home](../README.md)\n');
  await writeFile(join(workspace,'study','report','notes.md'),'# Report\n\n[Shared code](../../code.py)\n\n![Figure](../../plot.svg)\n\n[Methods](#methods)\n\n'+('Evidence paragraph.\n\n'.repeat(25))+'## Methods\n\nVerified. [Shared implementation](../../code.py)');
  await writeFile(join(workspace,'plot.svg'),'<svg xmlns="http://www.w3.org/2000/svg" width="160" height="80"><rect width="160" height="80" fill="seagreen"/></svg>');
  await page.route('**/terminals',r=>r.fulfill({json:{apps:[{name:'Terminal',path:'terminal'}],selected:'terminal'}}));
});
const cards=(page:import('@playwright/test').Page)=>page.locator('.preview:not([hidden])');

test('three levels preserve parents, follow sibling links, and edit without losing the original draft', async ({page,appURL,workspace},info)=>{
  await page.goto(appURL+'/?file=notes.txt');
  await editor(page).fill('An unfinished thought');
  await page.locator('#files-toggle').click();
  await page.locator('.file-link[data-file="README.md"][data-jade="."]').hover();
  await expect(cards(page)).toHaveCount(1);
  await cards(page).last().locator('.preview-keep').click();
  await cards(page).last().locator('iframe').contentFrame().getByRole('link',{name:'Study',exact:true}).click();
  await expect(cards(page)).toHaveCount(2);
  await cards(page).last().locator('iframe').contentFrame().getByRole('link',{name:'Report',exact:true}).click();
  await expect(cards(page)).toHaveCount(3);
  await cards(page).last().locator('iframe').contentFrame().getByRole('link',{name:'Shared code'}).click();
  await expect(cards(page)).toHaveCount(4);
  await expect(cards(page).last().locator('iframe').contentFrame().locator('pre')).toContainText('print("hello")');
  await fileIs(page,'notes.txt');
  await page.screenshot({path:info.outputPath('recursive-desktop.png')});
  await cards(page).last().locator('.preview-edit').click();
  await fileIs(page,'code.py');
  expect(await readFile(join(workspace,'notes.txt'),'utf8')).toBe('An unfinished thought');
  await expect(editor(page)).toBeFocused();
  await expect(cards(page)).toHaveCount(3);
  await cards(page).last().locator('.preview-back').click();
  await expect(cards(page)).toHaveCount(2);
  await expect(cards(page).last().locator('iframe').contentFrame().getByRole('heading',{name:'Study',exact:true})).toBeVisible();
});

test('hover descendants keep the chain alive, and Escape from inside a preview returns to its parent', async ({page,appURL})=>{
  await page.goto(appURL);await page.locator('#preview-toggle').click();
  await cards(page).last().locator('iframe').contentFrame().getByRole('link',{name:'Study',exact:true}).hover();
  await expect(cards(page)).toHaveCount(2);
  await cards(page).last().locator('iframe').contentFrame().getByRole('link',{name:'Report',exact:true}).hover();
  await expect(cards(page)).toHaveCount(3);
  await cards(page).last().locator('.preview-keep').hover();
  await expect(cards(page)).toHaveCount(3);
  await cards(page).last().locator('iframe').contentFrame().getByRole('link',{name:'Shared code'}).press('Escape');
  await expect(cards(page)).toHaveCount(2);
  await expect(cards(page).last().locator('iframe').contentFrame().getByRole('link',{name:'Report',exact:true})).toBeFocused();
});

test('cycles reuse ancestors and previews fit narrow screens with a usable way back', async ({page,appURL},info)=>{
  await page.setViewportSize({width:390,height:740});
  await page.goto(appURL);await page.locator('#preview-toggle').click();
  await cards(page).last().locator('iframe').contentFrame().getByRole('link',{name:'Study',exact:true}).click();
  await cards(page).last().locator('iframe').contentFrame().getByRole('link',{name:'Home',exact:true}).click();
  await expect(cards(page)).toHaveCount(2);
  // The original card is raised; dismissing it leaves the kept study available.
  await page.locator('#preview-close').click();
  await expect(cards(page)).toHaveCount(1);
  await cards(page).last().locator('iframe').contentFrame().getByRole('link',{name:'Report',exact:true}).click();
  await expect(cards(page)).toHaveCount(2);
  const current=cards(page).last();
  const box=(await current.boundingBox())!;expect(box.x).toBeGreaterThanOrEqual(0);expect(box.x+box.width).toBeLessThanOrEqual(390);expect(box.y+box.height).toBeLessThanOrEqual(740);
  await page.screenshot({path:info.outputPath('recursive-narrow.png')});
  await current.locator('.preview-back').click();await expect(cards(page)).toHaveCount(1);
});

test('images open locally, fragments stay in place and document scripts cannot run', async ({page,appURL,workspace})=>{
  await writeFile(join(workspace,'unsafe.html'),'<script>parent.document.body.dataset.compromised="yes"</script><h1>Source</h1>');
  await page.goto(appURL+'/?file=study/report/notes.md');await page.locator('#preview-toggle').click();
  const report=cards(page).first().locator('iframe').contentFrame();
  await report.getByRole('img',{name:'Figure'}).click();await expect(cards(page)).toHaveCount(2);
  await expect(cards(page).last().locator('iframe').contentFrame().getByRole('img')).toBeVisible();
  await cards(page).last().locator('.preview-close').click();
  await report.getByRole('link',{name:'Methods',exact:true}).click();
  await expect(report.getByRole('heading',{name:'Methods',exact:true})).toBeInViewport();
  const scroll=await report.locator('html').evaluate(node=>node.ownerDocument.defaultView!.scrollY);
  await report.getByRole('link',{name:'Shared implementation'}).click();
  await expect(cards(page)).toHaveCount(2);
  await cards(page).last().locator('.preview-back').click();
  await expect.poll(()=>report.locator('html').evaluate(node=>node.ownerDocument.defaultView!.scrollY)).toBe(scroll);
  await expect(cards(page)).toHaveCount(1);
  const response=await page.request.get(appURL+'/view?file=unsafe.html');
  expect(response.headers()['content-security-policy']).toContain("default-src 'none'");
  expect(await response.text()).toContain('&lt;script&gt;');
  await expect(page.locator('body')).not.toHaveAttribute('data-compromised','yes');
});

test('editing a preview keeps it available after a failed save, then succeeds from search', async ({page,appURL,workspace})=>{
  await page.goto(appURL+'/?file=notes.txt');
  await page.route('**/save',r=>r.fulfill({status:503,body:'Saving is temporarily unavailable'}));
  await editor(page).fill('Keep this draft');
  await page.locator('#search-toggle').click();
  await page.locator('#search-input').fill('README.md');
  await page.locator('#search-results button').first().hover();
  await expect(cards(page)).toHaveCount(1);
  await cards(page).last().locator('.preview-edit').click();
  await expect(cards(page).last().locator('.preview-status')).toContainText('Saving is temporarily unavailable');
  await fileIs(page,'notes.txt');
  await expect(page.locator('#search-dialog')).toBeVisible();
  await page.unroute('**/save');
  await cards(page).last().locator('.preview-edit').click();
  await fileIs(page,'README.md');
  await expect(page.locator('#search-dialog')).not.toBeVisible();
  await expect(editor(page)).toBeFocused();
  expect(await readFile(join(workspace,'notes.txt'),'utf8')).toBe('Keep this draft');
});

test('preview protection blocks embedded scripts while allowing trusted navigation', async ({page,appURL})=>{
  // Exercise the browser policy even if future rendering changes admit raw markup.
  await page.route('**/view?*',async route=>{
    const response=await route.fetch();
    const body=(await response.text()).replace('</body>', '<script>parent.document.body.dataset.compromised="yes"</script><button onclick="parent.document.body.dataset.compromised=\'yes\'">Unsafe action</button></body>');
    await route.fulfill({response,body});
  });
  await page.goto(appURL);await page.locator('#preview-toggle').click();
  const frame=cards(page).last().locator('iframe').contentFrame();
  await frame.getByRole('button',{name:'Unsafe action'}).click();
  await frame.getByRole('link',{name:'Study',exact:true}).click();
  await expect(cards(page)).toHaveCount(2);
  await expect(cards(page).last().locator('iframe').contentFrame().getByRole('heading',{name:'Study',exact:true})).toBeVisible();
  await expect(page.locator('body')).not.toHaveAttribute('data-compromised','yes');
  await page.context().route('http://example.test/**',r=>r.fulfill({body:'External reference'}));
  const popupPromise=page.waitForEvent('popup');
  await cards(page).first().locator('iframe').contentFrame().getByRole('link',{name:'Outside'}).click();
  const popup=await popupPromise;
  await expect(popup).toHaveURL('http://example.test/article');
  await popup.close();
  await expect(page).toHaveURL(appURL+'/');
});

test('a nested folder can edit shared code outside its own directory', async ({page,appURL,workspace})=>{
  await page.goto(appURL+'/?jade=study/report&file=notes.md');
  await page.locator('#preview-toggle').click();
  await cards(page).first().locator('iframe').contentFrame().getByRole('link',{name:'Shared code',exact:true}).click();
  await cards(page).last().locator('.preview-edit').click();
  await fileIs(page,'../../code.py');
  await expect(page.locator('body')).toHaveAttribute('data-jade','study/report');
  await editor(page).fill('print("shared edit")');
  await editor(page).press('ControlOrMeta+s');
  await expect.poll(()=>readFile(join(workspace,'code.py'),'utf8')).toBe('print("shared edit")');
});
