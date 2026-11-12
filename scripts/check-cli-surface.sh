#!/usr/bin/env bash
# T-139: checks that the worker CLIs still accept what Messenger passes them,
# and reports a version newer than the one recorded in
# internal/diagnostics/tested.go. No model is called: only --version,
# --help and `codex features list`.
#
# Usage: CLAUDE=path CODEX=path CURSOR=path scripts/check-cli-surface.sh
# A missing CLI variable skips that CLI. Exit 1 when a flag or feature
# Messenger uses is gone; a newer version alone is a warning.
set -u
root="$(cd "$(dirname "$0")/.." && pwd)"
summary="${GITHUB_STEP_SUMMARY:-/dev/null}"
failed=0

tested() { sed -n "s/^[[:space:]]*\"$1\":[[:space:]]*\"\([^\"]*\)\".*/\1/p" "$root/internal/diagnostics/tested.go" | head -1; }
note() { echo "$1"; echo "$1" >>"$summary"; }
fail() { note "- ❌ $1"; failed=1; }

# needs NAME HELP FLAG... : every flag must appear in the help text.
needs() {
  local name="$1" help="$2"; shift 2
  local flag
  for flag in "$@"; do
    if ! grep -qE -- "(^|[^[:alnum:]-])${flag}([^[:alnum:]-]|$)" <<<"$help"; then
      fail "$name: \`$flag\` is no longer in its help"
    fi
  done
}

version_line() {
  local name="$1" provider="$2" got="$3" want
  want="$(tested "$provider")"
  if [ -z "$want" ]; then
    note "- $name $got (no checked version recorded)"
  elif [ "$got" = "$want" ]; then
    note "- $name $got (the checked version)"
  else
    note "- ⚠️ $name $got, checked version $want: check it and raise \`TestedCLI\` in internal/diagnostics/tested.go"
    echo "::warning::$name $got is not the checked version $want"
  fi
}

note "## Worker CLIs"

if [ -n "${CLAUDE:-}" ]; then
  v="$("$CLAUDE" --version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1)"
  version_line "Claude Code" claude "${v:-unknown}"
  help="$("$CLAUDE" --help 2>&1)"
  # internal/adapters/tasks.go (claudeTaskArgs), stream.go, claude_coordinator.go
  needs "claude" "$help" -p --output-format --input-format --verbose --mcp-config --strict-mcp-config \
    --tools --setting-sources --permission-mode --model --allowedTools --resume --session-id \
    --settings --disable-slash-commands --no-chrome --dangerously-skip-permissions
fi

if [ -n "${CODEX:-}" ]; then
  v="$("$CODEX" --version 2>/dev/null | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' | head -1)"
  version_line "Codex" codex "${v:-unknown}"
  # internal/adapters/tasks.go (codexTaskArgs)
  needs "codex exec" "$("$CODEX" exec --help 2>&1)" --json --skip-git-repo-check --ignore-user-config \
    --sandbox --disable --dangerously-bypass-approvals-and-sandbox -c
  "$CODEX" exec resume --help >/dev/null 2>&1 || fail "codex: \`exec resume\` is gone"
  "$CODEX" app-server --help >/dev/null 2>&1 || fail "codex: \`app-server\` is gone"
  features="$(HOME="$(mktemp -d)" "$CODEX" features list 2>/dev/null)"
  for f in shell_tool apps plugins hooks multi_agent browser_use computer_use; do
    line="$(grep -E "^$f[[:space:]]" <<<"$features")"
    if [ -z "$line" ]; then
      fail "codex: feature \`$f\` (disabled for read-only tasks) is no longer listed"
    elif grep -q removed <<<"$line"; then
      note "- ⚠️ codex: feature \`$f\` is marked removed"
    fi
  done
fi

if [ -n "${CURSOR:-}" ]; then
  v="$("$CURSOR" --version 2>/dev/null | grep -oE '[0-9]{4}\.[0-9]+\.[0-9]+(-[0-9a-f]+)?' | head -1)"
  version_line "Cursor" cursor "${v:-unknown}"
  # internal/adapters/cursor_tasks.go (cursorTaskArgs)
  needs "cursor" "$("$CURSOR" --help 2>&1)" -p --output-format --trust --approve-mcps --sandbox --force --model --resume
fi

[ "$failed" = 0 ] && note "All flags and features Messenger uses are present."
exit "$failed"
