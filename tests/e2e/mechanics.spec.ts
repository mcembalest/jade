import { test, expect, editor, documentText } from './fixtures';

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
  });
}
