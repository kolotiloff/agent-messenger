# Plan to the first release

Written November 6, 2026. The current status below supplements the original [MVP](MVP.md), [architecture](PLAN.md), [window requirements](APP.md) and [broker deployment](DEPLOYMENT.md). Transitions are decided by checks.

## Planned conversation and document workflow

- **T-361 — File links and a document reader.** Resolve Markdown file links and plain paths in the report’s Files field against the task’s project on its source computer. Accept both legacy inline lists and Markdown links with optional line or heading references. Show each file as a link with its name, project-relative path and source when relevant. Open Markdown and supported text formats in a read-only viewer and restore the chat position on return. Explain when the path is unavailable. Keep the original report available through Copy message.
- **T-362 — File actions.** Give links and previews the same menu with Open, Open with, Save as, Copy path, Copy contents, and Reveal in the system file manager as appropriate. Start with files available on the local computer. Remote content requires an authorized project connection. Show the originating computer and preserve the path when its contents are unavailable. Binary files can be opened in an associated application when available.
- **T-363 — Work progress.** Show a collapsible work section before the final answer in original event order. Start with the worker’s first progress message. Group reads, edits and searches when the underlying events identify them, preserve raw commands in details, and show elapsed time when known.

Accept the workflow with a completed task report that has a legacy Files entry with several paths, a Markdown link to a specific line, an attached image and a file from an unavailable computer. Confirm that each available file can be opened or acted on and that the same links and progress appear after reopening from Archive.

## Chat typography and change summaries

- **T-366 — Interface legibility.** General offers four interface font choices. System high contrast keeps the system font and text size while adjusting the text colors. Inter and Source Sans 3 are bundled with their licenses. Validate the owner’s chosen appearance on their desktop.
- **T-363 follow-up — Change summaries.** When a completed task has verified file changes, show a card immediately below its final answer with a prominent changed-file count and total added and removed lines. Show the first three file rows with their project-relative paths and individual line counts. A button labeled “Show N more files” reveals the rest and can collapse them again. Show the verified executor and source computer when available. Open a file or its diff from its row when available. The card is read-only and has no undo action. Use only changes attributed to that task and omit unavailable line counts. Include all verified file changes, including documentation, configuration and assets. Omit the card when the task has no verified changed files. Keep the summary accessible when the task is reopened from Archive.
- **Change summary acceptance.** Check a completed task with more than three changed files, one with exactly three, and one with changed files but unavailable line counts. Verify that a documentation-only edit produces a card and that a read-only task or a task with no verified changed files leaves the final answer on its own. Verify that the header and per-file totals reflect the task's verified data, that keyboard users can expand and collapse the list, and that reopening the task preserves the summary.
- **Project folders.** Consider a pin action that keeps a project accessible when it has no active jobs. Keep local project removal explicit and confirmed.

## macOS name in the next desktop update

Upgrade a stock build 118 installation whose bundle is still named Agent Messenger.app. After the update, verify a single Kotel application in Finder and search, the Kotel tooltip and relaunch from the pinned Dock item, the same profile and task history, and successful installation of a second signed update. Preserve existing executable entry points while they have consumers and keep active tasks working through the transition.

## Published 1.0.14 fixes

[Build 117](https://github.com/kolotiloff/agent-messenger/releases/tag/v1.0.14-preview.117) passed all five native update checks and public channel verification.

- In direct chats, answer the owner's current question in their language. Show one verified reply and let the owner expand the preceding progress text. Keep pending and failed output available.
- Keep the remote download button in place on hover. Give Archive the same default width as Projects and preserve individually saved widths.
- Show the time and Copy message action below a worker reply on hover or keyboard focus. Copy its complete source text, including line breaks and Markdown.
- Suppress historical remote update failures when reopening the window. Show a new failure and report a request observed waiting if it subsequently times out.
- Restore the active local coordinator label when an update replaces the process serving the project's existing MCP chat.

## Published 1.0.13 bridge release

- Kotel branding, the approved desktop interface and signed in-place updates from the original GitHub repository.
- Fix opening an older project database after updating, including Windows installations that reported `no such column: remote_live_tokens`. Kotel adds each missing live-usage column, preserves manually repaired values and resumes the same task history and remote connection.

## Current status

Updated 2026-11-25. The details of each release are in [HANDOFF.md](HANDOFF.md) and [TASKS.md).

The program is a technical preview used on Mac, Windows and Linux. [Messenger 1.0.3, build 106](https://github.com/kolotiloff/agent-messenger/releases/tag/v1.0.3-preview.106) is the version covered by this overview. The desktop displays the short version, while the tag and signed manifest retain the numbered build suffix. See [the latest published release](https://github.com/kolotiloff/agent-messenger/releases/latest).

Done since the plan below was written:

- **Task model** ([TASK_MODEL.md](TASK_MODEL.md), D-021 to D-024): the owner talks to a coordinator in a Claude Code, Codex or Cursor chat; each task runs as its own worker session of Claude Code, Codex or Cursor and reports back. The persistent session paths of stages 2 and 3 were replaced by this model.
- **Network of computers** ([BROKER.md](BROKER.md)): the server runs inside the client on any computer of the network; invitations by code, TLS with a pinned certificate, tasks, answers, notes, logs and updates between computers.
- **Windows and Linux** ([WINDOWS_LINUX.md](WINDOWS_LINUX.md)): ready downloads for x64 and ARM64, the program's own window, installation for the current user, job objects on Windows.
- **Updates** ([UPDATES.md](UPDATES.md)) on all three systems from signed GitHub releases; the window restarts an older background client; Windows and Linux computers can be updated from another computer.
- **Window**: project-scoped task lists and statistics, planned work, alerts for unclear task outcomes, worker notes and answers, elapsed time, reported models and a feedback journal.
- **CI** on Linux, macOS and Windows, with the downloads built and installed; a daily watch of the worker CLIs.

Not done yet: Developer ID signing and notarization on Mac and code signing on Windows (T-013), an Intel Mac build (T-014), backup and rollback (T-011), coordinator role handover (T-017), a standby server computer, a phone view (T-147).

The sections below preserve the November 6 plan and its exit conditions. The current behavior is described in [Task model](TASK_MODEL.md), [Updates](UPDATES.md) and [Download and start](DOWNLOAD.md).

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
