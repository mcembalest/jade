# Personal mobile v1 — September 6, 2026

This is the behavior to preserve. It was used successfully at a wedding on September 5, including writing new notes and editing the existing Obsidian vault from an iPhone.

## Two deliberate workflows

**Notes** is a local-first personal notebook. Edits save on the phone and sync through Cloudflare to `~/JaDE Mobile`. Status distinguishes local pending edits, Cloudflare acceptance, and acknowledgement by the other device. A cloud upload is not reported as delivery to the Mac.

**Mac files** browses explicitly enabled folders on the awake Mac, currently including `~/Documents/first`. Opening a file retrieves it from the Mac. Typing saves a phone draft; **Save to Mac** explicitly applies it. Fetching and saving remain separate actions. Conflicting Mac edits must not be silently overwritten. Drafts can be recovered or exported.

The user prefers this separation and transparency over a consolidated sync experience. Preserve it unless they request a change. Coding and AI features are deferred.

## Daily use

- USB is unnecessary after installation. Use Wi-Fi or cellular.
- Keep the Mac plugged in, awake and online for Mac-file access. Keep the JaDE Mac Connection menu bar app running.
- Wait for **Saved on Mac** after a remote save. A timeout is an uncertain result, not proof that the write failed.
- Select additional folders through JaDE Mac Connection's folder chooser. This grants access to the actual selected working copy; repos are not automatically isolated.
- Free Apple signing requires periodic rebuild/reinstall, normally every seven days. Use the existing reinstall helper over the app; do not uninstall it to renew.

## Recovery and scope

The Mac's original remote-file contents are backed up under `~/Library/Application Support/JaDE/remote-backups`. Notes retain accepted revisions in Cloudflare D1. Local phone drafts are not a server backup until sent. Use Export when an independent copy is needed.

This is a personal v1, not complete Obsidian compatibility: no mobile plugin system, attachment editing, automatic rename/deletion propagation, terminal or AI. Mac files supports UTF-8 text up to 256 KB; Notes supports Markdown/text up to 512 KB. See the respective READMEs for other limits and recovery details.

## Validation evidence

- Physical iPhone installation, prior-note preservation and Notes delivery confirmed.
- User confirmed editing an existing `Documents/first` note on the physical phone and receiving the change on the Mac.
- Live Cloudflare/Mac tests verified read, write, stale revision rejection and folder boundaries.
- Signed simulator UI test passed: browse a source file, edit it, terminate/reopen the app, recover the draft, and receive **Saved on Mac**. The successful run is `.tmp/remote-ui-test4.xcresult` (local artifact, not part of Git).
- Earlier unsigned simulator attempts could not retain Keychain pairing. Live remote UI testing requires a signed simulator build and prior pairing; this was a test setup issue.

Source checkpointing does not require reinstalling or restarting either app. Keep functional changes separate from documentation and housekeeping so this working baseline remains easy to restore.
