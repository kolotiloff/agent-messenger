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

## Plans and requirements

| File | About |
|---|---|
| [RELEASE_PLAN.md](RELEASE_PLAN.md) | The roadmap: stages to the first release and their exit conditions |
| [MVP.md](MVP.md) | The staged MVP, first user scenario and acceptance rules |
| [PLAN.md](PLAN.md) | The original architecture and working plan v0.3 |
| [APP.md](APP.md) | Requirements for the app, installation and window |
| [DEPLOYMENT.md](DEPLOYMENT.md) | Where the broker runs: your own computer or a VPS |
| [VALIDATION.md](VALIDATION.md) | Hypotheses H1–H14 and what has been confirmed |
| [AGENTGRAM_REVIEW.md](AGENTGRAM_REVIEW.md) | Comparison with Agentgram and ideas taken from it |
| [RETHINK.md](RETHINK.md) | Proposal under review: one task, one worker session; the coordinator in the owner's app |
| [GUI_COORDINATOR.md](GUI_COORDINATOR.md) | Optional scenario: a coordinator in the graphical Claude app |

## How the parts work

| File | About |
|---|---|
| [CLIENT.md](CLIENT.md) | The local Go client: decision and work plan |
| [CLAUDE_STREAM.md](CLAUDE_STREAM.md) | Persistent Claude in one process |
| [CODEX_ADAPTER_NEXT.md](CODEX_ADAPTER_NEXT.md) | Persistent Codex: research and the components built so far |
| [BACKGROUND.md](BACKGROUND.md) | The background client and reconnecting to it |
| [DIAGNOSTICS.md](DIAGNOSTICS.md) | `doctor`: diagnostics without launching a CLI |
| [CLI_CHECK.md](CLI_CHECK.md) | `check-cli`: explicit version and login check |
| [UI_READ_API.md](UI_READ_API.md) | `overview`: data for the agent cards |
| [TIMELINE.md](TIMELINE.md) | `timeline`: message history API |
| [LOCAL_UI.md](LOCAL_UI.md) | The local browser UI: messages and queue |
| [SESSION_SETUP.md](SESSION_SETUP.md) | Setting up an agent from the window |
| [SCOPED_LAUNCH.md](SCOPED_LAUNCH.md) | Launching only the selected session |
| [ROUTING.md](ROUTING.md) | Replies between agents and the automatic reply limit |

## Install, release and checks

| File | About |
|---|---|
| [MAC_APP.md](MAC_APP.md) | Agent Messenger.app: download and install |
| [INSTALL.md](INSTALL.md) | Installing the client from an archive |
| [UPDATES.md](UPDATES.md) | Mac app updates through Sparkle and GitHub Releases |
| [CI.md](CI.md) | Automated checks in GitHub Actions |
| [ACCEPTANCE_CODEX_MAC.md](ACCEPTANCE_CODEX_MAC.md) | Runbook for the first real Codex run on the Mac (T-004) |

## Reports

[reports/](reports/) holds dated acceptance reports: what was run, where, with which versions, and what was and was not verified. Write a new report for every acceptance run on real hardware; do not edit old ones except to fix errors.

The design is described in [design/README.md](../design/README.md). Early research stands were removed from the tree; they are kept in git history: [experiments/](https://github.com/kolotiloff/agent-messenger/tree/41e0e6e7e3f20275d480f6b064a881e7b5f4cf9e/experiments/).
