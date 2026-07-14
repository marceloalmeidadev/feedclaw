# FeedClaw × OpenClaw — integration reference

> Source of truth for how FeedClaw plugs into OpenClaw. Verified against
> **OpenClaw v2026.7.1** on 2026-07-14. Where the CLI/docs and this document
> disagreed, the CLI won and this reflects the CLI.

## 1. What was verified (v2026.7.1)

- **Skills** install from a local directory: `openclaw skills install <dir> --as <slug> --global`. A skill is a `SKILL.md` with frontmatter `name`/`description` plus a `metadata.openclaw` block (`emoji`, `requires.bins`, `install`). `openclaw skills check` validates `requires.bins` against `$PATH`.
- **`openclaw cron add`** supports `--cron`/`--every`/`--at` schedules, agent payloads (`--message`, `--agent`, `--session`, `--thinking`, `--model`), command payloads (`--command`), delivery (`--announce`/`--no-deliver`/`--channel`), and **event triggers**: `--on-exit <shell>` ("fire once when this watched command exits; survives turn teardown", `--on-exit-cwd`), `--trigger-script <path|->` (a condition script) and `--trigger-once`.
- **`--trigger-script`** runs headlessly and must return a JSON object `{ fire, message?, state? }`; `fire` decides whether the payload runs, and `trigger.state` persists across evaluations. **What the docs do NOT specify:** how a watched command's *exit code / stdout* is handed to the trigger-script or to the agent turn. No env-var/arg/stdin contract is documented for that hand-off.
- **`utilityModel`** exists: `agents.defaults.model.utilityModel` (sibling of `primary`, currently unset). Set with `openclaw config set agents.defaults.model.utilityModel "<provider/model|alias>"`. It powers "short internal tasks".
- **Sessions** can be targeted (`--session`, `--session-key`) and are first-class in the Control UI (pin/rename/archive), with auto-generated titles.

## 2. Design decision — bridge the exit code via the report file, not the trigger

Because the exit-code → trigger-script → agent data-flow is **undocumented**, we do **not** rely on OpenClaw passing the `fetch` exit code into the agent turn. Instead we bridge it with an artifact FeedClaw fully controls:

- `feedclaw fetch --report <path>` always writes `last_run.json` (schema in the engine; includes `exit_code`, `articles_new`, `failed_feeds`, `unread_total`).
- The **`feedclaw-digest` skill's first instruction** is to read that report's `exit_code` and **abort on `10`/`30`/`40`/`50`**, proceeding only on `0`/`20`.

This is correct whether or not `--on-exit`/`--trigger-script` filters by exit code. If the trigger *can* be made to fire only on `0`/`20`, that is a pure optimization (the LLM is never woken on a quiet day); if it cannot, the skill still aborts cheaply on the first step. **No invented flags.**

### Exit codes (the contract, from `feedclaw fetch`)

| code | meaning | digest flow |
|---|---|---|
| `0`  | new unread articles | proceed |
| `10` | nothing new | abort (do not cluster) |
| `20` | partial: some feeds failed, but new articles | proceed; mention failed feeds |
| `30` | total network failure | abort; notify user |
| `40` | config / DB error | abort; notify; suggest `feedclaw doctor` |
| `50` | another fetch already running | abort silently |

## 3. The daily pipeline (two halves)

**Half A — deterministic fetch (no LLM), on schedule.** A command cron job:

```sh
openclaw cron add --name feedclaw-fetch \
  --cron "0 7 * * *" \
  --command "feedclaw fetch --report \"$HOME/.config/feedclaw/last_run.json\""
```

**Half B — the digest agent, woken when the fetch exits.** An event-triggered agent job that watches the fetch and fires the `feedclaw-digest` skill:

```sh
openclaw cron add --name feedclaw-digest \
  --on-exit "feedclaw fetch --report \"$HOME/.config/feedclaw/last_run.json\"" \
  --agent main \
  --session-key "agent:main:feedclaw-digest" \
  --thinking medium \
  --no-deliver \
  --message "Execute o fluxo do skill 'feedclaw-digest' lendo o relatório em $HOME/.config/feedclaw/last_run.json."
```

If a single job can both schedule the fetch and fire on its exit in your setup, collapse the two into one `--cron … --on-exit …` job. Either way, **the digest skill re-reads the report and self-gates**, so correctness does not depend on the exact trigger wiring. Add `--trigger-script` later to fire only on `exit_code` `0`/`20` as an optimization.

Deliver-to-chat is off for now (`--no-deliver`); the user reads the digest via `feedclaw digest show` or the UI. To deliver later: `--announce --channel <id>`.

## 4. Model tiering (`utilityModel`)

Clustering 30–60 titles into 4–8 themes with short summaries is classification, not deep reasoning — route it to the utility model; keep reading/drill-down on the primary. The plugin **hardcodes no model**; it only asks the agent to prefer the utility tier for clustering. Configure once:

```sh
openclaw config set agents.defaults.model.utilityModel "anthropic/claude-haiku-4-5"
```

`digest save --model-note` records which model produced each digest, so theme quality can be compared across tiers. **Validate the grouping quality** before trusting the cheaper tier; if themes come out generic, promote clustering back to the primary model.

## 5. Dedicated session

Bind the digest to a fixed, named session (`--session-key "agent:main:feedclaw-digest"`) so it always lands in the same Control-UI session instead of spawning a new auto-titled one each day.

## 6. Prerequisite — operator scope

Creating cron jobs requires the local CLI device to hold `operator.write`/`admin`/`pairing`. Under token auth the read-only CLI cannot self-approve; grant the upgrade from an admin context (`openclaw dashboard`, or an already-admin device) — see the deferred step in the install notes. Until then, the digest is run manually/conversationally (`feedclaw` skill is `Ready`).
