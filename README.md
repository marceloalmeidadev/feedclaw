# FeedClaw

Local RSS triage engine for [OpenClaw](https://github.com/marceloalmeidadev/feedclaw).
Single Go binary, state in SQLite, with a web UI (Nuxt) and an agent-facing CLI.

FeedClaw replaces a "scan every headline, open the few that matter, let the rest
pile up" Feedly workflow with: a daily automated fetch, an LLM-grouped daily
digest, drill-down by theme, and persistent read state — driven either
conversationally through the OpenClaw agent or visually through the local UI.

> **Status:** Phase 1 (engine core) — OPML import, feed management, the
> concurrent SSRF-guarded fetcher, and diagnostics. Read state, digests, the
> HTTP API and the UI arrive in later phases (see `docs/`).

## Architecture

- **Engine:** Go 1.23+, no CGO (`modernc.org/sqlite`) → static, cross-compilable binary.
- **State:** SQLite is the single source of truth for read state and cached content.
- **Feeds:** `github.com/mmcdole/gofeed` (RSS/Atom/JSON).
- **Theme clustering:** done by the agent (LLM), not the engine — the engine
  stays free of API keys and just exports unread items and stores the digest.

## Build & test

Requires Go 1.23+.

```sh
make build      # -> bin/feedclaw
make test       # go test ./...
make vet lint   # static analysis
```

## CLI (Phase 1 subset)

Every command accepts `--json` for structured output (the agent always uses it).
The database defaults to `$XDG_CONFIG_HOME/feedclaw/feedclaw.db`; override with
`--db <path>` or the `FEEDCLAW_DB` environment variable.

```sh
feedclaw import --opml <path|url>      # import an OPML export (file or http/https URL)
feedclaw feeds list [--json]
feedclaw feeds add <url> [--category X]
feedclaw feeds remove <url>
feedclaw fetch [--feed <url>] [--workers 8]
feedclaw doctor [--json]               # DB / feed health diagnostics
```

Example:

```sh
feedclaw import --opml feedly-export.opml
feedclaw fetch --json
feedclaw doctor
```

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

TBD.
