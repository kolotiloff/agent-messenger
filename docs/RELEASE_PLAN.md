# Plan to the first release

Updated November 6, 2026. This is the current sequence of work on top of the original [MVP](MVP.md), the [architecture](PLAN.md), the [window](APP.md) requirements and [broker deployment](DEPLOYMENT.md). No release date is set: transitions are decided by checks, not by the percentage of code written.

## Current status

The current functional step, preview.19, is accepted on Mac: [creating and explicitly launching a separate test Claude session from the window](SCOPED_LAUNCH.md). After the fake CLI, real Claude Code 2.1.318 was verified: two sequential messages, two confirmed replies in one process, replies appearing automatically, and three older messages of other sessions preserved. The shell was updated to preview.19 through the native updater. Automatically switching the version of a running background client is still a separate task. The next step is persistent Codex, then several independent owners without starvation, and Claude ↔ Codex on one Mac. Cosmetic issues are folded into functional steps and do not form a separate release queue.

We are at the stage of a local technical prototype on Mac. There is a Go client, SQLite, protected session ownership, linked MCP replies, persistent Claude, exact resume, explicit recovery after a real reboot, and background launch. This was verified with separate test CLI conversations, not working chats. The product lives in `cmd/messenger` and `internal`; early experiments are kept separately. A technical preview and a full release for three OSes are different checkpoints.

In the persistent product mode, Claude has been verified on Mac. Earlier Codex/Cursor experiments do not replace their acceptance in the current client. Linux/Windows builds do not replace running on those OSes. A local browser UI with history, sending and queue management has been added. Outgoing messages, once first written by the client, survive closing the tab and restarting the UI. There is a native Mac preview builder and installation into a user directory, verified locally; the first [native Mac shell](MAC_APP.md) has been added. Signed distribution, acceptance of the shell on other Macs, and the network broker are not finished yet.

Open problem: fully terminating a CLI and safely releasing its session within the same Mac boot. Background `start` lets you close the terminal while keeping a live owner, but it does not solve this problem. Release by timer, by empty group, or by a single exited PID is not added.

Preview.19 was published after four successful CI jobs. The background client was updated separately with the previous data unchanged; the shell was installed through the updater. After two real replies, the test queue was paused and the live owner kept. [Release report](reports/2026-11-06-window-launch-mac.md). The [Codex adapter investigation](CODEX_ADAPTER_NEXT.md) continues: the app-server connection is confirmed without a model; an empty new conversation after exit does not yet guarantee a saved rollout. A reproducible check established that an empty MCP table does not clear inherited servers.

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

## What is already done

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

## Order of upcoming updates

At the owner's request of November 6: UI issues are accumulated together with the current development stage. One completed step and the accumulated fixes ship as one update; a separate package is not prepared for every small thing. First we finish the stage's verification, then publish a signed build to the channel and announce its availability.

## Publishing for three platforms

The general release includes macOS, Windows and Linux. On the main page each platform has its own download entry. Until real acceptance it is marked «готовится» ("in preparation"). Each OS needs its own window/package, signed update channel, clean-install verification and queue preservation. Go cross-compilation does not replace these checks. The native Mac shell does not restrict Messenger's architecture to that system.

With each completed step, the sources and up-to-date README/instructions move to the main branch together with publishing the package. Accumulated UI issues are included in the same release.

By default the message queue is shown to the right of the conversation; an icon at the top hides and restores the panel. There is no separate queue tab. Currently the panel shows the number of pending messages and sessions requiring attention, based on client data. The queue panel, the message history and the list of computers scroll independently; the theme, current version and update are pinned at the bottom left.


## Takeaways from Agentgram

The [comparison, sources and priorities](AGENTGRAM_REVIEW.md) fix the accepted directions without expanding the current updates stage. Stages 2–3 include observing available agent events, restoring the view without resending, and a connection wizard. Before network stage 5, we separate the logical address from the CLI binding, keeping the ban on silently changing the recipient of an accepted message. Before automatic chains, we need a context delta, limits and token measurement. A REST/A2A adapter comes after core delivery; enterprise integrations are not required for the first release.
