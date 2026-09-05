# Sanjana

## Setup

| Requirement | Command |
| --- | --- |
| JaDE | `go install github.com/mcembalest/jade/cmd/jade@main` |
| Codex runtime on macOS | `brew install --cask codex` |
| Subscription sign-in | `codex login` |
| Check sign-in | `codex login status` |
| Check runtime | `codex --version` |

Live chat uses `codex` from `PATH`, ChatGPT subscription authentication, and the configured Codex model. Verified with Codex CLI 0.153.4. Upgrade Codex if the configured model requires a newer runtime. The editor and companion animation work without Codex. Node is not required for installation or use.

## Controls

| Control | Behavior |
| --- | --- |
| Sanjana | Open chat in the existing popover |
| Send / Enter | Send a message; explicit web-search requests are supported |
| Shift+Enter | Insert a newline |
| Stop | Cancel the current request |
| Speech bubble | Open an unread autonomous discovery |
| Hide Sanjana | Pause discoveries across JaDE windows |
| Show Sanjana | Resume without resetting the daily limit |
| Still animation | Stop animation; system reduced-motion preferences also apply |
| Escape / close / click outside | Dismiss the popover |

## Research and delivery

- Quiet research starts when JaDE is open and Sanjana is enabled, then runs at most once an hour across all JaDE windows. Missed hours do not accumulate.
- Each research turn uses the configured Codex model with low reasoning effort and a 90-second timeout. The prompt requests at most two searches, one follow-up page, one finding of at most 600 characters, and up to three original source links.
- Pending research in the popup shows all collected findings, their collection times, source links, the last completed check, and any research error. Opening the popup does not send a notification or clear pending findings.
- Research includes the character notes, recent conversation, and pending findings as context. Duplicate primary source URLs within pending research are skipped. Unsourced findings are not saved.
- At 8pm in the JaDE host computer’s local timezone, pending findings are combined into one daily chat update and speech bubble. Delivery uses no additional model call. The findings remain in chat history after leaving the pending list.
- At most one proactive update per local calendar day. Research continues afterward for the next update. If no findings are ready at 8pm, the next successful research can supply that evening’s update.
- Late openings can deliver that evening; missed days do not create multiple notifications. Before 8pm, pending findings remain available for manual check-ins.
- Pending research survives restarts. At 24 pending findings, research pauses until daily delivery frees space; existing findings are not dropped.
- Browser timer throttling and computer sleep can delay work. No research runs after all JaDE pages close. Hiding Sanjana pauses research and delivery without resetting their deadlines.
- Chat remains available at any time. Sending a chat message stops research running in the same page, and includes pending findings as context.
- Open pages check shared state approximately every 15 seconds. Errors wait until the next hourly opportunity instead of retrying continuously.
- Requests use the same subscription allowance as other Codex work.

## Character and history

| Data | Location / behavior |
| --- | --- |
| Character guidance | [character.md](character.md), embedded at build time |
| Saved chat, pending research, check status, visibility, deadlines, read status | `JaDE/companion/chat.json` under the OS user configuration directory |
| macOS configuration directory | `~/Library/Application Support` |
| Saved history | Latest 100 messages; latest 40 messages, bounded to 64 KB, supplied as context |
| Animation preference | Browser local storage, per origin |
| Credentials | Managed by Codex; not returned to the browser |

The profile and recent conversation are sent to Codex. Each request uses an ephemeral app-server thread in a temporary directory. JaDE does not attach editor files. Shell, connected apps, plugins, browser/computer control, and multi-agent features are disabled for companion threads; the sandbox is read-only. Additional permission requests are rejected. Source links open in a new tab; Reddit and paywalled-page access depends on web-search availability.

## Artwork

`spritesheet.png`: 8 columns × 9 rows, 192 × 208 pixel cells. Go embeds the sprite; the frontend plays idle and wave rows. Hidden pages suspend animation.

## Development

From the repository root:

```sh
npm --prefix engine/web run build
npm --prefix engine/web test
```

Rebuild and restart JaDE after changing the character profile, artwork, or code. Regression tests use a fake Codex process and intercepted browser responses; they do not spend subscription usage.

Optional live check (uses subscription allowance):

```sh
JADE_LIVE_CHECK=1 go test ./engine -run TestCompanionLive -v -count=1
```

## References

- [Codex app-server](https://learn.chatgpt.com/docs/app-server)
- [Codex authentication](https://learn.chatgpt.com/docs/auth)
- [Web-search configuration](https://learn.chatgpt.com/docs/config-file/config-basic)
