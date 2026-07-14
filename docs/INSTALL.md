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
├── feedclaw          # the binary (UI embedded)
├── SKILL.md          # the OpenClaw skill
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
(the on-exit daily flow). Put the `feedclaw` binary on your `PATH` (the skills
declare `requires.bins: [feedclaw]`), then install both skills:

```sh
# binary on PATH
install -m755 dist/feedclaw/feedclaw ~/.local/bin/feedclaw

# both skills
openclaw skills install dist/feedclaw/feedclaw        --as feedclaw        --global
openclaw skills install dist/feedclaw/feedclaw-digest --as feedclaw-digest --global
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

## 5. Daily digest via cron (on-exit pipeline)

The daily digest runs as two halves: a scheduled, deterministic `feedclaw fetch`
(no LLM), and an agent turn woken when it exits — which reads the run report and
only clusters/saves when there is something new. The exact cron commands, the
exit-code contract, `utilityModel` tiering and the operator-scope prerequisite
are documented in **[`openclaw-integration.md`](openclaw-integration.md)**.

Creating cron jobs requires the local CLI to hold `operator.write` — until you
grant that (`openclaw dashboard`), run the digest manually: ask the agent to
"gerar o digest de hoje", or invoke the `feedclaw-digest` flow directly.

## Notes

- The API and UI bind **only to 127.0.0.1**. Remote access is out of scope for v1.
- Upgrading: replace the `feedclaw/` directory; the SQLite schema migrates
  automatically on next run.
