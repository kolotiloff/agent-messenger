# Next adapter: persistent Codex

Research and first Go components as of November 6, 2026, with Codex CLI 0.155.1 installed. The full persistent adapter is not yet connected to the product client.

## Implemented in Go

`internal/adapters/codex_appserver.go` builds the launch of a separate stdio app-server for a managed profile and checks the actual `config/read`: read-only, a ban on requesting elevated permissions, web search disabled, twelve disabled features, and no enabled/undetermined MCP. A missing or `null` field is not treated as confirmation that something is disabled. Errors do not contain server names, setting values or secrets. The minimal check is not permission to call the model and does not prove isolation of all Codex features.

`internal/adapters/codex_turn.go` checks dynamic tool calls for a single already confirmed thread/turn. Only `messenger_bound.read_current` and `reply` are allowed, in that order. The reply uses the issued receipt and a write handler bound by the controller to a specific attempt. A recipient from the model is not accepted. A write error, someone else's ID, a stale receipt, a repeated call or `failed`/`interrupted` do not become success. After stopping, the tools are closed. The controller must separately match the RPC ID, write the result to SQLite and keep the process reservation in case of uncertainty.

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

System layers, other Codex features, actual tool startup, persistence of dynamic tools and exact resume are not yet accepted. Persistent Codex is not yet enabled in the window.

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
