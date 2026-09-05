import { execFileSync } from 'node:child_process';
import { mkdirSync } from 'node:fs';
import { buildSync } from 'esbuild';

export default function setup() {
  mkdirSync('.tmp/e2e', { recursive: true });
  execFileSync('go', ['build', '-o', '.tmp/e2e/jade', './cmd/jade'], { stdio: 'inherit' });
  buildSync({ entryPoints: ['tests/baseline.ts'], bundle: true, format: 'iife', outfile: '.tmp/e2e/baseline.js' });
}
