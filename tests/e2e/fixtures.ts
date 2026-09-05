import { test as base, expect, type Page } from '@playwright/test';
import { spawn } from 'node:child_process';
import { createServer } from 'node:http';
import { cp, mkdtemp, mkdir, readFile, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { once } from 'node:events';

type App = { url: string; restart: (signal?: NodeJS.Signals) => Promise<string> };

export const test = base.extend<{ workspace: string; app: App; appURL: string; baselineURL: string; expectedPageErrors: string[] }>({
  workspace: async ({}, use) => {
    const directory = await mkdtemp(join(tmpdir(), 'jade-e2e-'));
    const workspace = join(directory, 'workspace');
    await cp('tests/fixtures/workspace', workspace, { recursive: true, verbatimSymlinks: true });
    await mkdir(join(directory, 'home'));
    try { await use(workspace); } finally { await rm(directory, { recursive: true, force: true }); }
  },
  app: async ({ workspace }, use, testInfo) => {
    let child: ReturnType<typeof spawn> | undefined;
    let log = '';
    const stop = async (signal: NodeJS.Signals = 'SIGTERM') => {
      if (!child || child.exitCode !== null || child.signalCode !== null) return;
      const exited = once(child, 'exit');
      child.kill(signal);
      const force = setTimeout(() => child?.kill('SIGKILL'), 6_000);
      await exited;
      clearTimeout(force);
    };
    const start = async () => {
      child = spawn(resolve('.tmp/e2e/jade'), ['--no-open', workspace], {
        env: { ...process.env, HOME: join(dirname(workspace), 'home'),
          XDG_CONFIG_HOME: join(dirname(workspace), 'home', '.config'), JADE_TERMINAL: '' },
        stdio: ['ignore', 'pipe', 'pipe'],
      });
      let startup = '';
      child.stdout!.on('data', data => { startup += data; log += data; });
      child.stderr!.on('data', data => { log += data; });
      return await new Promise<string>((accept, reject) => {
        const timer = setTimeout(() => reject(new Error('No engine readiness: ' + log)), 10_000);
        child!.once('error', error => { clearTimeout(timer); reject(error); });
        child!.once('exit', code => { clearTimeout(timer); reject(new Error('Engine exited: ' + code + '\n' + log)); });
        child!.stdout!.on('data', () => {
          const url = /JaDE: (http:\/\/127\.0\.0\.1:\d+)/.exec(startup)?.[1];
          if (url) { clearTimeout(timer); accept(url); }
        });
      });
    };
    try {
      const app: App = { url: await start(), restart: async signal => {
        await stop(signal);
        app.url = await start();
        return app.url;
      } };
      await use(app);
    } finally {
      await stop();
      if (testInfo.status !== testInfo.expectedStatus) {
        await testInfo.attach('engine.log', { body: log, contentType: 'text/plain' });
      }
    }
  },
  appURL: async ({ app }, use) => { await use(app.url); },
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
  expectedPageErrors: async ({}, use) => { await use([]); },
  page: async ({ page, expectedPageErrors }, use, testInfo) => {
    const errors: string[] = [];
    page.on('pageerror', error => errors.push(error.message));
    await use(page);
    if (errors.length) await testInfo.attach('browser-errors.json', { body: JSON.stringify(errors), contentType: 'application/json' });
    expect(errors).toEqual(expectedPageErrors);
  },
});

export { expect };
export const editor = (page: Page) => page.getByRole('textbox', { name: 'Editor', exact: true });
export const documentText = async (page: Page) => (await page.locator('.cm-line').allTextContents()).join('\n');
export const fileIs = (page: Page, name: string) => expect(page.locator('#file-name')).toHaveText(name);
export const saved = (page: Page) => expect(page.locator('#save-status')).toHaveText('Saved');
