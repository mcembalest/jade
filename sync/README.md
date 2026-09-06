# Personal Mac ↔ iPhone sync

This page covers the **Notes** tab. The separate **Mac files** tab now edits enabled Mac folders, including `~/Documents/first`; see [remote editing](../remote/README.md).

JaDE saves locally first. A Cloudflare Worker and D1 database relay immutable text revisions between one Mac and one iPhone. The editor does not store documents in a proprietary format on the Mac.

## Current installation

- Workspace: `~/JaDE Mobile`
- Mac editor: <http://127.0.0.1:7339>
- Cloudflare Worker: `jade-personal-sync`
- D1 database: `jade-personal-sync`
- iPhone project: `mobile/JaDE.xcodeproj` (open in Xcode)
- Pairing key: `~/JaDE Mobile/.jade-sync/config.json`, readable only by your account
- Mac service: `~/Library/LaunchAgents/com.mcembalest.jade.sync.plist`
- Mac binary and logs: `~/Library/Application Support/JaDE/`

The Mac service runs automatically at login, including while the browser is closed. It cannot receive edits while the Mac is asleep. The phone synchronizes while JaDE is open, when reopened, or when you tap **Sync now**. Background delivery is not promised.

| Status | Meaning |
| --- | --- |
| Saved on this device · pending sync | The edit is stored locally; no server acknowledgement yet |
| Uploaded · other device pending | Cloudflare accepted the revision; the other device has not acknowledged it |
| Synced with Mac/iPhone | The other device acknowledged this exact revision |
| Conflict · both versions kept | Both devices edited a different version; local text remains untouched |

**Keep both versions** installs the incoming version at the original filename and saves local text as a separate conflict copy, which subsequently syncs as another note. Never silently choose the last writer.

## Install on your iPhone

1. Open `mobile/JaDE.xcodeproj` in Xcode.
2. Add your Apple account in **Xcode → Settings → Accounts**.
3. Connect and unlock the iPhone; accept **Trust This Computer**.
4. Enable **Settings → Privacy & Security → Developer Mode** on the phone when requested. This may require a restart.
5. In Xcode, select the **JaDE** target → **Signing & Capabilities**, leave automatic signing enabled, and select your team.
6. Select your connected iPhone as the destination, then **Product → Run** (⌘R).
   If iOS blocks opening the installed app, go to **Settings → General → VPN & Device Management** on your iPhone, select your developer account, and **Trust** it. Follow any restart confirmation.
7. Once JaDE launches, scan the private pairing QR on your Mac with the iPhone Camera and choose **Open in JaDE**. Alternatively enter the endpoint and pairing key in JaDE's sync settings.
8. Open `Welcome.md` and wait for **Synced with Mac**. Disconnect the cable: normal sync uses the network.

An unsigned `.app` is not installable on a physical iPhone. Free Personal Team provisioning expires after seven days; paid membership has different provisioning terms and still requires occasional renewal. This is a private development install, not an App Store release.

Regenerate the private pairing QR locally:

```sh
swift sync/make-pairing.swift "$HOME/JaDE Mobile/.jade-sync/config.json" "$HOME/Library/Application Support/JaDE/Pair iPhone.png"
```

The QR contains the workspace access key. It is generated on your Mac without sending the key to a QR-generation service. Do not publish it. The iPhone stores pairing in Keychain.

## Scope and limits

- One personal workspace, one Mac, one iPhone. The two role IDs are `mac` and `iphone`; a simulator used for testing occupies the iPhone role.
- `.md` and `.txt`, UTF-8, up to 512 KB each; 500 documents / 16 MB of current document content.
- Paths currently use ASCII letters/numbers, spaces, `_`, `-`, parentheses and periods. Hidden paths and symlinks are excluded.
- iPhone editing is a native plain-text editor. Desktop preview, code-language features, terminals and the companion are not included on mobile.
- No automatic deletion or rename propagation. A Mac deletion is shown as local-only; the server retains its copy. Renames appear as new files and leave the old remote file intact.
- No attachments, collaborative merge, or end-to-end encryption. HTTPS protects transport; your Cloudflare account and the service can read documents.
- A history of accepted revisions is retained in D1. This consumes storage over time and has no automatic retention policy yet. Monitor Cloudflare usage; exceeding a limit must leave edits pending locally.
- Preserve `.jade-sync/state.json` with your workspace. If corrupt, JaDE refuses to replace it automatically. Do not delete sync metadata as a troubleshooting shortcut.

## Backup and recovery

An independent database export preserves the latest files, accepted revision history, and receipts. From `sync/cloudflare`:

```sh
mkdir -p "$HOME/Library/Application Support/JaDE/backups"
umask 077
npx wrangler d1 export jade-personal-sync --remote --output "$HOME/Library/Application Support/JaDE/backups/jade-sync.sql"
```

Use a new output filename for each backup. Keep a separate copy of local workspace files and the pairing configuration. Pending phone edits are not in the server backup until uploaded; export an individual note with its share button if needed.

To recover an old note, inspect the exported `revisions` table using SQLite and copy the desired text into a **new** note first. Full database restoration should be rehearsed in a new database; avoid replacing the live database while devices are syncing. There is no in-app history browser yet.

To pause the Mac service:

```sh
launchctl bootout "gui/$(id -u)/com.mcembalest.jade.sync"
```

To resume:

```sh
launchctl bootstrap "gui/$(id -u)" "$HOME/Library/LaunchAgents/com.mcembalest.jade.sync.plist"
```

To update the Mac service after building the browser assets, run `python3 sync/install-mac.py` from the repository root. Removing its LaunchAgent prevents automatic startup; retain your notes and pairing files.

## Development and portability

`cloudflare/worker.js` implements protocol v1:

- `GET /v1/files`: current documents and device acknowledgements.
- `POST /v1/files`: `{path, content, baseRevision, mutationId, deviceId}`. Atomic compare-and-swap; immutable mutation IDs make retries safe even after later edits. Stale bases return 409 plus the current remote document.
- `POST /v1/ack`: acknowledge a revision only after the local file and metadata are saved.
- Authentication: one workspace-scoped bearer key; all document routes require it.

The D1 schema uses ordinary SQLite tables and a trigger. Hosting migration requires an implementation of this protocol and an export/import of **all** tables. It does not require changing the document editor. Credentials and endpoint settings must also be updated on both devices. Replacing the provider is an adapter/migration project, not an arbitrary URL swap.

Tests:

```sh
cd sync/cloudflare
npm ci
npm test
node test-server.mjs
```

With that local test server running, from the repository root:

```sh
JADE_TEST_SYNC_URL=http://127.0.0.1:8799 go test -race ./engine -run 'TestSync|TestCloudflare' -v
swiftc -parse-as-library mobile/JaDE/SyncStore.swift mobile/tests/SyncStoreTests.swift -o .tmp/swift-sync-tests
JADE_TEST_SYNC_URL=http://127.0.0.1:8799 .tmp/swift-sync-tests
```

The Xcode scheme includes a simulator UI test for typing, app termination, reopening, and visible pending status. That test uses an isolated local test workspace and disables pairing in Debug builds. The live Cloudflare pairing test runs only with `LIVE_SYNC_TEST` in `SWIFT_ACTIVE_COMPILATION_CONDITIONS`, on a deliberately paired test simulator.

## Validation on September 5, 2026

- iPhone Debug and Release builds succeeded (unsigned; physical-device signing remains user-specific).
- Native simulator UI test passed: type offline, terminate, reopen, verify text and pending status.
- Live pairing UI test passed against the deployed Cloudflare service; a Mac-created note arrived with its Mac receipt. The simulator-created note then arrived as an ordinary file on the Mac.
- Swift and Go integration tests passed against the local Cloudflare runtime: conflict preservation, durable staging, restart, and replay after a lost upload response. Swift also checks corrupt-store protection; Go checks path/symlink boundaries and local-only deletion behavior.
- Worker tests passed: authorization, concurrent compare-and-swap, historical retry, invalid paths, size limits, and revision-specific acknowledgements.
- Go race tests passed; 78 existing desktop regression checks and 4 new sync-interface checks passed in Chromium/WebKit.
- A remote D1 export was restored to a separate local SQLite database and passed `integrity_check`.
- Physical iPhone installation and launch verified on an iPhone 17 with iOS 26.6.1, using the user's Personal Team. A newly created Mac note was downloaded into the physical phone's local workspace and acknowledged by both devices. The user's `Firsttest.md` was then written by the iPhone, stored in Cloudflare, received as an identical Mac file, and acknowledged at the same revision by both devices. The initial signing profile expires September 12, 2026; renew by reinstalling over the existing app.
