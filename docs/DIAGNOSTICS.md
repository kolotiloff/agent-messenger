# Diagnostics without launching a CLI

```sh
messenger --root /absolute/data/path doctor
messenger --root /absolute/data/path doctor --json
```

The command works even when the client is stopped. It looks for executable candidates in `PATH`, checks that the data directory exists, and requests only `status` from an existing local client. It does not create the directory/database, install programs, launch a CLI, read credentials or change locks. Waiting for the control socket's response is limited to 750 ms; unavailability is not treated as proof that processes have stopped.

This is the passive part of diagnostics for the future window: version, CLI authenticity, login and session binding stay `not_checked` here. For a CLI chosen by the user, a [check-cli check](CLI_CHECK.md) with a limited Mac matrix is implemented separately. The presence of an executable does not mean "the agent is ready". Absence from `PATH` does not prove the program is absent from the whole computer; a GUI app may have a different `PATH`.

Search names: `claude`, `codex`, and for Cursor first `cursor-agent`, then `agent`. The last name may belong to another program: until the version/identity is verified, it is only a candidate. Arbitrary found programs are not executed for diagnostics. A relative path rejected by Go as `ErrDot` is not taken as a found CLI.

## Machine format, version 1

`doctor --json` outputs a single object:

- `schema_version`, `checked_at` (UTC), `os`, `arch`;
- `data.root` and `data.state`: `exists`, `missing`, `not_directory`, `unreadable` — only information about the directory, not the result of a database check;
- `controller.state`: `reachable`, `unreachable` or `unknown`; with a confirmed status — client PID, pause, CLI permission, number of pending messages/held sessions and the existing broker status;
- `agents`: three independent cards, `cli.state`, candidate path, command name, `PATH` source, `identity`, `version`, `authentication`, `session`;
- `probe_cli_starts=0`: the diagnostic operation itself did not launch a CLI. This is not a counter of actions by another already running client.

Conversation text, conversation IDs, MCP tokens, profile contents, raw errors and the extended `status.detail` field are not included in the report. Paths and PIDs are local diagnostics: anonymize them before publishing.

`held_sessions` includes any held owner, including a healthy persistent process; it is not an error count. Broker status `not_configured` does not mean a network connection. Per-session readiness is not derived from the aggregate counters.

Exit code 0 means "report generated", even if the CLI was not found or the client did not respond. For automation, read the JSON fields. Unknown flags are rejected; the command has no permission to launch models.

## Next parts

The window's agent cards now check the chosen CLI themselves ([CLI_CHECK.md](CLI_CHECK.md)). An explicit limited check of the version and login of the chosen CLI, including a path outside PATH, is described in [CLI_CHECK.md](CLI_CHECK.md). A login error must not wipe the profile or offer re-authorization unnecessarily.
