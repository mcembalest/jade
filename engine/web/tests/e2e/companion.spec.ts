import { readFile } from 'node:fs/promises';
import { join } from 'node:path';
import { revealFiles, test, expect, editor } from './fixtures';

test('companion chat, preferences and keyboard dismissal', async ({ page, appURL }, info) => {
  await page.route('**/companion',route=>route.fulfill({json:{messages:[],pending:[],enabled:true,paused:false,seen:''}}));
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await page.goto(appURL);
  await page.getByRole('button', { name: 'Visit Sanjana’s corner' }).click();
  await expect(page.getByRole('textbox', { name: 'Message Sanjana' })).toBeVisible();
  await expect(page.getByRole('log')).toContainText('Tell me what you’re in the mood for');
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
  await revealFiles(page);
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

test('chat sends, persists, and safely displays linked replies', async ({page,appURL}) => {
  const state = {messages:[] as {id:string;role:string;text:string;sources?:{title:string;url:string}[]}[],enabled:true,next:Date.now()+3_600_000,seen:''};
  await page.route('**/companion', async route => {
    const body = route.request().postDataJSON();
    if (body?.action === 'chat') {
      state.messages.push({id:'1',role:'user',text:body.message},{id:'2',role:'assistant',text:'<script>alert(1)</script> A little discovery.',sources:[{title:'Read the story',url:'https://example.com/story'},{title:'Unsafe',url:'javascript:alert(1)'}]});
    }
    if (body?.action === 'seen') state.seen = body.seen;
    await route.fulfill({json:state});
  });
  await page.goto(appURL);
  await page.locator('#companion-toggle').click();
  await page.getByRole('textbox',{name:'Message Sanjana'}).fill('Search for a vegetarian date spot');
  await page.getByRole('button',{name:'Send',exact:true}).click();
  await expect(page.getByRole('log')).toContainText('Search for a vegetarian date spot');
  await expect(page.getByRole('log')).toContainText('<script>alert(1)</script>');
  await expect(page.getByRole('link',{name:'Read the story'})).toHaveAttribute('href','https://example.com/story');
  await expect(page.getByRole('link',{name:'Unsafe'})).toHaveCount(0);
  await expect(page.locator('#companion-input')).toHaveValue('');
  await page.reload();
  await page.locator('#companion-toggle').click();
  await expect(page.getByRole('log')).toContainText('A little discovery.');
  await page.screenshot({path:'../../.tmp/companion-chat-'+test.info().project.name+'.png'});
});

test('opening, reloading and hiding never schedule cloud research or publication', async ({page,appURL}) => {
  await page.clock.install({time:new Date('2026-09-05T23:00:00Z')});
  const actions:string[]=[];
  const state={messages:[],pending:[{text:'Cloud discovery',sources:[{title:'Original source',url:'https://example.com/story'}]}],enabled:true,paused:false,next:0,researchNext:0,seen:''};
  await page.route('**/companion',async route=>{
    const body=route.request().postDataJSON();
    if(body) {actions.push(body.action);if(body.action==='settings')state.paused=body.paused;}
    await route.fulfill({json:state});
  });
  await page.goto(appURL);
  await page.locator('#companion-toggle').click();
  await page.locator('#companion-research-panel summary').click();
  await expect(page.locator('#companion-research')).toContainText('Cloud discovery');
  await page.clock.fastForward(25*60*60_000);
  await page.reload();
  await page.locator('#companion-toggle').click();
  await page.locator('#companion-hide').click();
  expect(actions).toEqual([]);
  await page.locator('#companion-restore').click();
  await page.locator('#companion-toggle').click();
  await page.locator('#companion-pause').click();
  await expect(page.locator('#companion-pause')).toHaveText('Resume research everywhere');
  expect(actions).toEqual(['settings']);
  await expect(page.locator('#companion-dock')).toBeVisible();
});

test('chat failures retain the message and requests can be stopped', async ({page,appURL}) => {
  let fail = true;
  await page.route('**/companion', async route => {
    if (route.request().postDataJSON()?.action === 'chat') {
      if (fail) await route.fulfill({status:503,body:'Run codex login and sign in with ChatGPT.'});
      // Leave the second request pending until Stop aborts it.
      return;
    }
    await route.fulfill({json:{messages:[],enabled:true,next:Date.now()+3_600_000,seen:''}});
  });
  await page.goto(appURL);
  await page.locator('#companion-toggle').click();
  await page.locator('#companion-input').fill('Hello');
  await page.locator('#companion-input').press('Enter');
  await expect(page.locator('#companion-status')).toContainText('codex login');
  await expect(page.locator('#companion-input')).toHaveValue('Hello');
  fail = false;
  await page.locator('#companion-send').click();
  await expect(page.locator('#companion-stop')).toBeVisible();
  await page.locator('#companion-stop').click();
  await expect(page.locator('#companion-status')).toHaveText('Stopped.');
  await expect(page.locator('#companion-send')).toBeEnabled();
});
