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
