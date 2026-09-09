# Sanjana

Sanjana is one persistent character shared by desktop and iPhone. The mobile tab
shows discoveries and proactive daily updates, with no chat composer. Her existing
artwork and [character profile](character.md) are preserved.

## Cloud ownership

`jade-personal-sync` runs an hourly Cloudflare Cron (`0 * * * *`). It first publishes
pending findings at or after 20:00 America/New_York, then reserves at most one
research opportunity per elapsed hour. New York calendar dates handle DST. Empty
queues produce no empty update. Missed hours do not cause catch-up model calls.
Neither client can trigger research or publication; opening/refreshing only reads.
The Mac, browser and phone can all be off.

D1's `companion_state` is the authority for the profile, latest 100 messages,
24 pending findings, shared pause setting, daily date, last research reservation,
and recent source deduplication (up to 2,000 normalized URLs). It is a small,
versioned document, updated with compare-and-swap. Publication appends its daily ID
and clears the queue in the same atomic update. Conflicting storage writes retry;
provider calls never retry automatically. A failed/uncertain attempt consumes its
hourly opportunity. Findings completing during a pause are kept for later delivery.
Pause prevents subsequent research and publication; resume does not trigger work.

**Pause research everywhere** is an explicit shared control. Hiding the character
or stopping animation is local and never changes the schedule. Both clients cache
cloud state for offline reading. The mobile feed excludes ordinary chat messages.

Desktop chat remains an explicit Mac feature using the signed-in Codex runtime.
It reads the cloud profile/history and appends the reply to that same cloud history.
It requires the Mac and cloud online. The bounded desktop prompt uses at most
40 messages / 64KB. This is not unlimited long-term memory.

## OpenAI/Codex subscription requirement

**Sanjana must use the user's OpenAI/Codex subscription.** Another provider or a
separately billed API is not an approved substitute. Desktop chat already uses
the signed-in Codex runtime and live search on the Mac.

The Cloudflare scheduler, shared history and daily publication are deployed, but
**autonomous research is not connected**. The cloud provider module reports this
blocker and makes no AI request. There is no AI binding or paid-provider activation
flag. An hourly opportunity is recorded as blocked, never as successful research;
existing findings remain available for scheduled publication.

Official [Codex authentication documentation](https://developers.openai.com/codex/auth/)
supports ChatGPT sign-in on remote/headless machines using device authentication.
It distinguishes that subscription access from API-key billing. A Codex runtime
hosted in Cloudflare (for example, in a container) is therefore a candidate path;
it still needs implementation, authenticated runtime persistence and validation.
A direct subscription-authenticated Worker inference endpoint has not been
established. Subscription-backed cloud execution is not ruled out.

Do not configure AI Gateway credits or native Cloudflare Web Search to activate
this implementation. Prefer a dedicated device-auth login for a future cloud
runtime; never expose login credentials or substitute paid API credentials.
No autonomous Mac scheduler or client-triggered research is enabled.
The target remains bounded, sourced research (about two searches and one page per
hour), up to 24 pending findings, and one daily update around 8pm New York time.

## Migration, deployment and recovery

Apply `sync/cloudflare/companion.sql` to the existing D1 database, deploy the Worker,
install the new Mac service, then run `python3 remote/migrate-companion.py`. The tool
locks legacy research/history, privately backs up `chat.json`, and persists the
migration payload before uploading. It uses the existing agent credential. A retry
uses the same identity; an existing different cloud history is never overwritten.
The original local file remains untouched. Mobile legacy drafts/receipts stay on
disk and are neither sent again nor displayed as a composer.

The existing 07:17 UTC daily R2 exporter snapshots the complete companion row and
schema in the same transaction as other mutable heads. Recovery restores the profile,
queue, history, pause, deduplication and reservation clocks. Old pre-companion
manifests still restore. `remote/backup-cloud.py` exports the entire database too.
Existing Notes, Cloud projects, folder grants and explicit fetch/submit semantics
are unchanged.

Tests: `npm test` in `sync/cloudflare`; Go race suite; browser companion/recursive
checks; `mobile/tests/SanjanaStoreTests.swift`; native `testSanjanaUpdatesWithoutChat`.
Fake providers and controlled clocks exercise no-UI scheduling, concurrent ticks,
failures, pause/resume, URL deduplication, DST, downtime, migration and restoration.
