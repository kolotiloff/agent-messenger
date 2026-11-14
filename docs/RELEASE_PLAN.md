# Plan to the first release

Written November 6, 2026; the status was last updated on November 14. This is the sequence of work on top of the original [MVP](MVP.md), the [architecture](PLAN.md), the [window](APP.md) requirements and [broker deployment](DEPLOYMENT.md). No release date is set: transitions are decided by checks, not by the percentage of code written.

## Current status

Updated 2026-11-14. The details of each release are in [HANDOFF.md](HANDOFF.md) and [TASKS.md](TASKS.md); this section is the overview.

The program is a technical preview used every day on the owner's Mac, Windows and Linux computers. Releases are numbered 0.N (tag `v0.0.0-preview.N`); 0.70 is the newest published one.

Done since the plan below was written:

- **Task model** ([TASK_MODEL.md](TASK_MODEL.md), D-021 to D-024): the owner talks to a coordinator in a Claude Code, Codex or Cursor chat; each task runs as its own worker session of Claude Code, Codex or Cursor and reports back. The persistent session paths of stages 2 and 3 were replaced by this model.
- **Network of computers** ([BROKER.md](BROKER.md)): the server runs inside the client on any computer of the network; invitations by code, TLS with a pinned certificate, tasks, answers, notes, logs and updates between computers.
- **Windows and Linux** ([WINDOWS_LINUX.md](WINDOWS_LINUX.md)): ready downloads for x64 and ARM64, the program's own window, installation for the current user, job objects on Windows.
- **Updates** ([UPDATES.md](UPDATES.md)) on all three systems from signed GitHub releases; the window restarts an older background client; Windows and Linux computers can be updated from another computer.
- **Window**: tasks of all computers with progress and tokens, answers and notes to workers, closing stuck tasks, agents with their models and limits, statistics, feedback journal.
- **CI** on Linux, macOS and Windows, with the downloads built and installed; a daily watch of the worker CLIs.

Not done yet: Developer ID signing and notarization on Mac and code signing on Windows (T-013), an Intel Mac build (T-014), backup and rollback (T-011), coordinator role handover (T-017), a standalone server for a VPS (T-036), a standby server computer, pictures from workers in reports, a phone view (T-147).

The stage table below is the plan of November 6 and is kept for its exit conditions; several stages were reached in a different order or replaced by the task model.

## Path to release

For persistent Codex, the first Go components have been added: checking the effective configuration and the linked tool calls of a single turn. Three non-model checks of the installed app-server on Mac passed; [report](reports/2026-11-06-codex-preflight-mac.md). Wiring these components to the process owner, SQLite and the window is still ahead; Codex is not yet claimed to work in the product.

| Stage | Work | Exit condition |
|---|---|---|
| 1. Updates | New-version badge, release notes, «Обновить и перезапустить» ("Update and restart")/«Позже» ("Later"); signed packages; delivery from public GitHub Releases; window and background client compatibility; only the lifecycle changes needed for updates | Two real builds: the one installed on Mac finds the next one, shows the badge and updates through the window. Conversations and the queue are preserved, agents are not duplicated. A corrupted/unsigned package, an interrupted download and incompatibility are refused while keeping the working version. An active session is not forced to stop. |
| 2. Finish the local client and adapters | Safe termination and recovery; persistent Codex and Cursor adapters; several bindings without starvation; restricted MCP send/reply with the address set by code | Three CLIs pass two sequential messages into exact sessions; contention, a wrong ID, detached descendants, a partial reply and a crash do not bypass the protection. Safe daily exit/return and its limitations are explicitly described. |
| 3. Connection and window | CLI discovery; separately: version/compatibility, login, client availability and session state; standard authorization; choosing the managed session; clear errors. Section order: «Переписка» ("Conversations") → «Агенты» ("Agents") → «Сервер» ("Server") | Absence, unknown and confirmed readiness are not mixed. Checks and the window do not invoke a model. Closing/reopening does not lose messages or create duplicate CLIs. Unimplemented server features do not pretend to be connected. |
| 4. Local release for Mac | Signed distribution, installation from GitHub, instructions for a human/agent, backup and uninstall; a local broker in the same format as the future network one | Clean install and update with a non-empty queue verified. Data is separated from the application. Restoring a backup verified. The three claimed CLIs work in the product. If anything is missing — only an explicitly limited technical preview. |
| 5. Mac↔Windows network | One broker on a PC or VPS; TLS and device registration; addresses; outbox/inbox; ACK after write; transport retries without model retries; Windows locks, IPC and processes; VPS wizard | Two real computers exchange messages. Connection loss, broker restart, redelivery and device revocation verified. The same server works on the chosen PC and on a VPS. The SSH wizard does not break other services and safely resumes an interrupted installation. |
| 6. Four computers and the coordinator | Windows x2, Linux, platform packages; sleep/wake; optional autostart; coordinator role address and explicit role handover | All four machines passed runtime checks. Role handover preserves messages and does not create two coordinators. The broker role is separate from the coordinator role. |
| 7. Acceptance and release | Full failure matrix, long waits, protection against endless exchanges, clean installs/updates/rollback, documentation and verifiable builds | All release promises confirmed. There is a log of limitations, recovery instructions, verified backups and diagnostics. Draft PRs reviewed and integrated; the release is built from a committed state. |
| After the main version | Telegram mirror, new adapters, additional conveniences | Core delivery, session ownership and access are already accepted; additions do not bypass these mechanisms. |

Stages partly overlap. For example, diagnostics and the window foundation can be done while safe process termination is being investigated. A release cannot be declared ready by simply bypassing an open check. The first limited preview on Mac and the release for all four machines are different checkpoints; the contents of the preview are stated explicitly.

## Accepted update delivery path

Delivering new builds through the window is accepted on Mac up to and including preview.17. The items below describe the update contract; automatic switching of the background client is not accepted yet. Current work is session setup and local exchange while keeping the process ownership constraints.

1. Integrate Sparkle into the Mac shell: version check, unobtrusive badge, manual check, release notes and installation on a button press. The base mechanism is [Sparkle](https://sparkle-project.org/documentation/), not a home-grown replacement of executables.
2. Prepare signed updates and a delivery channel. The repository was made public with the owner's permission after a history review. Public packages are downloaded without an account key; signatures are verified by the application.
3. Verify compatibility and the conditions for restarting the background client. Do not present an update of only the window as an update of the whole product. Do not delete protective reservations, kill CLIs, or treat a pause as proof of a safe stop.
4. Release two consecutive test builds and go through the path from the badge to the restart, then verify queue preservation and refusal on failures. After that, subsequent completed development steps are delivered the same way.

Status: the local signed channel is implemented and verified by the preview.5 → preview.6 transition from the window. Updating the application while keeping the previous background client works. The repository was made public with the owner's permission after reviewing the history for secrets. Delivery of signed packages from public GitHub is implemented without an account key in the application; the preview.12 → preview.13 transition via GitHub was [verified on Mac](reports/2026-11-06-github-update-mac.md). Switching the background client version and full rollback are not accepted yet. The first build with the updater was installed once manually; subsequent local builds are installed through the window. See [boundaries and checks](UPDATES.md).

## What was done by November 6

1. The background client is accepted on Mac: [result](reports/2026-11-05-background-client-mac.md).
2. `doctor` and `doctor --json` were added as the first part of diagnostics: finding CLI candidates and the status of the existing client, without launching CLIs/models and without reading secrets.
3. A separate [check-cli check](CLI_CHECK.md) was added for exact verified versions of Claude/Codex/Cursor on Mac: version and standard login status, check time, `unknown` state on error. `doctor` itself executes nothing; login/logout are not invoked. Extending the OS/version matrix is a separate acceptance.
4. An [overview](UI_READ_API.md) was added: a limited summary of cards from the database without launching CLIs, without secret identifiers and without message texts. Aliases of one session are merged; an owner hold is not presented as a busy model. The demo concept is kept; the local window is already connected to this summary.
5. [timeline](TIMELINE.md) and a [local browser view](LOCAL_UI.md) were added with the chosen [design/v1](../design/README.md) styling. Sessions, the queue, history pages and replies are read from the client; verified on a separate test database without models. Explicit sending, pausing and resuming of the queue were added: [verification on Mac](reports/2026-11-06-ui-actions-mac.md). A double click, a retry after a lost ACK, and reloading the tab do not create a second message. A persistent outgoing log was added to the client database: [fully closing the window and a client crash verified](reports/2026-11-06-ui-outbox-mac.md). A Mac preview builder, version/revision, checksums and installation into separate directories were added: [clean install and update with a non-empty queue verified](reports/2026-11-06-preview-install-mac.md). The native shell is already installed. Next come updates through the window; audit and integration of the PR chain are done as the corresponding changes become ready. A full release still requires the remaining stages.
6. [Automated checks](CI.md) were added: native race on Mac/Linux, compatibility with stable Go, cross-compilation and verification of Mac archives. All four jobs for `b31596c` passed: Mac/Linux on Go 1.22.2 and Go 1.27.3, target builds, and Mac archive install/update. Preview CI is built with a supported stable Go; Windows runtime and real models are not accepted by this. [Nightly check report](reports/2026-11-06-ci-preview.md).
7. Persistent Codex/Cursor and the lifecycle remain mandatory separate tasks of stage 2. Core development does not depend on external GUI integrations; the [note on the personal Claude Code scenario](GUI_COORDINATOR.md) is kept separately and is not a release condition.

## What may require the owner's involvement

Signing in to accounts through the standard browser; real Windows/Linux computers for acceptance; a chosen VPS for remote testing; access to build signing/distribution if the chosen release method requires it. Until those points, implementation, local checks and documentation continue without approving each technical step. Rebooting the computer on our own, changing the repository's visibility, or taking over working GUI chats is not allowed.

## Conditions that do not change

- Messenger delivers messages and does not manage the project's files or tasks.
- A VPS is optional. One server component runs on a chosen computer or a VPS; there is no automatic selection of a fallback broker.
- Waiting, showing statuses and technical ACKs are done by ordinary code. New substantive messages may spend tokens; one message does not equal one API call.
- The coordinator needs limits on automatic replies, loop stopping, and the ability to batch several notifications into one request. Savings are verified by observation, not promised as a percentage.
- An unknown result is not turned into success or an automatic retry. Session IDs, accounts, keys and queues are not published.
- Cursor GUI and working conversations are not taken over. Changing the coordinator transfers the role and messages, not the internal history between models.

## In-app updates

Requirement from November 6: a new-version notification, release notes, download and «Обновить и перезапустить» ("Update and restart"), as in a regular editor. Local and public delivery of the window are accepted. The detailed contract is [Mac app updates](UPDATES.md): trusted signatures, UI/client/schema compatibility, interruptible download, data preservation, and deferred installation while a CLI owner is active. The check spends no model tokens. Automatic switching of the background client and full rollback are not accepted yet.

## Order of upcoming updates (November 6)

At the owner's request of November 6: UI issues are accumulated together with the current development stage. One completed step and the accumulated fixes ship as one update; a separate package is not prepared for every small thing. First we finish the stage's verification, then publish a signed build to the channel and announce its availability.

## Publishing for three platforms

The general release includes macOS, Windows and Linux. On the main page each platform has its own download entry. Until real acceptance it is marked «готовится» ("in preparation"). Each OS needs its own window/package, signed update channel, clean-install verification and queue preservation. Go cross-compilation does not replace these checks. The native Mac shell does not restrict Messenger's architecture to that system.

With each completed step, the sources and up-to-date README/instructions move to the main branch together with publishing the package. Accumulated UI issues are included in the same release.

By default the message queue is shown to the right of the conversation; an icon at the top hides and restores the panel. There is no separate queue tab. Currently the panel shows the number of pending messages and sessions requiring attention, based on client data. The queue panel, the message history and the list of computers scroll independently; the theme, current version and update are pinned at the bottom left.


## Planned directions

These directions are accepted without expanding the current stage:

| Priority and stage | Direction | Readiness condition |
|---|---|---|
| Stage 3: connection | A step-by-step setup: choice of agent, connection and checks. | CLI installation, login, compatibility and session readiness are shown separately. A status check does not call the model. No false "connected". |
| Stages 2–3: observation | A live stream of available events: reply, tool start/result, waiting, error. Reconnecting the interface restores the view from an event cursor. | A window drop and reconnection do not create a new message or a model run. We do not promise hidden reasoning that the provider does not expose. |
| Stage 5: network | Separation of transport and execution, logical addresses, binding version, idempotent IDs and acknowledgement after write. The ban on silently changing the recipient of an accepted message stays. | Disconnecting/reconnecting a device does not lose a message and does not repeat a model run already started. Changing the binding does not change the recipient of an accepted message. |
| Before automatic group chains | Pass only new relevant context; limit size, number of hops and budget. | Context is tracked by confirmed IDs/cursors; truncation is explicit. Input/output tokens and the number of calls are measured. A loop stops at the limit without losing history. |
| Stage 5: permissions | Restricted MCP tools and allowed recipients, a log of control actions. | An agent does not get the right to change settings/keys merely because it can send a message. The log contains no secrets. |
| After basic exchange between four devices | A separate REST adapter for an API agent, if a real scenario appears. | One API agent passes the same contract for IDs, delivery, errors and limits, without bypassing the CLI guarantees. |

Routing, acknowledgement, waiting, window refresh and connectivity checks are done by ordinary code, not by a model. Corporate SSO/RBAC, Slack, public links and a marketplace are not tasks for the first release.
