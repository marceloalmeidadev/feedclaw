# Installing FeedClaw

FeedClaw ships as an OpenClaw **bundle plugin**: a single Go binary (with the web
UI embedded) plus the agent skill. Everything runs locally; state lives in one
SQLite file.

## 1. Get the bundle

Download the release tarball (`feedclaw-<version>.tar.gz`) and its `SHA256SUMS`,
or build it yourself:

```sh
git clone https://github.com/marceloalmeidadev/feedclaw
cd feedclaw
make package          # builds UI + binary, assembles dist/feedclaw/, writes checksums
```

The bundle (`dist/feedclaw/`) contains:

```
feedclaw/
├── feedclaw                     # the binary (UI embedded)
├── skills/
│   ├── feedclaw/SKILL.md        # conversational skill
│   └── feedclaw-digest/SKILL.md # daily digest skill
├── scripts/feedclaw.sh
└── INSTALL.md
```

## 2. Verify the checksum

```sh
cd dist
sha256sum -c SHA256SUMS
```

## 3. Install the skills into OpenClaw

FeedClaw ships **two** skills: `feedclaw` (conversational) and `feedclaw-digest`
(the daily digest flow). Put the `feedclaw` binary on your `PATH` (the skills
declare `requires.bins: [feedclaw]`), then install both skills:

```sh
# binary on PATH
install -m755 dist/feedclaw/feedclaw ~/.local/bin/feedclaw

# both skills
openclaw skills install dist/feedclaw/skills/feedclaw        --as feedclaw        --global
openclaw skills install dist/feedclaw/skills/feedclaw-digest --as feedclaw-digest --global
openclaw skills check          # both should be "Ready"
```

## 4. First run

```sh
feedclaw import --opml <your-feedly-export.opml>   # or: feedclaw feeds add <url>
feedclaw fetch
feedclaw serve                                     # http://127.0.0.1:8484 (UI + API)
```

The database is created at `$XDG_CONFIG_HOME/feedclaw/feedclaw.db`
(override with `--db` or `FEEDCLAW_DB`).

## 5. Daily digest via cron

Schedule the digest as an OpenClaw cron agent job. Each run runs `feedclaw fetch`
and only clusters/saves when there is something new — the `feedclaw-digest` skill
reads the fetch report's `exit_code` (`0`/`20` proceed; `10`/`30`/`40`/`50`
abort), so the model does no work on a quiet day:

```sh
openclaw cron add --name feedclaw-digest --cron "0 7 * * *" \
  --agent main --session-key "agent:main:feedclaw-digest" \
  --thinking medium --no-deliver \
  --message "Rode 'feedclaw fetch', depois siga o skill feedclaw-digest."

openclaw cron run feedclaw-digest    # test now; then: feedclaw digest show
```

Notes:

- Route clustering to a cheaper tier: `openclaw config set agents.defaults.model.utilityModel "<provider/model>"`.
- To deliver the digest to a chat, add `--announce --channel <id>` (default is `--no-deliver`; read it via `feedclaw digest show` or the UI).
- If `openclaw cron add` reports a pending scope upgrade, approve it in the Control UI (`openclaw dashboard` → Devices) or from an already-admin device, then re-run.
- `--cron` and `--on-exit` are mutually exclusive and `--on-exit` is one-shot, so the digest self-gates on the report's exit code rather than being wired as a two-job on-exit pipeline.

## Notes

- The API and UI bind **only to 127.0.0.1**. Remote access is out of scope for v1.
- Upgrading: replace the `feedclaw/` directory; the SQLite schema migrates
  automatically on next run.
