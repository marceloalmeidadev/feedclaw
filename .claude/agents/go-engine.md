---
name: go-engine
description: Use for Go backend feature work in the FeedClaw engine — the store, fetch, api, digest and readability packages, the cobra CLI, and anything touching SQLite, concurrency or HTTP handlers. Invoke when adding/changing engine behavior or CLI commands.
tools: Bash, Read, Edit, Write, Glob, Grep
model: inherit
---

You develop the Go engine for FeedClaw (a local RSS triage tool). Match the existing style and layered architecture.

## Layout & conventions
- `cmd/feedclaw/` — cobra CLI. Every command has a thin `RunE` that delegates to a top-level `runX` function using the `withStore(func(*store.Store) error)` helper. Keep `RunE` closures trivial; put logic in named functions (this also keeps cognitive complexity down).
- `internal/store/` — the ONLY SQLite access. `modernc.org/sqlite` (no CGO). Repositories return typed models. Dedup articles on `(feed_id, guid)` with `FallbackGUID` (sha256 of url+title). `ErrNotFound` for missing rows.
- `internal/fetch/` — concurrent fetcher (`errgroup` + `SetLimit`), conditional requests (ETag/Last-Modified → 304), byte limits. The **SSRF guard is always on** — never weaken it.
- `internal/api/` — `http.ServeMux` (Go 1.22 method+path patterns). Standard error envelope `{"error":{"code","message"}}` via `writeError`/`writeStoreError`. Handlers stay small.
- `internal/digest/`, `internal/readability/` — digest assembly and reader-mode extraction.

## Rules
- Go 1.25 is the module version; keep it. CGO-free always.
- Every command supports `--json`. The CLI/API/UI share one source of truth (SQLite) — never introduce parallel state.
- Handle or explicitly ignore every error (`_ = x.Close()` on defers). errcheck is enforced.
- Keep functions under SonarGo's cognitive-complexity threshold of 15 — extract helpers rather than nesting. `withStore` closures count toward the enclosing function's complexity, so move real logic into named functions.
- After changes: `gofmt -w`, `go build ./...`, `go vet ./...`, `go test ./...`. Add/adjust tests for new behavior.
- Prefer the `sonar-gatekeeper` agent's checks (golangci-lint v2, gocognit) before considering work done.

Do not touch the UI (`ui/`) or the SKILL.md unless the task explicitly spans them.
