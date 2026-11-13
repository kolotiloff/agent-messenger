# Application, installation and interface — requirements v0.3

> **Historical.** The original target requirements of November 5, 2026. The product went another way in places: tasks instead of conversations between agents ([TASK_MODEL.md](TASK_MODEL.md)), and the repository is public. For the current state see [RELEASE_PLAN.md](RELEASE_PLAN.md#current-status).

Agreed with the user on November 5, 2026. This is the target product; an item being in the plan does not mean the feature is ready. The first user-facing version is for Mac, then Windows x2 and Linux. The UI is part of the MVP.

Diagnostics are available as [doctor](DIAGNOSTICS.md): detection of CLI candidates and client status. In passive doctor, the version, login and readiness of a specific session remain `not_checked`; for an explicitly selected CLI, a separate [check-cli](CLI_CHECK.md) is implemented with a limited verified Mac matrix. A [local interface](LOCAL_UI.md) in the browser has been added: history, sending, pausing and resuming the queue. Outgoing messages, once written to the client database, are recovered after the window is fully closed; there is a [technical archive for Mac](INSTALL.md), but there is no native shell and no accepted public installer yet. The release plan is [RELEASE_PLAN.md](RELEASE_PLAN.md).

## First launch

1. Download the Agent Messenger installer from GitHub Releases, install and open the application.
2. Choose "Connect to a server" (address and device registration) or "Run a server on this computer". For a VPS, choose an existing server or the "Install broker on VPS" wizard with temporary SSH access.
3. See the local computer and cards for Claude Code, Codex and Cursor Agent CLI.
4. The program detects executables and versions, checks the available login status using the CLI's standard tools, and shows the time and result of the check.
5. For a missing CLI, "Install" and clear instructions are available; installation uses a verified official source for the OS. For a missing login, "Log in" via the provider's standard flow. The user performs the browser actions that require them personally.
6. Create a test CLI session or explicitly hand one over to Messenger's control. An already open working conversation is not taken over automatically.
7. Send a test message and see it go through and the reply.
8. For a local broker, access is initially limited to this machine; connecting other devices and network access are enabled separately.

Installation instructions for an agent are also needed: choose the build by OS/architecture, verify integrity, install, run diagnostics and show the user the remaining actions. Secrets and authorization are not embedded in the install command. The repository is currently private: download access depends on the user's access to GitHub. We do not change its visibility on our own.

Installing the application does not mean buying a server or subscriptions. Automatic setup of an existing VPS is started by a separate wizard on explicit user action. A device can work with a local broker on the same machine. A VPS is optional; the server can be any chosen supported computer. Regular connection settings belong to Messenger; SSH access is requested separately and only by the install/maintenance wizard.

## Main window

The design chosen by the user is fixed in [design/v1](../design/README.md): the exact source mockup, CSS, color and size parameters, and an openable copy. The implementation keeps this visual foundation.

The application is not tied to a specific coordinator or external window. Any supported agent can be the coordinator; the role can be handed over. Personal interface preferences do not become mandatory scenarios or release conditions.

The cards use the local [overview](UI_READ_API.md): recorded session states, aliases and queue counters. This is not a check of a live process or model availability. The concept is saved separately; the first [browser interface](LOCAL_UI.md) already reads this data and the log through the local client.

### Computers and agents

An agent card shows independent indicators:
- CLI: found/not found/unsupported version, version and selected path.
- Login: confirmed/login needed/could not check; time of the check. An unknown status is not shown as ready.
- Execution: not configured/ready/busy/paused/manually managed/needs review.
- Connection: local client reachable/unreachable, broker connected/no connection, last seen time of the remote device.
- Queue: number of pending messages and whether intervention is needed.
- Coordinator role and its current owner.

Actions: "Install CLI", "Log in", "Check", "Set up session", "Pause new jobs", "Stop current processing", "Open diagnostics". Stopping can leave an uncertain result; the interface does not hide this.

"Connected" refers to the managed CLI and the Messenger client. It does not mean a connection to an open Cursor GUI chat, Claude Desktop, or the current Codex chat.

### Conversation

Visible: the content of messages and replies, sender, recipient, machine, time, reply chain, and, separately, delivery and processing states. Filters: by machine, agent and conversation. Messages persist after the window is closed and the client is restarted.

The new message form lets you choose a recipient or the coordinator. A reply from the window keeps the link to the selected message. Technical acknowledgements are hidden from the main dialog but available in diagnostics. Viewing the conversation does not require running a model.

This is the Messenger message log, not a promise to show the model's internal reasoning or the full history of each CLI.

### Control and attention

Coordinator selection, device list, pausing queues and a list of problems: login needed, tool refusal, missing CLI, lost connection, uncertain launch. Changing the coordinator uses a verifiable role handover; we do not move CLI history between providers.

For each failure: a clear reason, what is already known about delivery/reply, what is blocked, and the safe available actions. There is no single "reset the lock and retry everything" button.

## Messenger server

Settings contain the address and identity of the connected broker, device registration and a connection check. For the local server role: enabling/stopping the service, autostart, access scope, address/port, database and backup state, inviting another device. "Service running" and "server reachable from this client" are shown separately.

The broker and the coordinator agent are different roles. Turning off the local broker does not automatically elect a new one. Changing the address to another broker does not send it the old queue; an explicit binding or migration is required. [Placement architecture](DEPLOYMENT.md).

### Install broker on VPS

The wizard takes the IP/name, SSH port, user, and a key or password. It verifies the server's authenticity, compatibility and permissions; shows the changes, installs the broker as a separate service, configures a secure connection and registers this device. After installation, SSH access is not needed for regular messaging.

Installation statuses are shown step by step; "Connected" means a verified connection between the application and the broker and a successful technical test without a model. The need to open the firewall in the provider's panel, a wrong certificate, or an unavailable service is shown as a separate reason. Secrets are not saved by default and are not passed to agents. [Full flow and acceptance](DEPLOYMENT.md#vps-setup-wizard-in-the-app).

## UI architecture requirements

The UI reads the local client's states and events through a restricted local API/IPC. Launch commands, permissions and the queue are executed by the client; the window is not a second independent CLI launcher. Service events, heartbeat and screen updates do not call models.

In the local MVP, closing the window must not lose messages or start duplicates. The user can clearly see whether the client keeps working in the background; there is a full quit command with a coordinated shutdown. Autostart is enabled by an explicit setting.

The specific UI framework is chosen through a small technical experiment: a window on Mac, communication with the local client, packaging, future Windows/Linux support. The preliminary Go core stack does not fix the UI framework. The Python test benches verify contracts; they are not the final choice of product language.

## Packaging and updates

For each supported release: version, builds per OS/architecture, checksums, install/uninstall instructions, known limitations, minimum CLI versions, and a safe local database upgrade. Signing/provenance verification of builds and system warnings are part of the installation check.

An update does not silently interrupt an active session. Queue data, device tokens and CLI history are stored separately from the application and the source repository.

## Acceptance of the first window on Mac

- On a machine with a missing CLI, it is visible what needs to be installed; if the check fails, there is no false green status.
- The standard authorization path and re-check are available without reading/showing secrets.
- A message and its linked reply are visible in the window; the states match the client log.
- Two messages for the same session are executed sequentially; a duplicate does not call the model.
- On connection loss/failure, the reason and the queue are visible; uncertainty does not turn into an automatic retry.
- Viewing the conversation and updating statuses do not run a model.
- The local broker starts without a VPS; client mode connects by address and device registration. There is no false network reachability status based only on the fact that the service started.
- Installation from a release and via the agent instructions is verified in a clean user environment; manual steps are explicitly described.

Final acceptance of the network and the other OSes remains a separate stage. Telegram is an add-on after the core messaging.
