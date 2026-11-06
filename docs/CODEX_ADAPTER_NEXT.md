# Next adapter: persistent Codex

Research and first Go components as of November 6, 2026, with Codex CLI 0.155.1 installed. The full persistent adapter is not yet connected to the product client.

## Implemented in Go

`internal/adapters/codex_appserver.go` builds the launch of a separate stdio app-server for a managed profile and checks the actual `config/read`: read-only, a ban on requesting elevated permissions, web search disabled, twelve disabled features, and no enabled/undetermined MCP. A missing or `null` field is not treated as confirmation that something is disabled. Errors do not contain server names, setting values or secrets. The minimal check is not permission to call the model and does not prove isolation of all Codex features.

`internal/adapters/codex_turn.go` checks dynamic tool calls for a single already confirmed thread/turn. Only `messenger_bound.read_current` and `reply` are allowed, in that order. The reply uses the issued receipt and a write handler bound by the controller to a specific attempt. A recipient from the model is not accepted. A write error, someone else's ID, a stale receipt, a repeated call or `failed`/`interrupted` do not become success. After stopping, the tools are closed. The controller must separately match the RPC ID, write the result to SQLite and keep the process reservation in case of uncertainty.

### Session setup (T-001)

`messenger --root DIR create-codex-session --environment MANAGED_PROFILE --alias NAME --cli FILE` creates a new Codex session before its first message. The client must run with `--allow-model-calls`; with `--only-session`, the alias must match. The default `~/.codex` profile is refused.

1. **Intent.** A `codex_setups` row records the intent, alias, profile, CLI and a private workspace (`workspaces/<id>`). This happens before anything is launched. The alias is reserved from this moment: `register-session`, Claude setup and other Codex setups cannot take it.
2. **Launch.** The state becomes `launching`. A reservation with `codex_setup_id` is written to `active-runs.json`. Only then does exec happen. The process gets an explicit environment (`HOME` and `TMPDIR` private to the setup, `CODEX_HOME` set to the managed profile, `PATH`) and the flags from `CodexAppServerArgs`.
3. **Config check.** After `initialize`/`initialized`, `config/read` is checked with `ValidateCodexAppServerConfig`. Only a SHA-256 of the checked config is stored.
4. **Thread request.** `thread_requested` is committed **before** `thread/start` is written. The request asks for a non-ephemeral thread with a read-only sandbox, approval policy `never`, and the `messenger_bound` dynamic tools (`read_current`, `reply`). Tools can only be declared at thread start.
5. **Thread ID.** The result must be an idle, empty, non-ephemeral thread in the same workspace with the same sandbox and policy. Its ID is stored (`created`) and added to the reservation. The live app-server stays the owner: an empty thread is not written to disk, so no other process could resume it.

Failure rules:

- Every failure after step 2 makes the setup `uncertain`: protocol error, timeout, EOF, rejected config, unexpected server request, process exit or client restart. The alias and the reservation stay. Nothing is retried, and `thread/start` is never sent twice.
- Only a definite OS exec refusal leads to `not_started` and releases the alias.
- `recover-failed` refuses a reservation held by a Codex setup.
- `recover-codex` resolves an uncertain setup after a proven reboot (see below).

The setup RPC client allows only `initialize`, `config/read` and `thread/start`. On success the session is registered under the alias as `codex_owned`, with no CLI path. Only its live owner serves it: generic claims, `register-session` and `ConfigureLaunch` refuse it, and `resolve` is blocked. An older client cannot launch it, because it only launches sessions that have a CLI path. [Verification report](reports/2026-11-06-codex-setup-linux.md).

Compatibility:

- **Clients from before schema version 1.** An older client (preview.19 and earlier) ignores `codex_setups`.
  - It cannot launch a Codex-owned session, because that session has no CLI path.
  - It does not see the alias reservation.
  - It treats the new registry entry as an ordinary blocking reservation.
- **Schema version 1 and later.** The database now records `schema_version`, and a client refuses to open a database written by a newer schema. This protects future versions; older clients cannot be changed retroactively.

### Message delivery (T-002)

Each owner has one controller goroutine (`serveCodex`) and a state in `codex_setups`: `created` means idle, `busy` means a turn is in flight.

1. **Idle.** While idle, the controller reads the server output. Notifications are dropped. A server request or EOF makes the owner uncertain. Every 30 ms it checks the queue, which never calls the model. It claims a message only while model calls are allowed for the alias (`--allow-model-calls`, `--only-session`) and the queue is not paused.
2. **Claim.** `ClaimCodexTurn` reserves the attempt and a one-time receipt (`codex_turns`), and the owner becomes `busy`.
   - **Config re-check.** Before anything is sent, `config/read` is called again. It must validate and have the same fingerprint as at setup.
   - **Canonical fingerprint.** The hash is taken over canonical JSON with sorted keys, because app-server changes key order between calls. This was seen on Codex 0.155.1.
   - **On a mismatch** the turn is not sent. `AbortCodexTurnUnsent` returns the message to the queue and makes the owner uncertain.
3. **Send intent.** `PrepareCodexTurnSend` commits `sent_intent` **before** `turn/start` is written. That request is never sent again.
4. **Turn start.** The only model input is `CodexTurnPrompt`. The message text is read through the bound tool.
5. **Bind.** The turn ID from the `turn/start` response is bound to the attempt (`BindCodexTurn`); only then do the tools open. Frames that arrive before that response are buffered, at most 256, and handled after binding.
6. **Tools.** `CodexTurnGuard` enforces `read_current` then `reply` with exact thread and turn IDs, the stored receipt and no model-chosen fields. Read and reply go through SQLite (`CodexRead`, `CodexReply`), and the recipient is the message's sender.
7. **Other server requests.** Approvals and the like are refused with a JSON-RPC error; the turn can still finish. A repeated server request ID, an unexpected response, a foreign turn, or a rejected tool call ends the turn.
8. **Success.** Success is only `turn/completed` with status `completed` after the reply is stored. The verification is written with `provider_ok=1` and no exit code, and the owner becomes idle again and keeps the thread.
9. **Failure.** Any other outcome makes the attempt and the owner uncertain: `failed`/`interrupted`, a missing reply, EOF, the 15-minute timeout, shutdown, or a DB error. The session stays locked and later messages stay queued. stdin is closed, the reservation is kept, and nothing is retried.

A restart cannot reach the old owner, so its setup and any in-flight attempt become uncertain. Shutdown closes the owners' stdin, as process exit would.

### Recovering an uncertain owner (T-007)

```sh
messenger --root DIR recover-codex --alias NAME --confirm preserve-uncertain
```

The command takes the uncertain Codex setup that holds the alias and removes its reservation.

**When it runs.** All of these must hold:

- the client is stopped;
- exactly one registry entry with the setup's `codex_setup_id` exists;
- that entry matches the database: same data directory, profile, thread and process observation;
- `ProvePreviousBoot` shows that the same host has booted again since launch. The recovery epoch is written into the reservation at launch.

**What it refuses.** The same boot, a missing PID, an empty group, a different host, and launches without a recorded epoch. This is the same rule as `recover-stream`.

**How it records progress.** It is two-phase:

1. `retiring` is committed together with the reservation's fingerprint.
2. The registry entry is removed.
3. The setup becomes `retired`.

If the command is interrupted, the client refuses to start until the same command is run again. A repeat finishes the work without changing anything else.

**Result.**

- The session is closed with `closed_without_retry`, so new messages to the alias are refused.
- Attempts, replies and uncertain messages are kept, and nothing is resent.
- No model is started.
- The thread is not resumed. To keep the session and resume its thread, use `--resume` instead (T-009, below).
- The alias stays bound to the closed session, so its history remains readable. A setup that failed before a thread existed releases its alias.

On Linux and Windows the boot identity source is not implemented. There the command always refuses.

### Resuming a saved thread (T-009)

```sh
messenger --root DIR recover-codex --alias NAME --confirm preserve-uncertain --resume
messenger --root DIR resume-codex-session --alias NAME [--cli FILE]
```

**Recovery with `--resume`.** The proof is the same as above: a stopped client, a matching registry entry and a proven reboot since launch. Three things differ:

- The thread must have at least one verified turn. Codex writes a thread to disk only after its first turn, so an empty thread cannot be resumed; the command refuses and says to recover without `--resume`.
- The setup becomes `resumable` instead of `retired`. The session stays open and the alias stays reserved, so new messages to it wait in the queue.
- A message the old owner held stays `uncertain` and is never resent, but it no longer locks the session. The queue is paused.

The mode is stored with the pending recovery (`recovery_mode`), so a rerun of a half-done recovery cannot switch between close and resume.

**New owner.** `resume-codex-session` needs a client with `--allow-model-calls` for the alias. It reuses the same `codex_setups` row, so the thread keeps one setup and one alias:

1. It refuses if the shared registry already holds anything for this setup or thread.
2. `resumable` → `launching` is committed, and the old process observation is dropped. Then the reservation is written, already naming the thread, and only then does exec happen. The CLI is the stored one, or `--cli` if Codex has moved, for example after an update.
3. The process is recorded, and `initialize` and `config/read` run as at setup. The config must pass `ValidateCodexAppServerConfig`; its fingerprint is stored again and checked before every turn.
4. `resume_requested` is committed **before** `thread/resume` is written. The request is sent once, by `CodexRPC.ResumeThread`, the only way to send that method. It names the stored thread and sets the workspace, read-only sandbox and approval policy `never` again. `excludeTurns` keeps the response small.
5. The response must be exactly the stored thread: persistent, idle, the same workspace, read-only, no approvals. Then the setup is `created` again and the controller serves the queue as before.

Resuming starts no turn and calls no model. The queue stays paused until it is resumed. Every failure after step 2 is uncertain, with the alias and reservation kept and nothing retried. Such failures include an unknown thread ("no rollout found"), a different or active thread, a timeout and a rejected config. A missing thread is never replaced with a new one. Only a definite exec refusal returns the setup to `resumable`.

**Dynamic tools after a resume.** `thread/resume` cannot declare dynamic tools. On Codex 0.155.1, a thread resumed in a new process still offered `messenger_bound.read_current` and `reply` to the model, together with the earlier history. This was checked with a local fake model that records each request; no real model was called. If a future Codex dropped the tools, the turn could not reply, and it would end uncertain rather than as a success. [Verification report](reports/2026-11-06-codex-resume-linux.md).

**Compatibility.** The schema version is now 2. A client at schema version 1 (preview.21 and earlier) refuses a data directory that a newer client has opened, because it does not know the resumable states.

On a Mac, three cases were verified with the real installed app-server and empty profiles, without a model: a clean profile was accepted, an inherited MCP was rejected, an explicitly disabled MCP was accepted. [Checks and limitations](reports/2026-11-06-codex-preflight-mac.md).

The components are not yet called from `serve` or the window. Still needed: a persistent transport that handles event ordering, atomic registration of the ID issued by Codex, binding to SQLite/reservations, a managed profile with an existing login, and a test with two real messages. The existence of these Go files does not mean the Codex launch button is ready.

## Verified locally without a model

The help output offers `codex app-server --listen stdio://` and protocol schema generation. The schemas for this installed version were obtained with the standard `generate-json-schema --experimental` command. Empty separate HOME/CODEX_HOME and working folder ruled out use of working conversations and saved logins.

1. `initialize` returned a response; after `initialized`, the process exited normally when stdin was closed. `turn/start` was not sent.
2. `thread/start` created a new ID without calling the model. After the process was closed, a `thread/resume` attempt in a new process with the same exact ID returned `-32600`, "no rollout found". Therefore, creating an empty conversation does not by itself prove that a saved session exists. There was no model task in this probe.

The IDs and full responses stayed local and were not included in the repository. A conversation that was not found must not be automatically replaced with a new one and treated as a continuation.

## Configuration isolation: confirmed limitations

`codex app-server --ignore-user-config --help` in the installed version 0.155.1 returns an unknown-argument error. Flags of the one-shot `exec` must not be automatically carried over to app-server.

The [reproducible probe](../experiments/codex_app_server/README.md) was run on a Mac without creating conversations or calling the model. Via `config/read` in three separate temporary profiles, the following was confirmed:

- `-c 'mcp_servers={}'` **does not remove** an MCP server from the configuration file: it stays enabled.
- `-c 'mcp_servers.fixture.enabled=false'` disables the specified server.
- In a clean profile, the server is absent. In each case, eight explicitly disabled features were confirmed by the actual configuration.

This is consistent with the per-key layering of settings described in the [official documentation](https://learn.chatgpt.com/docs/config-file/config-advanced). An empty table must not be treated as proof of isolation, and the model must not be launched right after building argv. The future adapter must check the actual configuration before creating a conversation, refuse on an unknown/enabled MCP, and not write the contents of the user configuration to the log. A read check by itself does not yet address the configuration changing between the check and the launch: the product mode requires either an immutable managed configuration, or a repeated check at the launch boundary plus confirmed loader behavior. Transferring user credentials into such a profile is not implemented by this research.

System layers, other Codex features, actual tool startup, persistence of dynamic tools and exact resume were not accepted by this research. Later steps covered some of this: real tool calls on the Mac (T-004), and on Linux with a fake model, tool persistence and exact resume (T-009). Exact resume with a real model on the Mac is still pending.

## Protocol and design decision

The [official App Server description](https://learn.chatgpt.com/docs/app-server) defines `initialize`/`initialized`, `thread/start`, exact `thread/resume`, `turn/start` and the `turn/completed` event. Success requires the `completed` status; `failed`/`interrupted` are not success. Dynamic tools are experimental; the protocol allows their calls to be handled on the Messenger side.

For the first adapter, we propose a separate stdio process per managed session, without connecting to the user's daemon or GUI process. This is a project decision, not a property automatically guaranteed by Codex.

## What to implement and verify

- First, a separate creation state: a Messenger address does not yet mean a saved Codex conversation. The ID issued by the server is recorded atomically before the first message; a lost response leaves an uncertain state and does not trigger a repeated creation.
- A single owner of the process/session, the same cross-process reservations as for Claude. Before identification, only setup protocol commands are allowed; no model messages are sent.
- Matching of RPC ID, thread ID, turn ID and attempt number; unexpected or repeated events do not complete a neighboring message.
- A limited set of Messenger tools, a reply address from code, a one-time receipt. Tool support and isolation are verified first on a fake CLI and on the installed CLI's schema.
- Do not inherit working MCP, hooks, tasks or automatic goals. The specific disabling settings still need a separate check; until then, the real model is not launched.
- The first two messages in one process, then a separate check of persistence and exact resume. While waiting, do not send `turn/start`, background hints or model checks.
- After two independent adapters — explicit Claude ↔ Codex routing with a limit on automatic replies. A simple reply to the user is not yet an exchange between agents.

Windows/Linux, termination of all descendants and Cursor are not accepted by this research.
