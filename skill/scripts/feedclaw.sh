#!/usr/bin/env bash
# feedclaw.sh — thin wrapper the OpenClaw agent calls to run the FeedClaw engine.
#
# Resolution order for the binary:
#   1. $FEEDCLAW_BIN, if set and executable
#   2. the binary bundled next to the skill (../feedclaw, relative to this script)
#   3. `feedclaw` on $PATH
#
# All arguments are forwarded verbatim, so the agent uses it exactly like the
# `feedclaw` CLI (always with --json).
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

bin="${FEEDCLAW_BIN:-}"
if [[ -z "$bin" || ! -x "$bin" ]]; then
  if [[ -x "$script_dir/../feedclaw" ]]; then
    bin="$script_dir/../feedclaw"
  elif command -v feedclaw >/dev/null 2>&1; then
    bin="$(command -v feedclaw)"
  else
    echo "feedclaw: binary not found (set FEEDCLAW_BIN, bundle it next to the skill, or add it to PATH)" >&2
    exit 127
  fi
fi

exec "$bin" "$@"
