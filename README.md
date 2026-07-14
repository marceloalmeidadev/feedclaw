# FeedClaw

Local RSS triage engine for [OpenClaw](https://docs.openclaw.ai).
Single Go binary, state in SQLite, with a web UI (Nuxt) and an agent-facing CLI.

FeedClaw replaces a "scan every headline, open the few that matter, let the rest
pile up" Feedly workflow with: a daily automated fetch, an LLM-grouped daily
digest, drill-down by theme, and persistent read state — driven either
conversationally through the OpenClaw agent or visually through the local UI.

> **Status:** Feature-complete (Phases 1–7). OPML import, feed management, the
> concurrent SSRF-guarded fetcher, diagnostics, read/star state, full-article
> extraction, full-text search, the theme-grouped daily digest, the OpenClaw
> skill/cron, the local REST API, the Nuxt web UI embedded in the binary, and the
> installable OpenClaw bundle. Install: [`docs/INSTALL.md`](docs/INSTALL.md).

## Architecture

- **Engine:** Go 1.23+, no CGO (`modernc.org/sqlite`) → static, cross-compilable binary.
- **State:** SQLite is the single source of truth for read state and cached content.
- **Feeds:** `github.com/mmcdole/gofeed` (RSS/Atom/JSON).
- **Theme clustering:** done by the agent (LLM), not the engine — the engine
  stays free of API keys and just exports unread items and stores the digest.

## Build & test

Requires Go 1.23+.

```sh
make build      # -> bin/feedclaw (no embedded UI)
make test       # go test ./...
make vet lint   # static analysis
make release    # build the UI and embed it into bin/feedclaw (-tags embedui)
make package    # assemble the installable OpenClaw bundle (dist/) + checksums
```

`make release` builds the Nuxt SPA and embeds it via `embed.FS`, so a single
binary serves the UI and API same-origin. `make package` produces the installable
bundle (`dist/feedclaw/` + `SHA256SUMS` + tarball) — see
[`docs/INSTALL.md`](docs/INSTALL.md).

## CLI

Every command accepts `--json` for structured output (the agent always uses it).
The database defaults to `$XDG_CONFIG_HOME/feedclaw/feedclaw.db`; override with
`--db <path>` or the `FEEDCLAW_DB` environment variable.

```sh
# Feeds & fetching
feedclaw import --opml <path|url>      # import an OPML export (file or http/https URL)
feedclaw feeds list [--json]
feedclaw feeds add <url> [--category X]
feedclaw feeds remove <url>
feedclaw fetch [--feed <url>] [--workers 8] [--report <path>]

# Triage / read state
feedclaw unread [--since 24h] [--category X] [--limit N] [--json]
feedclaw mark read <id...> [--older-than 7d] [--all-in-theme <theme-id>]
feedclaw mark unread <id...>
feedclaw star <id...>
feedclaw unstar <id...>
feedclaw full <id> [--force]           # reader-mode extraction (cached, SSRF-guarded)
feedclaw search <query> [--limit N]    # FTS5 full-text search

# Daily digest (themes are grouped by the agent; see skill/)
feedclaw digest save --date YYYY-MM-DD --input <json|->  # agent writes the grouping
feedclaw digest show [--date YYYY-MM-DD] [--json]        # today, or a given/most-recent date
feedclaw theme <theme-id> [--json]                       # all articles in a theme

feedclaw doctor [--json]               # DB / feed health diagnostics
```

The engine guarantees the digest covers every unread article: any unread article
the agent leaves out of a theme is collected into an engine-generated `Outros`
(residual) theme. `digest save` rejects article ids that don't exist or are
already read at save time.

Durations accept `h`/`m`/`s` plus `d` (days) and `w` (weeks): `24h`, `7d`, `2w`.

`fetch` exits with a **semantic code** (the contract the OpenClaw on-exit
pipeline reads) and always writes a JSON run report (`--report`, default
`<config>/feedclaw/last_run.json`):

| code | meaning |
|---|---|
| `0`  | new unread articles — proceed to the digest |
| `10` | success, nothing new — don't wake the agent |
| `20` | partial: some feeds failed, but there are new articles |
| `30` | total failure: no feed reachable |
| `40` | config / database inaccessible |
| `50` | another fetch is already running (lockfile) |

A concurrent `fetch` (e.g. a manual run during the cron, or the UI's
`POST /api/fetch`) is refused with `50`; a stale lock from a dead process is
reclaimed automatically.

Example triage session:

```sh
feedclaw import --opml feedly-export.opml
feedclaw fetch
feedclaw unread --since 24h
feedclaw mark read 101 102 103
feedclaw full 118
feedclaw search "symfony messenger"
```

## HTTP API

`feedclaw serve [--port 8484]` starts a REST API bound **exclusively to
127.0.0.1** (never `0.0.0.0`). It is the same source of truth as the CLI, and
(from Phase 6) serves the embedded UI. Errors use a standard envelope
`{"error": {"code", "message"}}`.

```
GET    /api/feeds
POST   /api/feeds                 {url, category}
DELETE /api/feeds/{id}
POST   /api/fetch                 # synchronous fetch; returns per-feed results
GET    /api/articles?status=unread&category=&theme=&q=&page=&per_page=
GET    /api/articles/{id}
POST   /api/articles/{id}/full    # force reader-mode extraction (SSRF-guarded)
PATCH  /api/articles/read         {ids: [...], read: true|false}
PATCH  /api/articles/star         {ids: [...], starred: true|false}
GET    /api/digests?date=         # given date, else most recent
GET    /api/digests/{date}/themes/{themeId}/articles
POST   /api/feeds/import          # import an uploaded OPML document (XXE-safe)
GET    /api/stats                 # unread total, per-category, starred (badges)
```

## Web UI

The UI is a Nuxt 4 SPA (`ssr: false`) in `ui/`, built with Nuxt UI. In
development it proxies `/api` to a running `feedclaw serve`; in production
(Phase 7) the built SPA is embedded in the Go binary and served same-origin.

```sh
cd ui
npm install
npm run dev        # dev server (proxies /api → 127.0.0.1:8484)
npm run generate   # static SPA into ui/.output/public (embedded in Phase 7)
npm run lint
```

Pages: **Hoje** (digest), **Triagem** (keyboard-driven triage: `j`/`k`/`m`/`s`/
`o`/`v`/`Shift+A`), **Leitor** (sanitized reader mode), **Feeds**, **Histórico**.
Feed HTML is sanitized with DOMPurify before rendering (XSS defense).

## OpenClaw integration

FeedClaw ships as a bundle plugin under `skill/` with **two skills**:

- **`feedclaw`** — conversational triage: maps requests (*"o que saiu hoje?"*,
  *"me mostra o tema 2"*, *"marca o tema 2 como lido"*, *"abre o artigo 118"*,
  *"procura sobre X"*) onto the CLI.
- **`feedclaw-digest`** — the daily flow, woken by a cron **on-exit** trigger
  when the scheduled `feedclaw fetch` finishes. Its first step reads the run
  report's `exit_code` and only clusters/saves when there is something new (0 or
  20), so the LLM is never woken on a quiet day.

Both instruct the agent to treat article content as **untrusted data** — never
execute instructions found inside articles.

The cron pipeline (deterministic fetch + on-exit digest), the exit-code
contract, `utilityModel` tiering, the dedicated session and the operator-scope
prerequisite are documented in
[`docs/openclaw-integration.md`](docs/openclaw-integration.md).

## Security (enforced from Phase 1)

- **SSRF guard, always on.** Only `http`/`https`; DNS is resolved and private,
  loopback and link-local addresses (RFC1918, `127.0.0.0/8`, `169.254.0.0/16`,
  `::1`, `fc00::/7`) are blocked — the fetcher dials the exact validated IP to
  defeat DNS rebinding, and re-validates every redirect hop (max 5). There is no
  "loose" mode. Modes: `restricted` (default) and `allowlist`.
- **OPML XXE-safe.** Parsing uses `encoding/xml`, which never resolves external
  entities or fetches DTDs; `DOCTYPE` declarations are rejected outright.
- **Payload limits.** Feed bodies are capped (default 2 MiB) with per-request
  connect/read timeouts.

## License

[MIT](LICENSE) © Marcelo Almeida.
