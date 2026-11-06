# T-023 wait pilot: Claude app on the Mac

Date: 2026-11-06. Branch `claude/vigilant-darwin-34d88b`, pilot at `0ae9182` ([experiments/wait_probe](../../experiments/wait_probe/README.md)). Run by the owner on a MacBook Pro (arm64), guided from a cloud session. Go 1.27.3 was downloaded into `~/AgentMessengerTest/go-sdk` only to build the probe.

Question: can a coordinator chat in the Claude desktop app wait for worker reports through a blocking MCP tool, wake by itself when a report arrives, and keep going? ([RETHINK.md](../RETHINK.md) version 3, [review](../RETHINK_REVIEW_V3.md) section 5, first slice.)

## Setup

- A new local session in the Claude app, Code tab, environment "This computer", folder `~/AgentMessengerTest/pilot-coordinator`. `pwd` in the chat confirmed the folder.
- Two earlier attempts ran in a cloud environment and could not see the probe; they were closed. Only test sessions were used; no working chat was touched.
- The project `.mcp.json` written by `setup.sh` was **not** loaded by the Code tab (`/mcp` showed only cloud connectors, and no approval prompt appeared). The server was added with `claude mcp add --scope local wait_probe -- ~/AgentMessengerTest/bin/wait-probe serve`, run in the test folder; this stores it in `~/.claude.json` for that folder only. `claude mcp list` showed it connected, and the chat confirmed the `wait` and `status` tools. With this route the `.mcp.json` `timeout` was not in effect; Claude's defaults applied.
- Claude app and Claude Code versions were not recorded.
- Prompt: "You are the coordinator. Work in a loop: call wait; when it returns, briefly tell me what arrived and call wait again with the new cursor. Do not end the answer; keep waiting until I say stop." (in Russian).
- Events were sent from Terminal with `wait-probe send --delay N`.

## Server log (`wait-probe log`)

```
21:18:37  server_start
21:20:11  server_start
21:22:07  server_start
21:22:34  wait #2 started, after=0 (given: true), limit 600s, progress: true
21:22:34  wait #2 woke with 2 event(s) after 0s, cursor=2
21:22:36  wait #3 started, after=2 (given: true), limit 600s, progress: true
21:27:41  wait #3 woke with 1 event(s) after 305s, cursor=3
21:27:47  wait #4 started, after=3 (given: true), limit 600s, progress: true
21:32:42  wait #4 woke with 1 event(s) after 295s, cursor=4
21:32:45  wait #5 started, after=4 (given: true), limit 600s, progress: true
21:34:04  wait #5 cancelled by the client after 79s

wait calls: 4; returned with events: 3; empty timeouts (model woke for nothing): 0; cancelled: 1; server starts: 3
```

The first two server starts are `claude mcp list` health checks and an earlier session; the third belongs to the test chat.

## Results

| # | Scenario | Result |
|---|---|---|
| 1 | Events sent before waiting (A, B) | Returned by the first `wait` at once; nothing lost. |
| 2 | Long wait, event after 5 min (C) | The call stayed in the foreground for 5 min ("Using: wait probe: wait 2 min 59 s"); it did **not** move to the background at 2 min. The coordinator woke without owner input, reported C in one line and called `wait` again. |
| 3 | Owner types while waiting ("All fine?") | The app moved the running `wait` to a background task; the coordinator answered. When the background call returned the next event (second C, at 21:32), the coordinator **woke by itself**, reported it (noting it was a repeat) and called `wait` again. Same call, `wait #4`, across the move to background. |
| 4 | Stop | "stop" cancelled the wait (`wait #5 cancelled`). |
| — | Cursor | The model passed `after` every time; no event was returned twice or skipped. |
| — | Progress | Claude sent a progress token on every call; the server sent progress every 60 s. |

Model turns spent on waiting: one per real event, **no empty wake-ups** in this run (events came before the 600 s default limit).

## Not verified

- An empty timeout in the app (nothing for 10+ min), and waits longer than 30 min.
- Closing and reopening the app, sleep, client restart during a wait.
- The project `.mcp.json` route and its `timeout` setting in the Code tab.
- Codex (app and CLI). Scenario 7 of the README is still open.
- Real workers, plan approval, ask/answer: later steps of the review's first slice.
- Token usage was not measured directly; only the number of wake-ups.

## Conclusion

In the Claude desktop app, a blocking MCP `wait` works as the coordinator's way to wait for workers: it waits for minutes, wakes the coordinator by itself on a report, survives the owner talking to the coordinator in between, and loses nothing. The main open risk of RETHINK version 3 for a Claude coordinator is answered positively. Codex remains to be checked.
