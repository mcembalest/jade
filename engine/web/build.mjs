import { build } from 'esbuild';
import { readFile, writeFile, mkdir } from 'node:fs/promises';
await mkdir('dist', {recursive:true});
await build({entryPoints:['editor.ts'], bundle:true, format:'iife', target:'es2022', minify:true, legalComments:'eof', outfile:'dist/editor.bundle.js'});
const lock = JSON.parse(await readFile('package-lock.json','utf8'));
const notices = ['Third-party licenses for the bundled JaDE editor.\n'];
for (const [directory,pkg] of Object.entries(lock.packages)) {
  if (!directory || pkg.dev) continue;
  const license = await readFile(directory+'/LICENSE','utf8');
  notices.push(directory.replace('node_modules/','')+' '+pkg.version+'\n\n'+license);
}
await writeFile('dist/THIRD_PARTY_NOTICES.txt', notices.join('\n\n---\n\n'));
