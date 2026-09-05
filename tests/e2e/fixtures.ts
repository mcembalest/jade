import { test as base, expect, type Page } from '@playwright/test';
import { spawn } from 'node:child_process';
import { createServer } from 'node:http';
import { cp, mkdtemp, mkdir, readFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { once } from 'node:events';

export const test = base.extend<{ workspace: string; appURL: string; baselineURL: string }>({
  workspace: async ({}, use) => {
    const directory = await mkdtemp(join(tmpdir(), 'jade-e2e-'));
    const workspace = join(directory, 'workspace');
    await cp('tests/fixtures/workspace', workspace, { recursive: true });
    await mkdir(join(directory, 'home'));
    try { await use(workspace); } finally { await rm(directory, { recursive: true, force: true }); }
  },
  appURL: async ({ workspace }, use, testInfo) => {
    const child = spawn(resolve('.tmp/e2e/jade'), ['--no-open', workspace], {
      env: { ...process.env, HOME: join(dirname(workspace), 'home'), JADE_TERMINAL: '' },
      stdio: ['ignore', 'pipe', 'pipe'],
    });
    let log = '';
    child.stdout.on('data', data => { log += data; });
    child.stderr.on('data', data => { log += data; });
    try {
      const url = await new Promise<string>((accept, reject) => {
        const timer = setTimeout(() => reject(new Error('No engine readiness: ' + log)), 10_000);
        child.once('error', error => { clearTimeout(timer); reject(error); });
        child.once('exit', code => { clearTimeout(timer); reject(new Error('Engine exited: ' + code + '\n' + log)); });
        child.stdout.on('data', () => {
          const url = /JaDE: (http:\/\/127\.0\.0\.1:\d+)/.exec(log)?.[1];
          if (url) { clearTimeout(timer); accept(url); }
        });
      });
      await use(url);
    } finally {
      if (child.exitCode === null && child.signalCode === null) {
        const exited = once(child, 'exit');
        child.kill('SIGTERM');
        const force = setTimeout(() => child.kill('SIGKILL'), 6_000);
        await exited;
        clearTimeout(force);
      }
      if (testInfo.status !== testInfo.expectedStatus) {
        await testInfo.attach('engine.log', { body: log, contentType: 'text/plain' });
      }
    }
  },
  baselineURL: async ({}, use) => {
    const bundle = await readFile('.tmp/e2e/baseline.js');
    const server = createServer((request, response) => {
      response.setHeader('Content-Type', request.url === '/baseline.js' ? 'text/javascript' : 'text/html');
      response.end(request.url === '/baseline.js' ? bundle : '<!doctype html><title>CodeMirror baseline</title><div id="editor"></div><script src="/baseline.js"></script>');
    });
    server.listen(0, '127.0.0.1');
    await once(server, 'listening');
    const address = server.address();
    if (!address || typeof address === 'string') throw new Error('No baseline address');
    try { await use(`http://127.0.0.1:${address.port}`); }
    finally { server.closeAllConnections(); await new Promise<void>(done => server.close(() => done())); }
  },
  page: async ({ page }, use, testInfo) => {
    const errors: string[] = [];
    page.on('pageerror', error => errors.push(error.message));
    await use(page);
    if (errors.length) await testInfo.attach('browser-errors.json', { body: JSON.stringify(errors), contentType: 'application/json' });
    expect(errors).toEqual([]);
  },
});

export { expect };
export const editor = (page: Page) => page.getByRole('textbox', { name: 'Editor', exact: true });
export const documentText = async (page: Page) => (await page.locator('.cm-line').allTextContents()).join('\n');
export const fileIs = (page: Page, name: string) => expect(page.locator('#file-name')).toHaveText(name);
export const saved = (page: Page) => expect(page.locator('#save-status')).toHaveText('Saved');
