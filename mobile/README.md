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

The **Sanjana** tab brings you her discoveries and proactive updates. It has no message composer or chat controls. It uses the same desktop character, discovery queue, visibility setting, and hourly research / 8pm daily-update limits through the existing paired connection. Ordinary desktop chat messages are not shown in this update feed.

The existing sprite is bundled unchanged, with idle/wave animation, a Still animation preference, and system Reduce Motion support. Source links open natively. Downloaded updates remain readable offline. Research checks run while this tab is foregrounded, sharing the desktop's limits; they do not require you to send a message. Closing/backgrounding the tab stops new checks, though work already accepted by the Mac may finish. Your Mac and its desktop JaDE service must be awake for new discoveries. This does not add iOS background notifications.

Version 0.3.1 build 5 removes the composer introduced in 0.3.0. Existing cached conversation/draft data is retained, but does not block discovery checks or appear as a chat interface. The native UI test verifies the update feed and absence of chat controls. The relay remains scoped to the existing companion service; note storage and folder grants are unchanged.
