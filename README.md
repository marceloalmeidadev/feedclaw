# FeedClaw

Local RSS triage engine for [OpenClaw](https://docs.openclaw.ai): a single Go
binary (state in SQLite) with an agent-facing CLI, a REST API, and an embedded
Nuxt web UI.

It replaces the Feedly "scan every headline, open the few that matter, let the
rest pile up" habit with a daily automated fetch, an LLM-grouped digest by theme,
drill-down, and persistent read state — used conversationally through the
OpenClaw agent or visually in the local UI. Theme clustering is done by the agent
(LLM), so the engine stays free of API keys: it exports unread items and stores
the digest the agent builds.

## Install

Download the latest [release](https://github.com/marceloalmeidadev/feedclaw/releases),
verify the checksum, and follow **[`docs/INSTALL.md`](docs/INSTALL.md)** (binary +
the two OpenClaw skills):

```sh
sha256sum -c SHA256SUMS
```

CLI only (no UI): `go install github.com/marceloalmeidadev/feedclaw/cmd/feedclaw@latest`.

## CLI

Every command accepts `--json` for structured output (the agent always uses it).
The database defaults to `$XDG_CONFIG_HOME/feedclaw/feedclaw.db`; override with
`--db <path>` or `FEEDCLAW_DB`. Durations accept `h`/`m`/`s` plus `d`/`w`
(`24h`, `7d`, `2w`).

```sh
# Feeds & fetching
feedclaw import --opml <path|url>      # OPML export (file or SSRF-guarded http/https URL)
feedclaw feeds list | add <url> [--category X] | remove <url>
feedclaw fetch [--feed <url>] [--workers 8] [--report <path>]

# Triage / read state
feedclaw unread [--since 24h] [--category X] [--limit N]
feedclaw mark read <id...> [--older-than 7d] [--all-in-theme <theme-id>]
feedclaw mark unread <id...>
feedclaw star | unstar <id...>
feedclaw full <id> [--force]           # reader-mode extraction (cached, SSRF-guarded)
feedclaw search <query> [--limit N]    # FTS5 full-text search

# Daily digest (themes grouped by the agent)
feedclaw digest save --date YYYY-MM-DD --input <json|->
feedclaw digest show [--date YYYY-MM-DD]
feedclaw theme <theme-id>

feedclaw serve [--port 8484]           # REST API + UI on 127.0.0.1
feedclaw doctor                        # DB / feed health
```

Every unread article is covered by the digest: whatever the agent leaves out of a
theme lands in an engine-generated `Outros` theme, and `digest save` rejects ids
that don't exist or are already read. `fetch` returns a **semantic exit code**
(`0` new · `10` nothing new · `20` partial · `30` network fail · `40` config ·
`50` locked) and always writes a JSON run report — the contract read by the
OpenClaw on-exit digest pipeline (see below).

## HTTP API

`feedclaw serve` binds **only to 127.0.0.1** (never `0.0.0.0`), is the same source
of truth as the CLI, and serves the embedded UI same-origin. Errors use a standard
envelope `{"error": {"code", "message"}}`.

```
GET    /api/feeds
POST   /api/feeds                 {url, category}
DELETE /api/feeds/{id}
POST   /api/feeds/import          # uploaded OPML document (XXE-safe)
POST   /api/fetch                 # synchronous fetch; per-feed results
GET    /api/articles?status=unread&category=&theme=&q=&page=&per_page=
GET    /api/articles/{id}
POST   /api/articles/{id}/full    # force reader-mode extraction (SSRF-guarded)
PATCH  /api/articles/read         {ids: [...], read: true|false}
PATCH  /api/articles/star         {ids: [...], starred: true|false}
GET    /api/digests?date=         # given date, else most recent
GET    /api/digests/{date}/themes/{themeId}/articles
GET    /api/stats                 # unread total, per-category, starred (badges)
```

## OpenClaw integration

Two skills under `skill/`: **`feedclaw`** (conversational triage — maps requests
like *"o que saiu hoje?"* or *"me mostra o tema 2"* onto the CLI) and
**`feedclaw-digest`** (the daily flow, woken by a cron **on-exit** trigger when
the scheduled `feedclaw fetch` finishes; its first step reads the run report's
`exit_code` and only clusters when there is something new, so the LLM is never
woken on a quiet day). Both treat article content as **untrusted data**.

The cron pipeline, exit-code contract, `utilityModel` tiering and the
operator-scope prerequisite are in
**[`docs/openclaw-integration.md`](docs/openclaw-integration.md)**.

## Web UI

Nuxt 4 SPA (`ssr: false`) in `ui/`, built with Nuxt UI and embedded into the
binary via `embed.FS`. Pages: **Hoje** (digest), **Triagem** (keyboard triage —
`j`/`k`/`m`/`s`/`o`/`v`/`Shift+A`), **Leitor** (reader mode), **Feeds**,
**Histórico**. Feed HTML is sanitized with DOMPurify before rendering (XSS
defense).

```sh
cd ui && npm install
npm run dev        # proxies /api → a running `feedclaw serve` on :8484
npm run generate   # static SPA (embedded by `make release`)
```

## Development

Go 1.25+, Node 22+.

```sh
make build      # -> bin/feedclaw (no embedded UI)
make test       # go test ./...
make vet lint   # go vet + golangci-lint
make release    # build the UI and embed it (-tags embedui)
make package    # installable bundle (dist/) + SHA256SUMS
```

Engine: no CGO (`modernc.org/sqlite`) → static, cross-compilable binary. SQLite
is the single source of truth; feeds parsed with `mmcdole/gofeed` (RSS/Atom/JSON).

## Security

- **SSRF guard, always on.** Only `http`/`https`; DNS is resolved and private,
  loopback and link-local addresses (RFC1918, `127.0.0.0/8`, `169.254.0.0/16`,
  `::1`, `fc00::/7`) are blocked — the fetcher dials the exact validated IP to
  defeat DNS rebinding and re-validates every redirect hop. No "loose" mode
  (`restricted` default, or `allowlist`). A `forbidigo` lint rule bans raw
  `net/http` clients outside the guarded layer.
- **OPML XXE-safe** — `encoding/xml` never resolves external entities; `DOCTYPE`
  is rejected.
- **Payload limits & timeouts** on every outbound request; the API/UI never leave
  loopback; feed HTML is sanitized before rendering.

## License

[MIT](LICENSE) © Marcelo Almeida.
