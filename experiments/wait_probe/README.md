# Coordinator wait pilot

Pilot for [RETHINK.md](../../docs/RETHINK.md) version 3, step 1 of the review's first slice ([RETHINK_REVIEW_V3.md](../../docs/RETHINK_REVIEW_V3.md), section 5). It answers one question before any product work: **can a coordinator chat in the Claude or Codex app wait for worker reports through a blocking MCP tool, and how does it behave in practice?**

It is not part of the product. It launches no agents and changes no Messenger data. Events are typed by the owner from a terminal, standing in for worker reports.

## What it is

`wait-probe serve` is a stdio MCP server with two tools:

- `wait(after, timeout_seconds)` blocks until there is an event after the cursor `after`, or until the timeout (default 600 s, at most 3600 s). It returns the new events and a new cursor. While it blocks, the model generates nothing. If the client sent a progress token, the server sends a progress notification every 60 s so that the client does not treat the call as idle; progress does not wake the model.
- `status(after)` returns the cursor and the number of new events without waiting.

The server never moves the cursor on its own. A caller that does not pass the previous cursor gets the same events again, so an interrupted `wait` loses nothing.

`wait-probe send TEXT` appends an event. `wait-probe log` prints every call the server saw and a summary: how many waits returned events, how many timed out empty (each is a model turn for nothing), how many were cancelled.

Data lives in `~/AgentMessengerTest/wait-probe` (`events.jsonl`, `calls.jsonl`). Temporary folders are refused.

## Verified here

On Linux, without models: `go test -race ./experiments/wait_probe` covers the MCP handshake, wake on an event within the poll interval, empty timeout, other calls answered while `wait` blocks, cancellation (no answer, no lost event, replay without a cursor), progress notifications, event numbering and the refusal of temporary folders. A manual run through the built binary woke 2 s after `send`.

Not verified here: any real Claude or Codex client. That is what the Mac run is for.

**Claude app on the Mac, 2026-11-06:** passed. The coordinator waited for 5 minutes in the foreground, woke by itself on each event, kept waiting after the owner wrote in between, and lost nothing; no empty wake-ups. [Report](../../docs/reports/2026-11-06-wait-pilot-claude-mac.md). Codex is still open.

## Client settings used

From the official Claude Code documentation ([MCP](https://code.claude.com/docs/en/mcp), [environment variables](https://code.claude.com/docs/en/env-vars), [desktop](https://code.claude.com/docs/en/desktop)), checked 2026-11-06:

- Per-server `timeout` in `.mcp.json`, in milliseconds; global default `MCP_TOOL_TIMEOUT` about 28 hours.
- A stdio call with no response and no progress for 30 minutes is aborted (`CLAUDE_CODE_MCP_TOOL_IDLE_TIMEOUT`).
- A call running longer than 2 minutes moves to a background task (`CLAUDE_CODE_MCP_AUTO_BACKGROUND_MS`, default 120000; `0` keeps it in the foreground). Its result arrives as a task notification. Whether that notification starts a turn in an idle chat is **not documented**; this is the main thing to observe.
- Local Code tab sessions load project `.mcp.json`; project servers need approval.

Codex: `tool_timeout_sec` (default 60 s) and `startup_timeout_sec` in `[mcp_servers.<name>]`, and a project `.codex/config.toml` that is read only for trusted projects. The Codex documentation could not be fetched from the cloud session; these keys come from search results and the [first review](../../docs/RETHINK_REVIEW_V3.md). Check them against your installed Codex.

`setup.sh` writes these settings only into `~/AgentMessengerTest/pilot-coordinator`. Global Claude and Codex settings are not changed.

## Mac run

Use a **new test chat** in the test folder only, never a working chat. Each wake-up is a real model turn.

```bash
bash experiments/wait_probe/setup.sh
```

The Claude app's Code tab did not load the project `.mcp.json` in the first run. Add the server for this folder only:

```bash
cd ~/AgentMessengerTest/pilot-coordinator
claude mcp add --scope local wait_probe -- ~/AgentMessengerTest/bin/wait-probe serve
claude mcp list
```

Open `~/AgentMessengerTest/pilot-coordinator` as a new local session in the Claude app (Code tab, environment "This computer", not a cloud environment), check with `pwd` in the chat that it runs in that folder, ask whether the `wait` and `status` tools are available, and write: "Call status, then wait for events with wait and tell me about each one." Then, in Terminal, run the scenarios and note what the chat shows:

| # | Scenario | Command | What to note |
|---|---|---|---|
| 1 | Short wait | `~/AgentMessengerTest/bin/wait-probe send --delay 30 "Worker A: done"` | Did the chat report it by itself? |
| 2 | Long wait (background) | `... send --delay 300 "Worker B: tests passed"` | After 2 min, is the chat free? Did the event wake it **without you typing**? |
| 3 | Typing while waiting | During a wait, write "what are you doing?" | Can you type? Does waiting continue afterwards? |
| 4 | Interrupt | Stop the wait (Esc or stop), then `send "Worker C: question"`, then write "keep waiting" | Is the event delivered after the restart (not lost)? |
| 5 | Nothing happens | Leave it for 20–30 min | `wait-probe log`: empty timeouts = turns spent for nothing |
| 6 | Foreground | `bash experiments/wait_probe/setup.sh --foreground`, a **new** chat, repeat 2 and 3 | Compare with background |
| 7 | Codex | Open the folder in Codex (app, then CLI), trust it, repeat 1, 4, 5 | Does a wait longer than 60 s survive? |

After each client, save the output of:

```bash
~/AgentMessengerTest/bin/wait-probe log
```

Send back the log and a line per scenario. Do not send chat transcripts or session IDs.

Cleanup removes everything the pilot created:

```bash
cd ~/AgentMessengerTest/pilot-coordinator && claude mcp remove --scope local wait_probe; cd ~
```

```bash
rm -rf ~/AgentMessengerTest/pilot-coordinator ~/AgentMessengerTest/wait-probe ~/AgentMessengerTest/bin/wait-probe
```
