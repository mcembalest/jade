import { test, expect, editor, saved } from './fixtures';
import { mkdir, writeFile, readFile, cp, readdir, rm } from 'node:fs/promises';
import { join } from 'node:path';
import type { Page } from '@playwright/test';

test.use({ deviceScaleFactor: 2 });

const longName = 'a-long-filename-that-must-not-push-save-controls-out-of-the-visible-window.txt';
const longError = 'The operation could not be completed. '.repeat(8);
const terminals = {apps:[{name:'Terminal',path:'terminal'},{name:'Ghostty',path:'ghostty'}],selected:'terminal',overridden:false};
async function fits(page: Page, selectors: string[]) {
  const size = page.viewportSize()!;
  for (const selector of selectors) {
    const element = page.locator(selector);
    await expect(element).toBeVisible();
    const box = (await element.boundingBox())!;
    expect(box.x, selector).toBeGreaterThanOrEqual(-1);
    expect(box.y, selector).toBeGreaterThanOrEqual(-1);
    expect(box.x + box.width, selector).toBeLessThanOrEqual(size.width + 1);
    expect(box.y + box.height, selector).toBeLessThanOrEqual(size.height + 1);
    expect(await element.evaluate(node => {
      const box = node.getBoundingClientRect();
      const hit = document.elementFromPoint(box.x + box.width / 2, box.y + box.height / 2);
      return hit === node || node.contains(hit);
    }), selector + ' is not obscured').toBe(true);
  }
}

test.beforeEach(async ({ page, workspace }) => {
  await page.route('**/terminals', route => route.fulfill({json:terminals}));
  await writeFile(join(workspace, longName), 'A long file\n'.repeat(100));
  await mkdir(join(workspace, 'a-very-long-directory-name-that-needs-an-ellipsis'), {recursive:true});
  await writeFile(join(workspace, 'a-very-long-directory-name-that-needs-an-ellipsis/notes.txt'), 'Nested notes');
  await cp('examples/mnist/mlx/learning.svg', join(workspace, 'plot.svg'));
  await writeFile(join(workspace, 'jade.md'), '# A workspace with a long but useful descriptive name\n\n' +
    'https://example.com/' + 'long'.repeat(80) + '\n\n' +
    '| Column |' + ' Measurement |'.repeat(8) + '\n| --- |' + ' --- |'.repeat(8) + '\n| Local |' + ' value |'.repeat(8) +
    '\n\n```python\n' + 'print("long code") '.repeat(30) + '\n```\n\n' + '![Learning curve](plot.svg)\n\n' + '## More notes\n\nA useful paragraph.\n\n'.repeat(20));
});

for (const size of [{width:1440,height:900},{width:900,height:600},{width:760,height:520},{width:390,height:844}]) {
  test(`workspace, preview and search fit ${size.width}`, async ({ page, appURL }, info) => {
    await page.setViewportSize(size);
    await page.goto(appURL);
    await expect(page.locator('body')).toHaveAttribute('data-drafts-ready', 'true');
    await fits(page, ['header', '#terminal-toggle', '#terminal-select', '#files-toggle', '#editor', '#file-name', '#save-status']);
    // WebKit paints a native contenteditable caret despite screenshot caret hiding.
    // Capture this initial layout unfocused; keyboard focus is exercised below.
    await editor(page).blur();
    await expect(page).toHaveScreenshot(`workspace-${size.width}.png`, {scale:'css'});
    await page.screenshot({scale:'css',path:info.outputPath('workspace.png')});
    if (await page.locator('#preview-toggle').getAttribute('aria-pressed') === 'false') await page.locator('#preview-toggle').click();
    await fits(page, ['#editor', '#view-frame']);
    const preview = page.frameLocator('#view-frame');
    await expect(preview.getByRole('heading', {level:1})).toBeVisible();
    expect(await preview.locator('html').evaluate(node => node.scrollWidth <= node.clientWidth + 1)).toBe(true);
    await preview.locator('body').evaluate(node => node.ownerDocument.defaultView!.scrollTo(0, node.scrollHeight));
    await page.screenshot({scale:'css',path:info.outputPath('preview-bottom.png')});
    await page.locator('#preview-toggle').click();
    await expect(page.locator('#resolved')).not.toBeVisible();
    await page.locator('#preview-toggle').click();
    await expect(page.locator('#resolved')).toBeVisible();
    if (await page.locator('#files-toggle').getAttribute('aria-expanded') === 'false') await page.locator('#files-toggle').click();
    await page.getByRole('link', {name:longName,exact:true}).click();
    await expect(page.locator('#file-name')).toHaveText(longName);
    await expect(page.locator('body')).toHaveAttribute('data-drafts-ready', 'true');
    if (size.width < 700) await expect(page.locator('#file-explorer')).not.toBeVisible();
    await editor(page).press('ControlOrMeta+f');
    await fits(page, ['#file-name', '#save-status', '.cm-search', '.cm-search input[name="search"]']);
    if (size.width === 390) await expect(page).toHaveScreenshot('search-narrow.png', {scale:'css'});
    await page.screenshot({scale:'css',path:info.outputPath('search.png')});
    await page.locator('.cm-search input[name="search"]').fill('long');
    await page.locator('.cm-search input[name="search"]').press('Escape');
    await expect(page.locator('.cm-search')).not.toBeVisible();
    await editor(page).press('Escape');
    await editor(page).press('Tab');
    await expect(editor(page)).not.toBeFocused();
  });
}

test('new file errors stay in the dialog and retry succeeds', async ({page,appURL,workspace},info) => {
  await page.setViewportSize({width:390,height:640});
  await page.goto(appURL+'/?file=notes.txt');
  await page.locator('#files-toggle').click();
  await page.locator('#new-file').click();
  const path = page.getByRole('textbox',{name:'New file path'});
  await path.fill('notes.txt');
  await page.getByRole('button',{name:'Create file',exact:true}).click();
  await expect(page.locator('#new-file-error')).toContainText('already exists');
  await expect(path).toHaveValue('notes.txt');
  await expect(path).toBeFocused();
  await fits(page,['#new-file-dialog', '#new-file-error', '#new-file-cancel']);
  await expect(page).toHaveScreenshot('new-file-error.png', {scale:'css'});
  await page.screenshot({scale:'css',path:info.outputPath('new-file-error.png')});
  await path.fill('new notes.md');
  await page.getByRole('button',{name:'Create file',exact:true}).click();
  await expect(page.locator('#file-name')).toHaveText('new notes.md');
  expect(await readFile(join(workspace,'new notes.md'),'utf8')).toBe('');
});

test('terminal notices fit, dismiss, and recover after failure', async ({page,appURL},info) => {
  await page.setViewportSize({width:390,height:640});
  await page.route('**/terminal', route=>route.fulfill({status:503,json:{error:longError}}));
  await page.route('**/terminal/preference', route=>route.fulfill({json:{...terminals,selected:'ghostty'}}));
  await page.goto(appURL);
  await page.selectOption('#terminal-select','ghostty');
  await page.getByRole('button',{name:'Open terminal',exact:true}).click();
  await expect(page.locator('#terminal-status')).toContainText('could not');
  await fits(page,['header','#terminal-notice','#dismiss-terminal','#editor']);
  await expect(page).toHaveScreenshot('terminal-error.png', {scale:'css'});
  await page.screenshot({scale:'css',path:info.outputPath('terminal-error.png')});
  await page.getByRole('button',{name:'Dismiss terminal message'}).click();
  await expect(page.locator('#terminal-notice')).not.toBeVisible();
  await page.unroute('**/terminal');
  await page.route('**/terminal', route=>route.fulfill({json:{message:'Opened Ghostty'}}));
  await editor(page).press('ControlOrMeta+j');
  await expect(page.locator('#terminal-status')).toHaveText('Opened Ghostty');
  await page.screenshot({scale:'css',path:info.outputPath('terminal-success.png')});
});

test('recovery has a visible Save action and errors leave room for editing', async ({page,appURL},info) => {
  await page.setViewportSize({width:760,height:520});
  const file = await (await page.request.get(appURL+'/file?jade=.&file=jade.md')).json();
  const draft = await page.request.post(appURL+'/drafts',{form:{jade:'.',file:'jade.md',id:'11111111-1111-1111-1111-111111111111',revision:file.revision,content:'# Recovered note\n'}});
  expect(draft.ok()).toBe(true);
  await page.goto(appURL);
  await expect(page.locator('#recover-draft')).toBeVisible();
  await fits(page,['#draft-select','#recover-draft','#download-draft','#discard-draft','#editor']);
  await expect(page).toHaveScreenshot('recovery.png', {scale:'css',mask:[page.locator('#draft-select')]});
  await page.screenshot({scale:'css',path:info.outputPath('recovery.png')});
  await page.getByRole('button',{name:'Recover draft',exact:true}).click();
  await expect(page.getByRole('button',{name:'Save',exact:true})).toBeVisible();
  await page.getByRole('button',{name:'Save',exact:true}).click();
  await saved(page);
  await expect(page.frameLocator('#view-frame').getByRole('heading',{name:'Recovered note',exact:true})).toBeVisible();
  await page.route('**/save',route=>route.fulfill({status:503,body:longError}));
  await editor(page).fill('Unsaved');
  await editor(page).press('ControlOrMeta+s');
  await expect(page.locator('#save-status')).toContainText('Not saved');
  await fits(page,['#save-now','#reload-file','#save-copy','#save-status','#editor']);
  expect((await page.locator('#editor').boundingBox())!.height).toBeGreaterThan(60);
  await expect(page).toHaveScreenshot('save-error.png', {scale:'css'});
  await page.screenshot({scale:'css',path:info.outputPath('save-error.png')});
});

test('linked artifacts are explicit and remain selected across refresh', async ({page,appURL,workspace,browserName,expectedPageErrors}) => {
  await writeFile(join(workspace,'artifact.md'),'# Artifact output\n');
  await writeFile(join(workspace,'jade.md'),'# Workspace introduction\n\n[Read the artifact](artifact.md)\n');
  await page.goto(appURL);
  await expect(page.frameLocator('#view-frame').getByRole('heading',{name:'Workspace introduction',exact:true})).toBeVisible();
  // WebKit 26.6 emits this native sandbox diagnostic during the click even
  // though user-activated navigation succeeds. Keep the sandbox and assert
  // both the exact diagnostic and the resulting navigation, not a blanket filter.
  if (browserName === 'webkit') expectedPageErrors.push(`${appURL.replace('http:/','')}/' from frame with URL '${appURL}/front?jade=.'. The frame attempting navigation of the top-level window is sandboxed, but the 'allow-top-navigation' flag is not set.\n`);
  await page.frameLocator('#view-frame').getByRole('link',{name:'Read the artifact'}).click();
  await expect(page.frameLocator('#view-frame').getByRole('heading',{name:'Artifact output',exact:true})).toBeVisible();
  await editor(page).fill('# Workspace updated\n\n[Read the artifact](artifact.md)\n');
  await editor(page).press('ControlOrMeta+s');
  await saved(page);
  await expect(page.locator('.project')).toHaveText('Workspace updated');
  await expect(page.frameLocator('#view-frame').getByRole('heading',{name:'Artifact output',exact:true})).toBeVisible();
  await page.locator('.file-link[data-file="jade.md"][data-jade="."]').click();
  await expect(page.frameLocator('#view-frame').getByRole('heading',{name:'Workspace updated',exact:true})).toBeVisible();
});

test('folders, go-to-line, dialog Escape and backup messages behave through navigation', async ({page,appURL},info) => {
  await page.goto(appURL+'/?file='+longName);
  await expect(page.locator('body')).toHaveAttribute('data-drafts-ready','true');
  const folder=page.locator('summary').filter({hasText:'a-very-long-directory-name'});
  const nested=page.locator('a[data-file="a-very-long-directory-name-that-needs-an-ellipsis/notes.txt"]');
  await folder.click(); await expect(nested).not.toBeVisible();
  await folder.click(); await expect(nested).toBeVisible();
  await editor(page).press('ControlOrMeta+Alt+g');
  await expect(page.locator('.cm-goto-line')).toBeVisible();
  await fits(page,['.cm-goto-line']);
  await page.screenshot({scale:'css',path:info.outputPath('go-to-line.png')});
  await page.locator('.cm-goto-line input').fill('95');
  await page.locator('.cm-goto-line input').press('Enter');
  await expect(page.locator('.cm-goto-line')).not.toBeVisible();
  await expect(page.locator('.cm-lineNumbers .cm-activeLineGutter')).toHaveText('95');
  await page.locator('#new-file').click();
  await page.getByRole('textbox',{name:'New file path'}).press('Escape');
  await expect(page.locator('#new-file-dialog')).not.toBeVisible();
  await expect(page.locator('#new-file')).toBeFocused();
  await page.route('**/drafts',r=>r.fulfill({status:503,body:longError}));
  await editor(page).fill('Backup failure, ordinary save works');
  await editor(page).press('ControlOrMeta+s');
  await saved(page);
  await expect(page.locator('#draft-status')).toContainText('Recovery backup unavailable');
  await fits(page,['#draft-status','#editor']);
  await page.screenshot({scale:'css',path:info.outputPath('backup-error.png')});
  await page.getByRole('link',{name:'code.py',exact:true}).click();
  await expect(page.locator('#file-name')).toHaveText('code.py');
  await expect(page.locator('#draft-status')).toBeEmpty();
});


test('empty workspace and live resizing keep navigation usable', async ({page,appURL,workspace},info) => {
  for (const name of await readdir(workspace)) await rm(join(workspace,name),{recursive:true,force:true});
  await page.goto(appURL);
  await expect(page.locator('#empty-editor')).toBeVisible();
  await expect(page.locator('#preview-toggle')).not.toBeVisible();
  await fits(page,['#empty-editor','#new-file']);
  await page.screenshot({scale:'css',path:info.outputPath('empty-workspace.png')});
  await page.locator('#new-file').click();
  await page.getByRole('textbox',{name:'New file path'}).fill('jade.md');
  await page.getByRole('button',{name:'Create file',exact:true}).click();
  await expect(page.locator('#file-name')).toHaveText('jade.md');
  await expect(page.frameLocator('#view-frame').getByRole('heading',{name:'Untitled JaDE'})).toBeVisible();
  await page.setViewportSize({width:390,height:640});
  await expect(page.locator('#file-explorer')).not.toBeVisible();
  await page.locator('#preview-toggle').click();
  await fits(page,['#editor','#files-toggle','#preview-toggle']);
  await page.setViewportSize({width:1440,height:900});
  await expect(page.locator('#file-explorer')).toBeVisible();
  await page.locator('#preview-toggle').click();
  await fits(page,['#editor','#view-frame','#file-explorer']);
});
