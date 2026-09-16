# Daily companion research

Each installation owns a companion notebook: name, character description, research instructions in ordinary language, persistent working memory, image, time zone and daily hour. New users start with blank identity and instructions. Sanjana is an optional existing user's configuration, not the default personality.

The desktop **Character, research & history** dialog edits this notebook and uploads a custom image (PNG/JPEG/WebP under 500 KB). The iPhone Companion settings edit the same notebook, retain drafts offline, and show paginated research history. Both save against the version they loaded; a newer edit on another device produces a conflict instead of silently overwriting it.

Cloudflare checks the daily schedule hourly. A Durable Object starts one Linux container running Codex with a separately authorized ChatGPT subscription. No open Mac or iPhone is required. Sign in from the desktop dialog with **Connect cloud Codex**, complete OpenAI device authentication, then **Check connection**. Cloud credentials stay in the Durable Object's private storage; they are not copied from the Mac or included in research archives/backups. Device login may need enabling in ChatGPT security settings. Container hosting uses the Cloudflare account; model execution uses the connected subscription and its usage limits.

Every run receives the retained archive, identity, brief and writable MEMORY.md. It returns a daily report, source-linked findings and updated memory. Reports, individual findings, notebook versions and run outcomes are permanently journaled separately from the 100-message recent feed. Concurrent human memory edits win; the agent's proposed memory remains in the archive for the next run. Pausing prevents new runs and withholds publication of an already running report until resumed. Its results are still retained.

A durable daily reservation prevents duplicate model calls from overlapping cron ticks. An uncertain/failed run is not automatically replayed that day; its error is shown and the next day can try again. The runner currently loads up to 32 MB of retained history, with an eight-minute research target and nine-minute process limit. Exceeding capacity fails visibly and preserves history; it does not silently discard older records. Chat remains an explicit Mac-backed action; daily research is server-side.

## Deployment

1. Export and verify a database backup: `python3 remote/backup-cloud.py`.
2. In this directory run `npm ci`, then `npx wrangler d1 execute jade-personal-sync --remote --file companion.sql`.
3. Run `npx wrangler deploy` with Docker running. `production.js` registers the container class and exports the existing Worker. The account needs Containers support.
4. Backfill the archive with `UPDATE companion_state SET revision=revision+1 WHERE id=1` using D1. Set an existing user's notebook through the authenticated API; preserve their profile, history and pause setting. Never install a personal character as a new-user default.
5. Complete the separate cloud device login and check the connection. Fill a research brief and resume research explicitly if paused.

Worker tests cover persistent history, backup restoration, concurrent daily ticks, notebook conflicts, pause/resume and failure preservation. A successful deployment or device login alone is not proof of a successful model run; inspect the recorded daily outcome. Container sandbox support must also be checked on the deployed runtime (local AMD64 emulation on Apple Silicon may not support the Linux sandbox).
