#!/usr/bin/env bash
# Builds wait-probe and prepares a separate test folder for the coordinator pilot.
# Touches nothing outside ~/AgentMessengerTest: no global Claude or Codex settings.
#   bash experiments/wait_probe/setup.sh               Claude default (long calls go to background)
#   bash experiments/wait_probe/setup.sh --foreground  Claude keeps the call in the foreground
set -euo pipefail

repo="$(cd "$(dirname "$0")/../.." && pwd)"
base="$HOME/AgentMessengerTest"
bin="$base/bin/wait-probe"
work="$base/pilot-coordinator"

mkdir -p "$base/bin" "$work/.codex"
(cd "$repo" && go build -o "$bin" ./experiments/wait_probe)

# Claude Code: project MCP server; timeout is in milliseconds (70 minutes).
cat >"$work/.mcp.json" <<EOF
{
  "mcpServers": {
    "wait_probe": {
      "command": "$bin",
      "args": ["serve"],
      "timeout": 4200000
    }
  }
}
EOF

# Codex: project config, read only when the folder is trusted.
cat >"$work/.codex/config.toml" <<EOF
[mcp_servers.wait_probe]
command = "$bin"
args = ["serve"]
startup_timeout_sec = 20
tool_timeout_sec = 3600
EOF

if [[ "${1:-}" == "--foreground" ]]; then
  mkdir -p "$work/.claude"
  cat >"$work/.claude/settings.json" <<'EOF'
{
  "env": { "CLAUDE_CODE_MCP_AUTO_BACKGROUND_MS": "0" }
}
EOF
  mode="foreground"
else
  rm -f "$work/.claude/settings.json"
  mode="background (Claude default)"
fi

echo "wait-probe: $bin"
echo "test folder: $work"
echo "Claude mode: $mode"
echo "send an event: $bin send \"Worker A: done\""
echo "see the log:   $bin log"
