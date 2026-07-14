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

## 3. Install as an OpenClaw skill

Copy the bundle into your OpenClaw skills directory:

```sh
mkdir -p ~/.openclaw/skills
cp -r dist/feedclaw ~/.openclaw/skills/feedclaw
```

The skill's wrapper (`scripts/feedclaw.sh`) resolves the binary sitting next to
it, so no `PATH` changes are needed. To use `feedclaw` directly from a shell,
optionally symlink it:

```sh
ln -sf ~/.openclaw/skills/feedclaw/feedclaw ~/.local/bin/feedclaw
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

```sh
openclaw cron add --name feedclaw-digest --schedule "0 7 * * *" \
  --task "Execute o fluxo de digest diário do FeedClaw conforme o SKILL.md"
```

## Notes

- The API and UI bind **only to 127.0.0.1**. Remote access is out of scope for v1.
- Upgrading: replace the `feedclaw/` directory; the SQLite schema migrates
  automatically on next run.
