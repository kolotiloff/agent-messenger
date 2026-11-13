# Documentation map

Start here. Rules for coding agents are in [AGENTS.md](../AGENTS.md).

## Working documents

These change all the time. Every tool reads them at the start and updates them at the end. Together they replace a shared memory between Claude Code, Cursor and Codex.

| File | What it holds | When to update it |
|---|---|---|
| [HANDOFF.md](HANDOFF.md) | Current state: what is on `main`, what is in progress on which branch, what is next | At the end of every session |
| [TASKS.md](TASKS.md) | Task list with stable IDs: Now / Next / Backlog / Done | When a task starts, finishes or is found |
| [DECISIONS.md](DECISIONS.md) | Decisions already made, with links to the reasoning | When something is decided; never silently reverse an entry |
| [QUESTIONS.md](QUESTIONS.md) | Questions for the owner and open discussions | Instead of guessing; when an answer arrives |
| [LOG.md](LOG.md) | Short work log, one entry per session | At the end of every session |

## Using the program

| File | About |
|---|---|
| [DOWNLOAD.md](DOWNLOAD.md) | Download buttons and how to start the program on Mac, Windows and Linux |
| [TASK_MODEL.md](TASK_MODEL.md) | How it works: projects, tasks, workers, the coordinator's tools, the window |
| [BROKER.md](BROKER.md) | The server and the network of computers: hosting, invitations, joining, what travels between computers |
| [MAC_APP.md](MAC_APP.md) | The Mac app: installation, the window, the menu |
| [WINDOWS_LINUX.md](WINDOWS_LINUX.md) | Windows and Linux: the window, agents, job objects, what is verified |
| [UPDATES.md](UPDATES.md) | Updates on Mac, Windows and Linux, and of another computer of the network |
| [CLI_CHECK.md](CLI_CHECK.md) | Checking an agent's CLI version and sign-in; the «не проверена» mark |
| [DEPLOYMENT.md](DEPLOYMENT.md) | Where the server runs: your own computer or, later, a VPS (partly a plan) |

## Plans

| File | About |
|---|---|
| [RELEASE_PLAN.md](RELEASE_PLAN.md) | Current status and the stages to the first release |
| [UI_STYLE.md](UI_STYLE.md) | Window style rules: spacing, radii, sizes, components |

## Building and checks

| File | About |
|---|---|
| [CI.md](CI.md) | Automated checks in GitHub Actions, the daily CLI watch, releases |
| [INSTALL.md](INSTALL.md) | The command-line archive for developers |
| [BACKGROUND.md](BACKGROUND.md) | The background client and reconnecting to it |
| [DIAGNOSTICS.md](DIAGNOSTICS.md) | `doctor`: diagnostics without launching a CLI |
| [CLIENT.md](CLIENT.md) | The local Go client: the original decision and its history |

## History

Kept for the reasoning behind decisions. They describe the program as it was on their date, not as it is.

| File | About |
|---|---|
| [PLAN.md](PLAN.md) | The original architecture and working plan v0.3 |
| [MVP.md](MVP.md) | The staged MVP of November 5 |
| [APP.md](APP.md) | The original requirements for the app, installation and window |
| [RETHINK.md](RETHINK.md) | One task, one worker session; the coordinator in the owner's chat. Accepted as D-021 to D-023 |
| [RETHINK_REVIEW.md](RETHINK_REVIEW.md), [RETHINK_REVIEW_V3.md](RETHINK_REVIEW_V3.md) | Reviews of that proposal |
| [VALIDATION.md](VALIDATION.md) | Hypotheses H1–H14 and what the early research confirmed |
| [TOKEN_REVIEW.md](TOKEN_REVIEW.md) | Token use review of 2026-11-07 |
| [GUI_COORDINATOR.md](GUI_COORDINATOR.md) | A coordinator in the graphical Claude app (superseded by D-024) |
| [LOCAL_UI.md](LOCAL_UI.md) | The first browser page of the session and queue model |
| [CLAUDE_STREAM.md](CLAUDE_STREAM.md) | Persistent Claude in one process (old session path) |
| [CODEX_ADAPTER_NEXT.md](CODEX_ADAPTER_NEXT.md) | Persistent Codex research (not wired in) |
| [UI_READ_API.md](UI_READ_API.md) | `overview`: data for the first window |
| [TIMELINE.md](TIMELINE.md) | `timeline`: message history API of the old session path |
| [SESSION_SETUP.md](SESSION_SETUP.md) | Setting up a session from the old window |
| [SCOPED_LAUNCH.md](SCOPED_LAUNCH.md) | Launching only the selected session (preview.19) |
| [ROUTING.md](ROUTING.md) | Replies between agents in the old session path |
| [ACCEPTANCE_CODEX_MAC.md](ACCEPTANCE_CODEX_MAC.md) | Runbook for the first real Codex run on the Mac (T-004) |

## Reports

[reports/](reports/) holds dated acceptance reports: what was run, where, with which versions, and what was and was not verified. Write a new report for every acceptance run on real hardware; do not edit old ones except to fix errors.

The design is described in [design/README.md](../design/README.md). Early research stands were removed from the tree; they are kept in git history: [experiments/](https://github.com/kolotiloff/agent-messenger/tree/41e0e6e7e3f20275d480f6b064a881e7b5f4cf9e/experiments/).
