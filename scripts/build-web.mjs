import { build } from 'esbuild';
import { readFile, writeFile } from 'node:fs/promises';
await build({entryPoints:['engine/web/editor.js'], bundle:true, format:'iife', target:'es2022', minify:true, legalComments:'eof', outfile:'engine/web/editor.bundle.js'});
const lock = JSON.parse(await readFile('package-lock.json','utf8'));
const notices = ['Third-party licenses for the bundled JaDE editor.\n'];
for (const [directory,pkg] of Object.entries(lock.packages)) {
  if (!directory || pkg.dev) continue;
  const license = await readFile(directory+'/LICENSE','utf8');
  notices.push(directory.replace('node_modules/','')+' '+pkg.version+'\n\n'+license);
}
await writeFile('engine/web/THIRD_PARTY_NOTICES.txt', notices.join('\n\n---\n\n'));
