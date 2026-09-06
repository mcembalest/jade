import {remote} from './remote.js';
const MAX_BYTES = 512 * 1024;
const MAX_FILES = 500;
const MAX_WORKSPACE_BYTES = 16 * 1024 * 1024;
const json = (value, status = 200) => Response.json(value, {status, headers: {'Cache-Control': 'no-store'}});
export const validPath = p => typeof p === 'string' && p.length <= 240 &&
  p.split('/').every(s => /^[a-zA-Z0-9 _().-]+$/.test(s) && !s.startsWith('.') && s.trim() === s) && /\.(md|txt)$/i.test(p);
const validID = s => typeof s === 'string' && /^[a-zA-Z0-9-]{1,80}$/.test(s);
const selectFiles = `SELECT f.*, COALESCE((SELECT json_group_object(deviceId, revision)
  FROM acknowledgements a WHERE a.path=f.path), '{}') AS acks FROM files f`;
const decode = row => row ? {...row, acks: JSON.parse(row.acks || '{}')} : null;
async function current(db, path) { return decode(await db.prepare(selectFiles + ' WHERE f.path=?').bind(path).first()); }
export default {
  async fetch(request, env) {
    const url = new URL(request.url);
    if (url.pathname.startsWith('/v1/remote/')) {
      try { return await remote(request,env); } catch { return json({error:'Remote connection unavailable'},503); }
    }
    if (url.pathname === '/health' && request.method === 'GET') return json({service: 'JaDE Sync', protocol: 1});
    if (!env.SYNC_TOKEN || request.headers.get('Authorization') !== `Bearer ${env.SYNC_TOKEN}`) return json({error: 'Pairing key required'}, 401);
    try {
      if (url.pathname === '/v1/files' && request.method === 'GET') {
        const {results} = await env.DB.prepare(selectFiles + ' ORDER BY f.path').all();
        return json({protocol: 1, files: results.map(decode)});
      }
      if (!['/v1/files', '/v1/ack'].includes(url.pathname) || request.method !== 'POST') return json({error: 'Not found'}, 404);
      const raw = await request.text();
      if (new TextEncoder().encode(raw).length > MAX_BYTES * 6 + 2048) return json({error: 'Request too large'}, 413);
      const b = JSON.parse(raw);
      if (!validPath(b.path) || !['mac', 'iphone'].includes(b.deviceId)) return json({error: 'Invalid path or device'}, 400);
      if (url.pathname === '/v1/ack') {
        if (!validID(b.revision)) return json({error: 'Invalid revision'}, 400);
        await env.DB.prepare(`INSERT INTO acknowledgements(path, deviceId, revision)
          SELECT ?, ?, ? WHERE EXISTS(SELECT 1 FROM files WHERE path=? AND revision=?)
          ON CONFLICT(path, deviceId) DO UPDATE SET revision=excluded.revision`).bind(b.path,b.deviceId,b.revision,b.path,b.revision).run();
        return json({file: await current(env.DB, b.path)});
      }
      if (typeof b.content !== 'string' || new TextEncoder().encode(b.content).length > MAX_BYTES ||
          !validID(b.mutationId) || !(b.baseRevision === '' || validID(b.baseRevision))) return json({error: 'Invalid edit (512 KB maximum)'}, 400);
      const previous = await env.DB.prepare('SELECT * FROM revisions WHERE revision=?').bind(b.mutationId).first();
      if (previous) {
        if (previous.path !== b.path || previous.content !== b.content || previous.writer !== b.deviceId || previous.baseRevision !== b.baseRevision)
          return json({error: 'Mutation ID already used'}, 409);
        return json({acceptedRevision: b.mutationId, file: await current(env.DB, b.path)});
      }
      // The WHERE condition is the compare-and-swap. The trigger and this insert
      // run atomically; simultaneous writers cannot both replace the same base.
      const inserted = await env.DB.prepare(`INSERT OR IGNORE INTO revisions(revision,path,content,baseRevision,writer,updatedAt)
        SELECT ?,?,?,?,?,? WHERE COALESCE((SELECT revision FROM files WHERE path=?),'')=?
        AND (EXISTS(SELECT 1 FROM files WHERE path=?) OR (SELECT COUNT(*) FROM files) < ?)
        AND COALESCE((SELECT SUM(length(CAST(content AS BLOB))) FROM files WHERE path<>?),0) + ? <= ?`)
        .bind(b.mutationId,b.path,b.content,b.baseRevision,b.deviceId,new Date().toISOString(),b.path,b.baseRevision,
          b.path,MAX_FILES,b.path,new TextEncoder().encode(b.content).length,MAX_WORKSPACE_BYTES).run();
      if (!inserted.meta.changes) {
        // Another retry may have committed this exact operation concurrently.
        const accepted = await env.DB.prepare('SELECT * FROM revisions WHERE revision=?').bind(b.mutationId).first();
        if (accepted && accepted.path === b.path && accepted.content === b.content && accepted.writer === b.deviceId && accepted.baseRevision === b.baseRevision)
          return json({acceptedRevision:b.mutationId,file:await current(env.DB,b.path)});
        const file = await current(env.DB, b.path);
        if ((file?.revision || '') !== b.baseRevision) return json({error:'Conflict: both versions preserved',file},409);
        return json({error:'Workspace limit reached (500 files / 16 MB)'},413);
      }
      return json({acceptedRevision: b.mutationId, file: await current(env.DB, b.path)});
    } catch (e) {
      if (e instanceof SyntaxError) return json({error:'Invalid JSON'},400);
      return json({error:'Sync storage unavailable; your device should keep pending edits'},503);
    }
  }
};
