# Cloud sync repair handoff

Goal: make JADE's Cloudflare-backed sync with local staging a reliable primary path between Mac and iPhone, independent of Obsidian Sync. Preserve every edit, keep explicit conflicts, and distinguish local save, cloud acceptance, and device delivery.

Start from branch codex/mobile-sanjana, including commits bee82c8 and 9768593. Do not revert the configurable companion/research foundation. Cloud research is separately deployed and its Codex login and restricted runtime have been verified connected; it is not the source of the file-sync problem.

## Observed September 16, 2026

- Dedicated Notes workspace: eight files matched cloud state and Mac/iPhone acknowledgements during the audit. This is separate from the writing vault.
- Writing vault Documents/first was permitted for direct Mac-file access but had NOT opted into persistent cloud projects. The cloud projects list was empty. Direct Mac files and persistent Cloud projects are different phone entry points and this distinction confused the user.
- Opening Obsidian earlier downloaded queued phone edits and reported fully synced. A checked note matched local and remote reads afterward. This demonstrated an Obsidian dependency, not a working replacement sync path.
- Later, opening/listing Documents/first hung in both new and previous JADE builds and a direct Python directory listing. Metadata reads succeeded. macOS Files & Folders showed jade-sync Documents access enabled. Root cause remains unconfirmed; do not label this a proven JADE algorithm bug or claim to fix the actual machine from a cloud sandbox.
- The desktop now shows project cloud opt-in and stale helper status, separate from Notes receipts.
- Existing cloud-project implementation excludes hidden/generated files, binaries, attachments and oversized text; no rename/deletion propagation. Assess these limits explicitly against reliable vault syncing.
- The user has not yet answered the explicit choice to upload the entire eligible writing vault into Cloudflare. Use synthetic fixtures for development; do not activate real-folder uploads or alter Obsidian settings.

## Code and validation

Inspect remote/cloud.py, remote/bridge.py, remote/manage.py and the native Mac connection helper; sync/cloudflare/projects.js and PROJECTS.md; mobile/JaDE/CloudStore.swift and related cloud-project UI; engine/sync.go and project_sync_status.go. Trace actual disk roots and installed/running versions before attributing stale data to cloud consistency.

Reproduce offline editing, simultaneous Mac/phone edits, lost acknowledgements, process termination, helper restart, external editor writes, stale base versions, revocation, folder-read hangs and skipped files. Add meaningful regression tests for discovered failures. Ensure folder failures do not block other projects or indefinitely leave UI requests pending. Preserve original files and local drafts on every uncertain outcome.

Checks: go test -race ./...; python3 -m unittest discover -s remote -p 'test_*.py'; npm ci && npm test in sync/cloudflare; npm ci && npm run build in engine/web, then Playwright tests excluding @measure. iPhone signing, physical-device testing and macOS native UI require later local validation.

Deliver a reviewable branch/PR with code fixes, test evidence, remaining limits and a precise Mac/iPhone verification checklist. Cloud tasks do not have the user's local vault, credentials, installed processes, or full conversation. Do not invent successful live sync or a confirmed root cause. Do not deploy or migrate production personal data as part of this repair task.
