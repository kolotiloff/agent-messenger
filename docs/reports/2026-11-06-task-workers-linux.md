# Task workers on real CLIs with fake models (T-024–T-026)

Date: 2026-11-06 (night), cloud session on Linux x86-64, Go 1.24.7. No real model was called. The real CLIs talked to local HTTP servers that imitate the model APIs (fixed answers, not models). Data was kept in `~/AgentMessengerTest/night`, not in a temporary folder.

| CLI | Version | Fake API |
|---|---|---|
| Codex | 0.155.1 (npm) | Responses API (`model_providers` in a separate `CODEX_HOME`) |
| Claude Code | 2.1.319 (npm) | Anthropic Messages API (`ANTHROPIC_BASE_URL`, dummy key) |

The client ran as `messenger serve --allow-tasks`. The coordinator was `messenger coordinator --project Probe`, driven over stdio. The worker profile was `worker` (D-023).

## Codex

| Step | Result |
|---|---|
| `start_task` → the client runs `codex exec --json --sandbox workspace-write -c mcp_servers.messenger_task…` in the project folder | Codex emitted `thread.started`; the ID was bound once |
| Tools the model saw | Codex's normal tools (`exec_command`, `write_stdin`, …) plus the namespace `mcp__messenger_task` with `read_task`, `report`, `ask`, `ask_owner` |
| `read_task` → `report` | Verified: exit 0, empty group, `turn.completed`, one report. `wait` returned the report. The task is `done`. |
| `read_task` → `ask` | The task became `waiting_coordinator`, and `wait` returned the question |
| `answer` → next run `codex exec … resume <ID>` | Same thread. `read_task` returned the answer and the question (run 2). |

## Claude Code

| Step | Result |
|---|---|
| `start_task` → `claude -p … --session-id <UUID> …` in the project folder | Claude used the recorded UUID; identity verified |
| Default MCP loading | Claude 2.1.319 **defers MCP tools behind `ToolSearch`**: the first request listed only built-in tools. The fake model did not search, so no report was made, and the task was correctly marked `failed` (exit 0, identity and provider fine, no disposition). |
| With `ENABLE_TOOL_SEARCH=false` in the client's environment | The model saw the `mcp__messenger_task__*` tools. `read_task` → `report` was verified, and `wait` returned the report. |

A real model is expected to find the deferred tools through `ToolSearch`, because the prompt names them. This was **not verified**. It is recorded as Q-010 for the acceptance run (T-027); the product keeps Claude's default.

## Not verified

- Real models and the Claude ask → `--resume` round with a real CLI. The Claude resume flags were checked only against `--help`.
- A worker that does real edits, denied permissions, partial work.
- macOS (CI only).
