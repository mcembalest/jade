import { build } from 'esbuild';
import { readFile, writeFile, mkdir } from 'node:fs/promises';
await mkdir('engine/web/dist', {recursive:true});
await build({entryPoints:['engine/web/editor.ts'], bundle:true, format:'iife', target:'es2022', minify:true, legalComments:'eof', outfile:'engine/web/dist/editor.bundle.js'});
const lock = JSON.parse(await readFile('package-lock.json','utf8'));
const notices = ['Third-party licenses for the bundled JaDE editor.\n'];
for (const [directory,pkg] of Object.entries(lock.packages)) {
  if (!directory || pkg.dev) continue;
  const license = await readFile(directory+'/LICENSE','utf8');
  notices.push(directory.replace('node_modules/','')+' '+pkg.version+'\n\n'+license);
}
await writeFile('engine/web/dist/THIRD_PARTY_NOTICES.txt', notices.join('\n\n---\n\n'));
