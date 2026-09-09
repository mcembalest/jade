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
It requires the Mac and cloud online, and is independent of paid cloud research.
The bounded desktop prompt uses at most 40 messages / 64KB; cloud research uses at
most 12 messages / 12,000 characters and bounded pending context. This is not
unlimited long-term memory.

## AI and live search: activation still required

The deployed scheduler and shared reads are active, but **paid research is disabled**
(`SANJANA_AI_ENABLED=false`). There is no production AI success claim yet.

The isolated adapter in `sync/cloudflare/companion-provider.js` targets Anthropic
Haiku 4.5 through Cloudflare AI Gateway's AI binding. Anthropic handles both inference
and live web search. A request permits two searches, one fetched page (3,000 content
tokens), 1,200 output tokens and a 90-second local wait; final findings are limited
to 600 characters and three original citation URLs. Incomplete/uncited outputs are
rejected without continuation or retry. Publication makes no AI call.

To activate: create authenticated gateway `jade-sanjana`, disable request/response
logging and retries, configure a monthly spend limit (proposed $10), fund Unified
Billing credits, and then set `SANJANA_AI_ENABLED=true` and deploy. Verify one sourced
scheduled result and the gateway's enforced spending rule before claiming it active.
Existing Wrangler OAuth can deploy Workers but returned 403 when reading gateway
settings, so gateway setup requires dashboard access or an appropriately scoped
Cloudflare credential. Never put that credential in the repository.

As checked September 8, 2026, Cloudflare Unified Billing adds a 5% credit-purchase
fee and passes through provider rates. Haiku 4.5 is $1/million input tokens and
$5/million output tokens; web search is $10/1,000 searches, plus text tokens.
Two searches every hour would alone cost about $14.40 in a 30-day month; a $10
spending cap therefore can stop research before month end. The limit must be
configured in AI Gateway; it is **not currently configured or a code-enforced
monetary cap**. Daily publication, cached reading and pause continue without credits.

Cloudflare's native Web Search binding was also investigated. The installed
Wrangler supports it, but a real query returned `account_disabled` (7078). It is
not usable for this account today. AI Search indexes a configured corpus; it is not
a substitute for general live discovery. Browser rendering alone does not search.
A Codex subscription/sign-in is not a Worker execution credential.

References:
- [Cloudflare web-search providers](https://developers.cloudflare.com/ai-gateway/usage/web-search/)
- [Unified Billing and credit setup](https://developers.cloudflare.com/ai-gateway/features/unified-billing/)
- [Provider pricing](https://platform.claude.com/docs/en/about-claude/pricing)
- [Native Web Search introduction](https://github.com/cloudflare/workers-sdk/releases/tag/wrangler%404.96.0)

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
