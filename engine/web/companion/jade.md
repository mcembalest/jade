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
| Show Sanjana | Resume with a new interval |
| Still animation | Stop animation; system reduced-motion preferences also apply |
| Escape / close / click outside | Dismiss the popover |

## Discovery schedule

- Random interval: 20–60 minutes, checked approximately every 15 seconds by open JaDE pages.
- Browser timer throttling and computer sleep can delay an opportunity. No searches run after all JaDE pages close.
- One shared deadline and one in-flight request across tabs and JaDE processes for the same OS user.
- Chat requests also reset the discovery deadline. Failed searches wait for the next interval; missed intervals do not accumulate.
- Sanjana may search or remain quiet at an opportunity. Discoveries use a speech bubble without opening the popover or moving keyboard focus.
- Requests use the same subscription allowance as other Codex work. Rate-limit and sign-in errors appear in chat.

## Character and history

| Data | Location / behavior |
| --- | --- |
| Character guidance | [character.md](character.md), embedded at build time |
| Saved chat, visibility, deadline, read status | `JaDE/companion/chat.json` under the OS user configuration directory |
| macOS configuration directory | `~/Library/Application Support` |
| Saved history | Latest 100 messages; latest 40 messages, bounded to 64 KB, supplied as context |
| Animation preference | Browser local storage, per origin |
| Credentials | Managed by Codex; not returned to the browser |

The profile and recent conversation are sent to Codex. Each request uses an ephemeral app-server thread in a temporary directory. JaDE does not attach editor files. Shell, connected apps, plugins, browser/computer control, and multi-agent features are disabled for companion threads; the sandbox is read-only. Additional permission requests are rejected. Source links open in a new tab; Reddit and paywalled-page access depends on web-search availability.

## Artwork

`spritesheet.png`: 8 columns × 9 rows, 192 × 208 pixel cells. Go embeds the sprite; the frontend plays idle and wave rows. Hidden pages suspend animation.

## Development

```sh
npm run build
npm test
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
