# Cloud projects protocol (additive v2 feature)

Cloud projects are opt-in persistent replicas of specific Mac folders. The existing `/v1/files` Notes protocol and `/v1/remote/*` direct-edit relay are unchanged. Projects use separate `projects`, `project_files` and `project_revisions` tables.

## Delivery contract

1. Typing persists a phone draft only.
2. **Submit to Cloudflare** persists an immutable upload request on the phone before network access. Failed delivery keeps that request for retry when Cloud projects is refreshed/opened.
3. Cloudflare accepts the revision durably, even if the Mac is off. There is no request expiry for project edits.
4. The Mac compares its current file with its previous synchronized version. If unchanged, it backs up and applies the cloud revision, persists local sync metadata, then acknowledges that exact revision.
5. The phone displays Cloudflare acceptance separately from Mac application. A later unsubmitted phone draft remains distinct from an older queued snapshot.

Mac-file access alone never enables cloud uploading. Enable a specific folder in the Mac helper's **Cloud projects** menu. Pausing disables future submissions and Mac replication after the setting reaches the service; existing cloud copies/history remain readable. Previously cached phone drafts also remain. This is pause, not data erasure or credential revocation.

## HTTP API

All routes require the existing phone pairing key or the separate Mac agent key. Only the agent may register/pause a project or acknowledge Mac delivery. The writer is derived from authentication, not request JSON.

- `GET /v1/projects`: project list, enabled status, last Mac check and summary.
- `POST /v1/projects/:id` (agent): `{name,enabled,status}`.
- `GET /v1/projects/:id/files`: metadata only, including current and Mac-acknowledged revisions.
- `GET /v1/projects/:id/file?path=...`: current text and metadata.
- `POST /v1/projects/:id/file`: `{path,content,baseRevision,mutationId}`. Atomic compare-and-swap; immutable IDs allow retry after a lost response, even if a newer revision has since been accepted.
- `POST /v1/projects/:id/ack` (agent): `{path,revision,applied,issue}`. Stale receipts do not advance the current file's Mac state.
- `GET /v1/projects/:id/history?path=...`: latest 50 accepted revision descriptors.
- `GET /v1/projects/:id/revision?path=...&revision=...`: historical contents for viewing/export.

## Initial scope

Up to 16 projects, 2,000 text files / 32 MB per project, 256 KB per file. UTF-8 paths are supported. Hidden paths, generated directories, symlinks, binary files and key/certificate files are excluded by the Mac publisher. Git repos additionally respect untracked-file ignore rules; tracked files remain included. Secrets in otherwise eligible tracked/text files are not automatically detected. Enabling cloud storage is permission to upload eligible contents; Cloudflare can read them.

Project browsing with an offline Mac uses Cloudflare's last uploaded snapshot. Use **Download missing files for offline use** to cache a whole project. Each file is persisted before proceeding. Interrupted downloads resume by skipping saved files; existing copies and drafts are never replaced. Without network, downloaded files remain readable/editable. A download is an explicit action, not a background iOS guarantee. Refresh fetches project metadata and retries submitted requests; it never automatically replaces a phone draft or uploads unsubmitted typing. Fetch is explicit for newer contents.

Local Mac deletion is marked for attention and does not recreate the file or erase the cloud copy. Conflicting Mac and cloud contents are retained separately with a visible issue; Mac-versus-cloud disk conflicts still require deliberate resolution through existing Mac-file editing or a separate copy. Phone-versus-cloud conflicts have a three-version comparison screen: original base, phone draft, and competing cloud version. Choose either text or combine manually; resolving atomically preserves the original phone draft under `Recovered drafts/`, stages the result against the reviewed cloud revision, and leaves submission separate. Another concurrent cloud edit still produces a conflict, never a forced overwrite. It does not automatically merge, rename, delete, commit, run code or expose arbitrary Mac directories.

The bridge uses a shared in-process write lock for direct and cloud writes. Other Mac applications do not share the lock; the existing final-content-check/atomic-replacement approach still has a narrow external-editor race. Dedicated repo worktrees remain appropriate for separate work.

## Deployment and recovery

Back up the existing database with `python3 remote/backup-cloud.py`. The script exports through Wrangler and validates the SQL by restoring it into a separate SQLite database. Backups contain private contents and are stored with mode 600 under the user's JaDE support directory.

Apply `projects.sql`, then deploy `worker.js`. It imports `projects.js`; existing tables are untouched. Install the updated Mac bridge and menu app together, then the phone build. No existing folder is automatically opted in.

Cloudflare runs a daily backup at **07:17 UTC**, using its native Cron Trigger and private R2 binding. A D1 transaction captures the current manifest and immutable history boundaries. Revision history is copied in bounded pages; a completion manifest is published only after every page is stored. SHA-256 checksums and restored current revisions are verified by `remote/restore-cloud-backup.py`. Failed jobs preserve the last success and expose the failure through `/v1/backup`, shown in the phone UI.

Native R2 lifecycle rules retain completion manifests for **30 days** and data for **31 days** (the extra day prevents parts expiring before a later manifest). Partial backups have no completion marker and their data expires automatically. Configure `backup-data-31-days` on `daily/`, and `backup-manifests-30-days` on `daily/manifests/`. These bucket settings are separate from `wrangler.jsonc`; recreate them when moving accounts. The bucket is private and has no public URL enabled.

To recover without overwriting live files:

```sh
python3 remote/restore-cloud-backup.py daily/manifests/COMPLETED-BACKUP.json /new/path/recovery.sqlite
```

Find the manifest key in the private R2 bucket or D1 `backup_status`. The tool downloads through your local Wrangler login, verifies each part, reconstructs Notes/projects/history/receipts, and publishes a new SQLite recovery file only after integrity and revision checks pass. It never restores into the live service. The existing `backup-cloud.py` provides an independent full SQL export on your Mac.

Backups cover accepted cloud contents and history. Unsubmitted phone drafts and Mac-only files are not cloud backups. Transient remote requests, credentials and local folder grants are deliberately outside the recovery archive. D1 Time Travel provides Cloudflare's additional native database recovery. R2 is in the same Cloudflare account, so the local SQL export remains useful as a separate copy.

Accepted revision history is retained indefinitely to preserve historical retry IDs and recovery; the UI shows the latest 50 versions. Local pre-write backups are also retained. Automatic expiration applies to **daily backup archives**, not current documents or revision history. Full-history backup costs grow with history; this is a small personal-workspace design, not a general Drive replacement. Backup failures remain visible; expanding scale calls for revisiting that policy.

## Verified September 6, 2026

- Six Worker tests passed, including unchanged Notes/remote behavior, project isolation, separate agent credentials, atomic concurrent writes, history and paused-project writes.
- Nine Mac tests passed, including stopped-agent delivery, local deletion, independent edits, lost replies, nested files, symlinks and corrupt state.
- Swift model tests passed for explicit submissions, local queue persistence, restart after cloud acceptance, preservation of newer unsubmitted typing, exact Mac receipts, conflicts and history. The existing Notes model regression also passed.
- Signed simulator UI test passed: fetch a live Cloudflare code file, edit, terminate/reopen, recover the draft, submit while the project agent is stopped, and display Cloudflare acceptance with Mac pending.
- The live edit remained pending for 118 seconds, beyond the old direct-relay request lifetime. Resuming the isolated project agent applied identical contents on Mac and acknowledged the matching revision.
- A full D1 export was restored into a separate SQLite database and passed integrity validation.
- Cloudflare deployment and Mac helper update completed. Existing `Documents/first` direct access was rechecked successfully. The temporary cloud test project was removed; no real project was automatically opted in.
- Signed iPhone version 0.2.0 (build 2) installed successfully. Automatic launch was blocked by the phone lock screen; normal manual launch remains available.

For existing Mac installations, use `python3 remote/update.py` to build and back up the helper before replacing it, retain configured folder access, and install the paired bridge/cloud modules together. The separate Notes service is not restarted.


## Reliability design and September 6 follow-up

This is a custom client sync protocol on native Cloudflare storage, not a Cloudflare-built file-sync product. The design uses ordinary durable outboxes, immutable revision IDs, optimistic concurrency, atomic SQL triggers, and exact device receipts. D1 holds small full-text snapshots and metadata in one transaction. Keeping them together avoids cross-service commit machinery for this bounded workload. R2 is the backup store; queues, Durable Objects, CRDTs and block-level transfer are unnecessary for this version. Client HTTP boundaries allow later storage changes, but replacement providers must satisfy the same concurrency/durability tests.

Additional validation: seven Worker tests including concurrent writes during backup, failed R2 writes, checksum rejection and actual SQLite restoration; Swift tests for interrupted/resumed bulk downloads and conflicts that change again during resolution; nine Mac regression tests; a signed simulator UI test covering bulk download, conflict comparison/resolution and app restart. The first production R2 backup was downloaded and restored into an isolated SQLite file. Phone 0.2.1 build 3 installed successfully; the lock screen prevented automatic launch.
