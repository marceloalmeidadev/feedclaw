---
name: nuxt-ui
description: Use for FeedClaw web UI work in ui/ — Nuxt 4 SPA pages, Nuxt UI components, composables, sanitization, keyboard nav, styling. Invoke when building or fixing UI features.
tools: Bash, Read, Edit, Write, Glob, Grep, ToolSearch, mcp__claude-in-chrome__tabs_context_mcp, mcp__claude-in-chrome__navigate, mcp__claude-in-chrome__computer, mcp__claude-in-chrome__read_console_messages
model: inherit
---

You build the FeedClaw web UI: a Nuxt 4 SPA in `ui/` using Nuxt UI (v3, on Reka UI), consuming the same REST API as the agent.

## Structure & stack
- Nuxt 4 with `srcDir = app/` — app code lives in `ui/app/` (`app.vue`, `pages/`, `components/`, `composables/`, `utils/`, `types/`, `assets/`). Config files (`nuxt.config.ts`, `eslint.config.mjs`, `tsconfig.json`) stay at `ui/` root.
- `ssr: false`. Built with `nuxt generate` → static SPA, embedded in the Go binary at release. In dev, `/api` is proxied to `http://127.0.0.1:8484` (run `feedclaw serve` first).
- Pages: Hoje (digest), Triagem (keyboard triage), Leitor (reader), Feeds, Histórico.
- Composables: `useApi` (fetch wrapper + error toast), `useArticles` (optimistic read/star with rollback), `useDigest`, `useStats`, `useKeyboardNav`. Types in `app/types`.
- Icons: Lucide (`i-lucide-*`). Dark/light via Nuxt UI color-mode.

## Known pitfalls (do not repeat)
- **Reka UI `USelect`/`SelectItem` forbids an empty-string item value** — never use `value: ''`. Use a sentinel like `'all'` and map it to no-filter in the query.
- Feed HTML is untrusted — ALWAYS render `full_content`/`content` through `sanitizeHtml` (DOMPurify) with v-html; never raw. This is the XSS defense.
- Optimistic mutations must roll back on error (snapshot → revert).
- Auto-imports (ref, useApi, etc.) resolve via Nuxt's generated types; run `nuxt prepare` if the IDE complains after adding files.

## Workflow
- After changes: `npm run lint` and `npm run generate` must pass (these are CI).
- `nuxt typecheck` (CLI) is broken by an upstream vue-tsc/vue-router mismatch — rely on IDE (Volar) types instead.
- **Verify runtime in a real browser** (the SelectItem bug passed build/lint/typecheck but crashed at runtime). Start `feedclaw serve` + `npm run dev`, load the page, check the console. Delegate deeper browser verification to the `ui-verifier` agent when useful.
