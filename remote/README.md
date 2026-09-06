# Remote writing and code editing

JaDE's **Mac files** tab edits UTF-8 text/source files in explicitly enabled Mac folders. It is independent of the Notes sync module. Your Mac must remain awake and online. No router ports, inbound server, or additional phone pairing is required.

## Installed configuration

- Open `~/Applications/JaDE Mac Connection.app` (or the **JaDE** menu bar item) to select a writing folder or repo working copy. Confirm **Allow this folder**. This normal macOS selection grants access to Documents/Desktop folders without Full Disk Access.
- `~/Documents/first` is enabled and a physical iPhone edit of an existing vault note has been confirmed on the Mac. Hidden Obsidian configuration is excluded; this is text editing, not an implementation of Obsidian plugins or attachment previews.
- In JaDE on iPhone, select **Mac files → Connect / refresh**, then the folder and file.
- Type your changes; each edit is saved in a local phone draft. Tap **Save to Mac** to apply it. **Saved on Mac** means the bridge has acknowledged the file write.
- Existing drafts can be opened from **Saved phone drafts**, including without connectivity. Saving to the Mac requires connectivity.
- Reload and export are available in the editor's menu. **Save as a new file** preserves a conflicting draft separately.
- The folder selector grants access to that actual working copy. To isolate repo work, select a separate Git worktree. This release does not automatically make a worktree, execute commands, or perform Git operations.

Folders can be listed or revoked locally:

```sh
python3 remote/manage.py --list
python3 remote/manage.py --remove FOLDER_ID
```

Permission changes apply on the next request; no restart is needed. There are no remotely callable folder-grant or shell-command endpoints.

## Machinery

The existing Cloudflare Worker relays request/response messages through a separate D1 table. The Mac makes an outbound authenticated poll every three seconds. The phone's existing pairing key may submit requests; a separate Mac-only secret is required to receive jobs or report results. This uses the account's existing Worker deployment, rather than a Cloudflare Tunnel.

The phone waits for explicit Mac acknowledgement. A network timeout can happen after the write succeeds, so the draft remains and the user is told that delivery is uncertain. Retrying identical content is safe; a different intervening Mac version causes a conflict. Requests expire after 60 seconds. The relay removes rows older than one hour when the bridge polls; it does not retain repo revision history. Contents travel over HTTPS and are readable by the Cloudflare service.

The Mac reads permissions from `~/Library/Application Support/JaDE/remote.json` (mode 600). LaunchAgent `com.mcembalest.jade.remote` opens the **JaDE Mac Connection** menu bar app at login. The app holds folder bookmarks and runs the bridge as a child so that macOS folder permission applies. Its installed scripts and logs are under `~/Library/Application Support/JaDE/`. Python uses only the standard library.

File access rejects symlinks, hidden/parent paths, generated directories, binary files and files larger than 256 KB. It walks directories with descriptor-relative no-follow operations. Existing contents are checked against the phone's SHA-256 revision, backed up under `remote-backups`, and replaced using a flushed sibling file. The bridge checks the revision again immediately before replacement. Other programs do not participate in a shared lock, so avoid simultaneous saves to the exact same file; using a dedicated worktree helps. Backups currently have no retention policy.

No automatic background uploads, directory creation, deletion, rename, terminal, build/test execution, Git commit/push, syntax highlighting or attachment editing is provided by this remote file editor. Existing Notes features remain available in their tab.

## Build and validation

```sh
python3 -m unittest discover -s remote -p 'test_*.py'
cd sync/cloudflare && npm test
```

Deploy the additive schema and Worker, then run `python3 remote/install.py` followed by `python3 remote/install-mac-app.py` from the repo root. The installer provisions the Mac-only agent secret through stdin and grants no folder access by default. Renew the phone with the existing `mobile/install-phone.py` helper.

The optional `LIVE_REMOTE_TEST` Xcode UI test uses an explicitly enabled isolated `JaDE Remote Test` folder containing `sample.py`. It checks browsing, phone draft recovery after app termination, and an acknowledged save through live Cloudflare to the Mac. Remove the test folder permission when finished.

## Verified September 5, 2026

- Updated signed Release app installed on the physical iPhone; all four prior notes retained.
- Four Mac bridge tests passed for save/conflict/backup, new files, revocation, traversal/symlink boundaries, binary files and size limits.
- Four Cloudflare tests passed, including existing note-sync regression coverage and separate agent authentication.
- Live Cloudflare round trip passed for listing, reading, saving a source file, rejecting a stale revision and rejecting an out-of-folder path.
- Documents access initially failed for the bare background service. After the user granted folder access through the Mac helper, a temporary vault note was created/read/verified and removed.
- User confirmed an edit of a real `Documents/first` note on the physical iPhone arrived on the Mac.

- Signed simulator remote UI test passed: source-file browsing, local draft recovery after termination, and acknowledged Save to Mac. Temporary test-folder access was removed after validation.
