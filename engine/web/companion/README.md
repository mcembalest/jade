# Sanjana

## Setup

| Requirement | Command |
| --- | --- |
| JaDE | `go install github.com/mcembalest/jade@main` |
| Codex runtime on macOS | `brew install --cask codex` |
| Subscription sign-in | `codex login` |
| Check sign-in | `codex login status` |
| Check runtime | `codex --version` |

Chat: `codex` on `PATH` · subscription sign-in · configured model
Verified: CLI 0.153.4; newer models may require an upgrade
Editor + animation: no Codex required · Node not required

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

| Research / delivery | Setting |
| --- | --- |
| Research | Enabled, open pages only; ≤1/hour across windows; no catch-up |
| Model | Configured model · low reasoning · 90 s timeout |
| Search budget | ≤2 searches + 1 page; ≤600 characters; ≤3 original sources |
| Context | Character, recent chat, pending findings |
| Filtering | Source required; duplicate pending source URLs skipped |
| Pending view | Findings, timestamps, sources, last check, errors; opening preserves findings |
| Delivery | 8pm host-local time; ≤1 update/day; no extra model call |
| Late delivery | Same evening on reopening or next successful research; no missed-day backlog |
| History | Delivered findings retained in chat |
| Queue | Persistent; research pauses at 24 findings until delivery |
| Hide / close all pages | Pause research and delivery; deadlines retained |
| Sleep / background tabs | May delay research and delivery |
| Chat | Always available; stops same-page research; includes pending findings |
| Shared state | Checked approximately every 15 s |
| Errors | Retry next hourly opportunity |
| Usage | Codex subscription allowance |

## Character and history

| Data | Location / behavior |
| --- | --- |
| Character guidance | [character.md](character.md), embedded at build time |
| Saved chat, pending research, check status, visibility, deadlines, read status | `JaDE/companion/chat.json` under the OS user configuration directory |
| macOS configuration directory | `~/Library/Application Support` |
| Saved history | Latest 100 messages; latest 40 messages, bounded to 64 KB, supplied as context |
| Animation preference | Browser local storage, per origin |
| Credentials | Managed by Codex; not returned to the browser |

| Requests / artwork | Details |
| --- | --- |
| Sent to Codex | Profile + recent conversation; no editor files |
| Session | Ephemeral app-server thread; temporary directory; read-only sandbox |
| Disabled | Shell, connected apps, plugins, browser/computer control, multi-agent; additional permissions rejected |
| Source links | New tab; Reddit/paywalls depend on web-search access |
| Sprite | `spritesheet.png` · 8 × 9 cells · 192 × 208 px |
| Playback | Embedded; idle + wave; paused on hidden pages |

## Development

Repository root:

```sh
npm --prefix engine/web run build
npm --prefix engine/web test
```

Profile, artwork, code changes → rebuild + restart
Regression tests: fake Codex + intercepted responses; no subscription usage.

Optional live check (uses subscription allowance):

```sh
JADE_LIVE_CHECK=1 go test ./engine -run TestCompanionLive -v -count=1
```

## References

- [Codex app-server](https://learn.chatgpt.com/docs/app-server)
- [Codex authentication](https://learn.chatgpt.com/docs/auth)
- [Web-search configuration](https://learn.chatgpt.com/docs/config-file/config-basic)
