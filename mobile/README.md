# JaDE for your iPhone

The [personal stable-v1 baseline](STABLE-V1.md) records the two workflows and behavior to preserve.

Open **JaDE.xcodeproj** in Xcode. This is a native SwiftUI iPhone/iPad app with local text storage, a persistent upload queue, Keychain pairing, and explicit sync status.

See [personal sync setup](../sync/README.md) for installation, pairing, limits and recovery.

`JaDE/SyncStore.swift` is independent of SwiftUI and can be tested on macOS. `JaDE/JaDEApp.swift` supplies the native interface and secure pairing. `JaDEUITests` verifies offline editing survives terminating the app.

Build without signing to verify compilation:

```sh
xcodebuild -project mobile/JaDE.xcodeproj -scheme JaDE -destination 'generic/platform=iOS' CODE_SIGNING_ALLOWED=NO build
```

Installing on a physical phone requires your Apple account, a selected signing team and a connected trusted device. A macOS build cannot be copied onto the phone.

For subsequent installation or renewal with the same Apple team and iPhone:

```sh
python3 mobile/install-phone.py --team YOUR_TEAM_ID --device YOUR_IPHONE_UDID
```

Add `--pair` on the first installation to use the Mac's existing pairing configuration. The installer updates the existing app without deleting its local notes. With a free Personal Team, renew before the seven-day provisioning expiry; do not uninstall the app to renew it.

The **Mac files** tab browses and edits enabled writing folders and source files on your awake Mac. See [remote editing](../remote/README.md) for folder permissions, manual Save to Mac, and draft recovery.

The optional **Mac files → Cloud projects** flow fetches persistent cloud copies and explicitly submits drafts for delivery while the Mac is off. It is distinct from direct **Save to Mac**. Enable folders separately in the updated Mac helper. See [Cloud projects](../sync/cloudflare/PROJECTS.md).

Cloud projects 0.2.1 adds resumable bulk offline downloads, three-version conflict comparison with preserved original drafts, and daily Cloudflare backup status. Downloads fill missing phone copies; Fetch still updates existing copies explicitly. See [the cloud contract](../sync/cloudflare/PROJECTS.md) for backup retention and recovery.
## Source editing

The Mac files editor uses a native text view with a keyboard toolbar: indent selected lines by four spaces, remove up to four leading spaces (or one tab), insert code punctuation, undo/redo, and dismiss the keyboard. Smart quotes and smart dashes are disabled so typing preserves source punctuation. Each action uses the same phone-draft binding as typing; **Save to Mac** is still explicit. Notes editing is unchanged.

`JaDE/SourceTextEditor.swift` contains only presentation and in-memory editing. Its line-edit helper uses UTF-16 selection offsets to match UIKit and preserves existing line endings. It has no storage or network dependencies. See [editor tests](tests/README.md) for pure Swift and isolated native verification.

## Sanjana

The **Sanjana** tab is another entry point to the existing desktop companion, using the same profile, conversation history, pending discoveries, visibility setting, and research/daily-delivery limits. It uses the existing paired Cloudflare relay to reach the desktop `/companion` service at `127.0.0.1:7339`; credentials stay on the Mac. Keep the Mac awake with its desktop JaDE service and updated Mac Connection helper running.

The existing sprite is bundled unchanged, with native idle/wave animation, a Still animation preference, and system Reduce Motion support. Chat and discovery source links are native views. The phone keeps its last downloaded conversation and unsent message draft offline. It checks shared state while the tab is foregrounded; when no draft is being composed, it can request research using the desktop's shared hourly limit. New discovery delivery follows the existing 8pm Mac-local schedule. Closing/backgrounding the tab stops new checks; a request already accepted by the Mac may still finish.

Chat messages persist their request ID before submission. Refresh checks that ID without resending the chat, even after a lost response or app restart. An unconfirmed message stays as a draft; check shared history before deliberately sending it again. This does not introduce a separate persona or expand the desktop's existing 100-message retained history. Cloudflare relays conversation data under the existing pairing credentials; this is not end-to-end encryption.

Install the updated Worker and Mac Connection helper before the phone build. Version 0.3.0 build 4 includes the new tab. Tests: `SanjanaStoreTests.swift` covers shared state, uncertain delivery, new typing, offline cache and corrupt-cache preservation; `testSanjanaCornerAndOfflineDraft` exercises the native screen and restart. Relay and bridge tests use fake responses, without consuming model allowance. A production read-only relay check matched the desktop state exactly.
