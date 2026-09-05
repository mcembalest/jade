import { test, expect, editor, documentText } from './fixtures';
import { readFile, writeFile } from 'node:fs/promises';
import { join } from 'node:path';
import { createHash } from 'node:crypto';
import { cpus } from 'node:os';

for (const target of ['JaDE', 'CodeMirror baseline']) {
  test.describe(target, () => {
    test.beforeEach(async ({ page, appURL, baselineURL }) => {
      await page.goto(target === 'JaDE' ? appURL + '/?file=notes.txt' : baselineURL);
    });

    test('Unicode text survives editing', async ({ page }) => {
      const text = 'naïve 😀 漢字\nsecond line\n';
      await editor(page).fill(text);
      expect(await documentText(page)).toBe(text);
    });

    test('undo and redo restore the edit', async ({ page }) => {
      const original = await documentText(page);
      await editor(page).fill('replacement');
      await editor(page).press('ControlOrMeta+z');
      expect(await documentText(page)).toBe(original);
      await editor(page).press('ControlOrMeta+Shift+z');
      expect(await documentText(page)).toBe('replacement');
    });

    test('Tab and Shift-Tab indent and unindent', async ({ page }) => {
      await editor(page).fill('alpha');
      await editor(page).press('Home');
      await editor(page).press('Tab');
      expect(await documentText(page)).toBe('  alpha');
      await editor(page).press('Shift+Tab');
      expect(await documentText(page)).toBe('alpha');
    });

    test('search and replace use the editor controls', async ({ page }) => {
      await editor(page).fill('oak oak');
      await editor(page).press('ControlOrMeta+f');
      await page.getByRole('textbox', { name: 'Find', exact: true }).fill('oak');
      await page.getByRole('textbox', { name: 'Replace', exact: true }).fill('pine');
      await page.getByRole('button', { name: /^replace all$/i }).click();
      expect(await documentText(page)).toBe('pine pine');
    });

    test('replace a selected line without changing surrounding text', async ({page}) => {
      await editor(page).fill('first\nreplace me\nlast');
      await editor(page).press('ControlOrMeta+Home');
      await editor(page).press('ArrowDown');
      await editor(page).press('Home');
      await editor(page).press('Shift+End');
      await page.keyboard.insertText('replacement');
      expect(await documentText(page)).toBe('first\nreplacement\nlast');
    });

    test('indent and unindent a multiline selection', async ({page}) => {
      await editor(page).fill('alpha\nbeta');
      await editor(page).press('ControlOrMeta+a');
      await editor(page).press('Tab');
      expect(await documentText(page)).toBe('  alpha\n  beta');
      await editor(page).press('Shift+Tab');
      expect(await documentText(page)).toBe('alpha\nbeta');
    });

    test('paste multiline Unicode text over a selection', async ({page}) => {
      await editor(page).fill('replace me');
      await editor(page).press('ControlOrMeta+a');
      // Exercise the editor paste handler without touching the macOS clipboard.
      await editor(page).evaluate(node => {
        const clipboardData = new DataTransfer();
        clipboardData.setData('text/plain','café 😀\n漢字\n');
        node.dispatchEvent(new ClipboardEvent('paste',{clipboardData,bubbles:true,cancelable:true}));
      });
      expect(await documentText(page)).toBe('café 😀\n漢字\n');
      await editor(page).press('ControlOrMeta+z');
      expect(await documentText(page)).toBe('replace me');
    });

    test('Backspace preserves neighboring text when deleting emoji and combining accents', async ({page}) => {
      for (const [suffix,remaining] of [['😀',''],['e\u0301','e']]) {
        await editor(page).fill('keep '+suffix);
        await editor(page).press('ControlOrMeta+End');
        await editor(page).press('Backspace');
        expect(await documentText(page)).toBe('keep '+remaining);
      }
    });

    test('invalid search expression is recoverable', async ({page}) => {
      await editor(page).fill('oak pine oak');
      await editor(page).press('ControlOrMeta+f');
      await page.getByRole('checkbox',{name:'regexp',exact:true}).check();
      await page.getByRole('textbox',{name:'Find',exact:true}).fill('[');
      await page.getByRole('textbox',{name:'Find',exact:true}).fill('oak');
      await page.getByRole('textbox',{name:'Replace',exact:true}).fill('elm');
      await page.getByRole('button',{name:'replace all',exact:true}).click();
      expect(await documentText(page)).toBe('elm pine elm');
      await page.getByRole('textbox',{name:'Find',exact:true}).press('Escape');
      await editor(page).press('Escape');
      await editor(page).press('Tab');
      await expect(editor(page)).not.toBeFocused();
    });
  });
}

// Run alone with npm run test:measure. Timings include browser automation and
// rendering. Typing measures beforeinput → two animation frames in the browser;
// the baseline excludes JaDE's HTTP save/recovery work.
for (const bytes of [50_000, 500_000, 4_500_000]) {
  test(`@measure open, type, and search ${bytes} bytes`, async ({page,appURL,baselineURL,workspace,browser},info) => {
    test.setTimeout(120_000);
    const line = 'Observation: the next run should confirm this result.\n';
    const contents = line.repeat(Math.floor(bytes/line.length))+'FINAL-PASSAGE';
    const hash = (text: string) => createHash('sha256').update(text).digest('hex');
    const results: {target:string; trial:number; bytes:number; openMs:number; typeMs:number; searchMs:number}[] = [];
    const paint = () => page.evaluate(()=>new Promise<void>(done=>requestAnimationFrame(()=>requestAnimationFrame(()=>done()))));
    for (let trial=0;trial<3;trial++) {
      // Alternate order to reduce cache/order bias. Each visit uses the same file.
      for (const target of trial%2 ? ['CodeMirror','JaDE'] : ['JaDE','CodeMirror']) {
        await writeFile(join(workspace,'notes.txt'),contents);
        const start = performance.now();
        await page.goto(target==='JaDE' ? appURL+'/?file=notes.txt' : baselineURL);
        await expect(editor(page)).toBeVisible();
        if(target==='JaDE') await expect(page.locator('body')).toHaveAttribute('data-drafts-ready','true');
        await paint();
        const openMs = performance.now()-start;
        await editor(page).press('ControlOrMeta+End');
        await editor(page).evaluate(node => {
          node.addEventListener('beforeinput',()=>{
            performance.mark('typing-start');
            requestAnimationFrame(()=>requestAnimationFrame(()=>performance.measure('typing-paint','typing-start')));
          },{once:true});
        });
        await page.keyboard.insertText('!'); await paint();
        const typeMs = await page.evaluate(()=>performance.getEntriesByName('typing-paint').at(-1)?.duration);
        expect(typeMs).toBeDefined();
        await editor(page).press('ControlOrMeta+f');
        const searching = performance.now();
        await page.getByRole('textbox',{name:'Find',exact:true}).pressSequentially('FINAL-PASSAGE');
        await page.getByRole('textbox',{name:'Find',exact:true}).press('Enter');
        await expect(page.locator('.cm-searchMatch-selected')).toHaveText('FINAL-PASSAGE');
        await paint();
        const searchMs = performance.now()-searching;
        await page.getByRole('textbox',{name:'Find',exact:true}).press('Escape');
        if(target==='JaDE') {
          await editor(page).press('ControlOrMeta+s');
          await expect(page.locator('#save-status')).toHaveText('Saved');
          expect(hash(await readFile(join(workspace,'notes.txt'),'utf8'))).toBe(hash(contents+'!'));
        }
        results.push({target,trial,bytes:contents.length,openMs,typeMs:typeMs!,searchMs});
      }
    }
    const output = info.outputPath('measurements.json');
    await writeFile(output,JSON.stringify({browser:info.project.name,browserVersion:browser.version(),cpu:cpus()[0].model,platform:process.platform,arch:process.arch,unit:"ms",results},null,2));
    await info.attach('measurements.json',{path:output,contentType:'application/json'});
  });
}
