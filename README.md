# Agent Messenger

Messaging between Claude Code, Codex, Cursor Agent CLI and other agents. The project's goal is to run on macOS, Windows and Linux, with the broker on a computer or VPS of your choice and the ability to hand off the coordinator role.

**Right now this is a technical preview: accepted on Mac, with first Windows and Linux builds to test; not a finished release.** The product is written in Go. It has a SQLite queue, protected session ownership, a browser window, a first native Agent Messenger.app shell for Mac and a verified local install from an archive. The product source lives in `cmd/messenger` and `internal`, the Mac shell in `desktop/macos`. Early research test rigs were removed from the tree and stay in git history.

## Download the app

All downloads are on the **[releases page](https://github.com/kolotiloff/agent-messenger/releases)**; take the newest preview.

| Platform | What to download | How to start |
|---|---|---|
| macOS 15+, Apple Silicon | the `.zip` of the app (named by its checksum) | [Agent Messenger.app](docs/MAC_APP.md); updates arrive in the app |
| Windows 10/11 | `agent-messenger-…-windows-amd64.zip` (`arm64` for ARM) | unpack, double-click `messenger.exe`; the window opens in the browser |
| Linux | `agent-messenger-…-linux-amd64.tar.gz` (`arm64` for ARM) | unpack, run `./messenger open`; the window opens in the browser |
| macOS, Intel | not built yet | — |

Windows and Linux builds are new in preview.32 and not yet accepted on real computers: see [Windows and Linux](docs/WINDOWS_LINUX.md) for what is verified and what is missing (no self-update, no installer, not signed). Apple Developer ID/notarization is not done either; macOS may block the downloaded preview the first time. Running a prebuilt build does not require Python, Node or Go.

## What has been verified

| Capability | Current evidence |
|---|---|
| Persistent Claude on Mac | Two sequential messages, linked replies, pause/resume, exact continuation of a test session, and explicit recovery after a real Mac reboot |
| Background client | Closing the terminal loses neither the queue nor the live owner. Stopping the client does not by itself release the CLI session |
| Local window | Cards, history, linked replies, sending, pausing and resuming the queue; verified on a separate database without models |
| Launching from the window | Creating a new Claude test session, a separate permission for that session only, two messages in one process, and replies appearing automatically; acceptance with a fake CLI |
| Send recovery | Once the client has written the envelope, fully closing the window and a client crash preserve the address, text and ID. A transport retry does not create a second job |
| Install and update | Native Mac arm64 preview archive, SHA-256 verification, install into a user directory, and updating a stopped client while preserving the queue and the old binary |
| Updating the app from the window | Signed Sparkle channel and package downloads from public GitHub Releases. An already running background client stays on its version; limits and acceptance are described in docs/UPDATES.md |
| Protection | A corrupted/empty/null registry or an ambiguous directory does not allow bypassing the lock. An unknown result is never turned into success |

Persistent Codex/Cursor adapters, full Windows/Linux support, a network broker, coordinator handoff, acceptance of the native shell on other Macs and signed distribution are not finished yet. Early experiments with individual CLIs do not replace their acceptance in the current product. Safely terminating a CLI owner within the same Mac boot is still an open question; installing does not bypass remaining reservations.

## Getting started

To launch with a double click, use [Agent Messenger.app](docs/MAC_APP.md): its own macOS window and a built-in client, with no separate browser or terminal.

In preview.19: **Агенты → Claude Code → Настроить → Создать тестовую сессию** ("Agents → Claude Code → Configure → Create test session"), then **Сессии → Запустить тестовую сессию** ("Sessions → Launch test session"). After you grant explicit permission, you can write in the conversation; replies consume tokens. Existing work chats are not connected automatically. A version 19 background client is required; see [launch and recovery limitations](docs/SCOPED_LAUNCH.md).

[Installing the technical preview on Mac](docs/INSTALL.md) covers archive verification, the `version` command, installation and update limitations. The prebuilt binary does not need Python or Node. The public preview contains the native Mac app. Gatekeeper acceptance on a clean, unfamiliar Mac remains a separate stage; local ad-hoc signing does not replace Developer ID/notarization.

To build from source and run checks without models, use the [local client guide](docs/CLIENT.md) and [the same scenarios as in CI](docs/CI.md). A first trial needs separate test data, not work/GUI chats. If a client or a protective reservation already exists, do not delete it for the sake of testing.

The window connects to an already running client that uses the same data directory:

```sh
messenger --root DATA_DIR ui
```

Open the private link it prints on this Mac. Opening the window, browsing and waiting do not call a model. A new substantive message can call one only if the client has been explicitly allowed to launch and processing is not paused. More in [local interface](docs/LOCAL_UI.md).

## Documentation

- [Current plan to release and readiness criteria](docs/RELEASE_PLAN.md)
- [Updates from the window: capabilities and limitations](docs/UPDATES.md)
- [Installing, updating and building the preview](docs/INSTALL.md)
- [Windows and Linux: download, window, process control, what is verified](docs/WINDOWS_LINUX.md)
- [Automated checks and their limits](docs/CI.md)
- [Local window, messages and send recovery](docs/LOCAL_UI.md)
- [CLI setup and creating a test session from the window](docs/SESSION_SETUP.md)
- [Background launch and reconnecting](docs/BACKGROUND.md)
- [Passive diagnostics](docs/DIAGNOSTICS.md) and [explicit CLI check](docs/CLI_CHECK.md)
- [Persistent Claude: queue and recovery limitations](docs/CLAUDE_STREAM.md)
- [Architecture and protocol](docs/PLAN.md), [MVP](docs/MVP.md), [testable hypotheses](docs/VALIDATION.md)
- [App requirements](docs/APP.md), [saved v1 design](design/README.md), [broker hosting](docs/DEPLOYMENT.md)

## Project scope

Messenger delivers messages and manages their processing. The agents themselves maintain the project's tasks and documentation. The coordinator is not tied to Claude or any other single provider; handing off this role is planned separately from the broker role.

This GitHub repository is for Agent Messenger. Keys, local profiles, queues, real CLI session IDs and their history are never uploaded here.

The research [Python test rig](https://github.com/kolotiloff/agent-messenger/tree/41e0e6e7e3f20275d480f6b064a881e7b5f4cf9e/experiments/session_safety/README.md) and the [unfinished draft](https://github.com/kolotiloff/agent-messenger/tree/41e0e6e7e3f20275d480f6b064a881e7b5f4cf9e/experiments/python_client_draft/README.md) are kept as sources of scenarios. They are not the product and do not mean rewriting Go back into Python. Historical results are in the [reports](docs/reports/); their date and verified commit matter.

A Telegram mirror, additional adapters and conveniences come after core delivery and session protection. The next mandatory steps are listed in the [release plan](docs/RELEASE_PLAN.md); a green build by itself does not confirm release readiness.
