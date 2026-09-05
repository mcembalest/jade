import { test, expect, editor, documentText, fileIs, saved } from './fixtures';
import { mkdir, readFile, writeFile } from 'node:fs/promises';
import { join } from 'node:path';

test.beforeEach(async ({page}) => {
  await page.route('**/terminals', r => r.fulfill({json:{apps:[{name:'Terminal',path:'terminal'}],selected:'terminal'}}));
});

async function ready(page: import('@playwright/test').Page) {
  await expect(page.locator('body')).toHaveAttribute('data-drafts-ready','true');
}

test('writing: draft Markdown, revise a phrase, preview, and reopen', async ({page,appURL,workspace}) => {
  await page.goto(appURL); await ready(page);
  const draft = '# Experiment notes\n\nThe first result needs a second check.\n\n- Dataset: MNIST\n- Next: repeat the run\n';
  await editor(page).fill(draft);
  await editor(page).press('ControlOrMeta+f');
  await page.getByRole('textbox',{name:'Find',exact:true}).fill('second check');
  await page.getByRole('textbox',{name:'Replace',exact:true}).fill('fresh run');
  await page.getByRole('button',{name:'replace all',exact:true}).click();
  await page.getByRole('textbox',{name:'Find',exact:true}).press('Escape');
  await editor(page).press('ControlOrMeta+s'); await saved(page);
  const expected = draft.replace('second check','fresh run');
  expect(await readFile(join(workspace,'jade.md'),'utf8')).toBe(expected);
  const preview = page.frameLocator('#view-frame');
  await expect(preview.getByRole('heading',{name:'Experiment notes'})).toBeVisible();
  await expect(preview.locator('p')).toHaveText('The first result needs a fresh run.');
  await page.reload(); await ready(page);
  expect(await documentText(page)).toBe(expected);
});

test('coding: type a function, indent, toggle comments, and save', async ({page,appURL,workspace}) => {
  await page.goto(appURL+'/?file=code.py'); await ready(page);
  await editor(page).fill('');
  await editor(page).pressSequentially('def square(x):');
  await editor(page).press('Enter');
  await editor(page).pressSequentially('return x * x');
  const code = 'def square(x):\n  return x * x';
  expect(await documentText(page)).toBe(code);
  await editor(page).press('ControlOrMeta+/');
  expect(await documentText(page)).toContain('# return x * x');
  await editor(page).press('ControlOrMeta+/');
  expect(await documentText(page)).toBe(code);
  await page.getByRole('link',{name:'notes.txt',exact:true}).click();
  await fileIs(page,'notes.txt');
  expect(await readFile(join(workspace,'code.py'),'utf8')).toBe(code);
  await page.getByRole('link',{name:'code.py',exact:true}).click();
  await fileIs(page,'code.py'); await ready(page);
  expect(await documentText(page)).toBe(code);
});

test('notes: create a meeting note, continue a list, switch workspace, return', async ({page,appURL,workspace}) => {
  await page.goto(appURL); await ready(page);
  await page.getByRole('button',{name:'New file',exact:true}).click();
  await page.getByRole('textbox',{name:'New file path'}).fill('meetings/2026-09-04.md');
  await page.getByRole('button',{name:'Create file',exact:true}).click();
  await fileIs(page,'meetings/2026-09-04.md'); await ready(page);
  await editor(page).fill('# Meeting\n\n- Check latency');
  await editor(page).press('ControlOrMeta+End');
  await editor(page).press('Enter');
  await editor(page).pressSequentially('Repeat on GPU');
  const note = '# Meeting\n\n- Check latency\n- Repeat on GPU';
  expect(await documentText(page)).toBe(note);
  await page.locator('a[data-jade="inner"][data-file="inner/jade.md"]').click();
  await expect(page.locator('body')).toHaveAttribute('data-jade','inner');
  expect(await readFile(join(workspace,'meetings/2026-09-04.md'),'utf8')).toBe(note);
  await page.getByRole('link',{name:'JADE',exact:true}).click();
  await ready(page);
  await page.getByRole('link',{name:'2026-09-04.md',exact:true}).click();
  await fileIs(page,'meetings/2026-09-04.md'); await ready(page);
  expect(await documentText(page)).toBe(note);
});

test('navigation: edit two files and use browser Back and Forward', async ({page,appURL,workspace}) => {
  await page.goto(appURL+'/?file=notes.txt'); await ready(page);
  await editor(page).fill('Note before switching');
  await page.getByRole('link',{name:'code.py',exact:true}).click();
  await fileIs(page,'code.py'); await ready(page);
  await editor(page).fill('print("updated")');
  await page.goBack();
  await fileIs(page,'notes.txt'); await ready(page);
  expect(await documentText(page)).toBe('Note before switching');
  expect(await readFile(join(workspace,'code.py'),'utf8')).toBe('print("updated")');
  await page.goForward(); await fileIs(page,'code.py'); await ready(page);
  expect(await documentText(page)).toBe('print("updated")');
});

test('long notes: find a passage, revise it, and preserve the surrounding document', async ({page,appURL,workspace}) => {
  const original = Array.from({length:1200},(_,i)=>`Observation ${i+1}: unchanged`).join('\n')+'\n';
  await writeFile(join(workspace,'notes.txt'),original);
  await page.goto(appURL+'/?file=notes.txt'); await ready(page);
  await editor(page).press('ControlOrMeta+f');
  await page.getByRole('textbox',{name:'Find',exact:true}).fill('Observation 900: unchanged');
  await page.getByRole('textbox',{name:'Replace',exact:true}).fill('Observation 900: verified');
  await page.getByRole('button',{name:'replace all',exact:true}).click();
  await page.getByRole('textbox',{name:'Find',exact:true}).press('Escape');
  await editor(page).press('ControlOrMeta+s'); await saved(page);
  expect(await readFile(join(workspace,'notes.txt'),'utf8')).toBe(original.replace('Observation 900: unchanged','Observation 900: verified'));
  await editor(page).press('ControlOrMeta+End');
  await expect(page.locator('.cm-line').filter({hasText:'Observation 1200: unchanged'})).toBeVisible();
});

test('reading: a report in a subdirectory resolves its own figure', async ({page,appURL,workspace},info) => {
  await mkdir(join(workspace,'reports'),{recursive:true});
  await writeFile(join(workspace,'reports/result.md'),'# Result\n\n![Latency plot](latency.svg)\n');
  await writeFile(join(workspace,'reports/latency.svg'),'<svg xmlns="http://www.w3.org/2000/svg" width="200" height="100"><rect width="200" height="100" fill="green"/></svg>');
  await page.goto(appURL+'/?view=reports/result.md'); await ready(page);
  const preview = page.frameLocator('#view-frame');
  await expect(preview.getByRole('heading',{name:'Result'})).toBeVisible();
  await expect.poll(()=>preview.getByRole('img',{name:'Latency plot'}).evaluate(i=>(i as HTMLImageElement).naturalWidth)).toBe(200);
  await page.screenshot({path:info.outputPath('report.png')});
});

test('navigation failure: Back keeps the current file and address until edits are saved', async ({page,appURL,workspace}) => {
  await page.goto(appURL+'/?file=notes.txt'); await ready(page);
  await page.getByRole('link',{name:'code.py',exact:true}).click();
  await fileIs(page,'code.py'); await ready(page);
  const currentURL = page.url();
  await page.route('**/save',r=>r.fulfill({status:503,body:'Disk unavailable'}));
  await editor(page).fill('print("keep this")');
  await page.goBack();
  await expect(page.locator('#save-status')).toContainText('Not saved');
  await fileIs(page,'code.py');
  await expect(page).toHaveURL(currentURL);
  expect(await documentText(page)).toBe('print("keep this")');
  await page.unroute('**/save');
  await editor(page).press('ControlOrMeta+s'); await saved(page);
  expect(await readFile(join(workspace,'code.py'),'utf8')).toBe('print("keep this")');
  await page.goBack(); await fileIs(page,'notes.txt'); await ready(page);
});

test('navigation during a slow save: repeated Back keeps unsaved text in its file', async ({page,appURL,workspace}) => {
  await page.goto(appURL); await ready(page);
  await page.getByRole('link',{name:'notes.txt',exact:true}).click(); await fileIs(page,'notes.txt'); await ready(page);
  await page.getByRole('link',{name:'code.py',exact:true}).click(); await fileIs(page,'code.py'); await ready(page);
  const currentURL = page.url();
  let release!: () => void;
  const held = new Promise<void>(resolve=>{release=resolve;});
  await page.route('**/save',async r=>{await held; await r.fulfill({status:503,body:'Disk unavailable'});});
  await editor(page).fill('print("keep this too")');
  await page.goBack();
  await expect(page.locator('#save-status')).toHaveText('Saving…');
  await page.goBack();
  await expect(page).toHaveURL(currentURL);
  release();
  await expect(page.locator('#save-status')).toContainText('Not saved');
  await expect(page).toHaveURL(currentURL);
  await fileIs(page,'code.py');
  expect(await documentText(page)).toBe('print("keep this too")');
  expect(await readFile(join(workspace,'code.py'),'utf8')).toBe('print("hello")\n');
});

test('repeated writing and coding: switching, external changes, and reopening preserve each file', async ({page,appURL,workspace}) => {
  test.setTimeout(30_000);
  const files = ['notes.txt','code.py','reading.md'];
  await writeFile(join(workspace,'reading.md'),'Reading notes\n');
  await page.goto(appURL+'/?file=notes.txt'); await ready(page);
  const expected = new Map<string,string>();
  for(let round=0;round<6;round++) {
    for(const file of files) {
      await page.getByRole('link',{name:file,exact:true}).click(); await fileIs(page,file); await ready(page);
      if(expected.has(file)) expect(await documentText(page)).toBe(expected.get(file));
      const contents = `${file}: revision ${round}\n\nNotes: café, 😀, 漢字\n`;
      await editor(page).fill(contents); expected.set(file,contents);
    }
  }
  await editor(page).press('ControlOrMeta+s'); await saved(page);
  for(const file of files) expect(await readFile(join(workspace,file),'utf8')).toBe(expected.get(file));
  await writeFile(join(workspace,'reading.md'),'Updated by an external tool\n');
  await expect.poll(()=>documentText(page)).toBe('Updated by an external tool\n');
  expected.set('reading.md','Updated by an external tool\n');
  await page.reload(); await ready(page);
  for(const file of files) {
    await page.getByRole('link',{name:file,exact:true}).click(); await fileIs(page,file); await ready(page);
    expect(await documentText(page)).toBe(expected.get(file));
  }
});
