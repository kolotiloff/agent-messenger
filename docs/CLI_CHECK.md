# Explicit version and login check

`doctor` stays passive: it does not execute programs found in PATH. For a CLI the user has chosen there is a separate command:

```sh
messenger check-cli --provider claude \
  --cli /absolute/path/to/claude \
  --environment /absolute/path/to/.claude --json
```

For Codex, pass `--provider codex`, the path to its CLI and profile; for Cursor, `--provider cursor`, the path to `cursor-agent`/`agent` and the profile. Both paths are required, must be absolute and must exist. Arbitrary `agent` binaries in PATH are neither searched for nor executed automatically. The CLI path may be outside PATH.

The command does not open sessions, send a prompt, call login/logout or lift locks. It runs only `--version` and, for a verified version/OS contract, the standard status request. This executes the chosen program: use a trusted installed CLI. The version string is not a check of the file's digital signature or authenticity.

## Verified commands and versions

On a Mac, specifically the diagnostic commands of the following versions were verified:

| Provider | Version | Status command |
|---|---|---|
| Claude Code | 2.1.318 | `auth status --json` |
| Codex CLI | 0.155.1 | `login status` |
| Cursor Agent CLI | 2026.11.05-86047df | `status --format json` |

If the version has changed, its recognized number is shown, but the login request is not run until the new contract is verified. For an unknown version format the status is unknown. On Linux/Windows this first implementation returns `platform_not_qualified` without executing the CLI. This is a deliberately limited diagnostic matrix; it does not mean that all these CLIs are accepted for persistent Messenger message handling.

Command sources: [Claude CLI reference](https://code.claude.com/docs/en/cli-reference), [Codex developer commands](https://learn.chatgpt.com/docs/developer-commands?surface=cli), [Cursor authentication](https://cursor.com/docs/cli/reference/authentication). Cursor's JSON flag was additionally confirmed by the installed CLI's help. The commands are not replaced with prompt/print on an error or unknown version.

## What the result means

`--json` outputs a version 1 schema with the check time, the selected CLI/profile and independent results:

- `version.state=reported`: the selected CLI's response was recognized; `value` contains only a number in the established format.
- `authentication.state=authenticated`: the standard command reported a login/presence of credentials. This is not a check of the model's network availability, quota, the token's validity on the server or a specific working conversation.
- `not_authenticated`: a recognized negative response was received (Claude/Codex). For a negative Cursor response the contract is not yet accepted, so `unknown` is returned.
- `unknown`: an error, timeout, contradictory exit code, unexpected format or output limit exceeded. No re-login is performed, and it is not automatically suggested as the only fix.
- `not_checked`: the status request was not run, e.g. because of the version/OS.

`session` and `messenger_compatibility` remain `not_checked`. Exit code 0 means a report was produced; automation must look at the state fields and not treat 0 as the agent being ready.

## Profile, environment and secrets

Profile selection uses the same function as message launches. For the default `~/.claude`, `~/.codex`, `~/.cursor`, the corresponding override variable is removed; for a separate canonical directory the selected value is set. This keeps the default profile's standard login and does not inherit someone else's directory setting.

The rest of the environment is inherited. An API key or another standard provider mechanism may affect the effective login; the report explicitly states the scope `selected_profile_and_inherited_environment`. The result is not automatically attributed to another registered session or another environment.

The check's working folder is a new temporary directory outside the project, deleted afterwards. Messenger does not open credential files itself, but the selected CLI reads its storage in the usual way and may maintain its own cache/credentials. No claim is made that the provider's files remain completely unchanged.

Raw stdout/stderr stays only in process memory. Email address, organization, userInfo, keys and their masks, and error messages are not included in the JSON, logs or saved Messenger history. Each stream's output is limited to 64 KiB; overflow gives an unknown result. Unknown version strings are not published.

Each command gets up to 8 seconds, waiting for inherited pipes up to 500 ms. On cancellation/timeout the result is unknown. This bounds the diagnostics and does not prove that all detached descendants stopped; the result is never used to release a session. Messenger never on its own initiative auto-updates the CLI, installs it, or logs into or out of an account.

Results are not written automatically into `doctor`: it keeps `not_checked`. In the window, the «Настроить» ("Configure") card shows the selected paths and a separate «Проверить версию и вход» ("Check version and login") button. Opening the card launches nothing. The check shows the time, path and profile; changing the fields resets the previous result. The report is kept only in the current window and does not count as a check of another session.

On a Mac, passive discovery for the window additionally checks `~/.local/bin`, `/opt/homebrew/bin`, `/usr/local/bin`: an app launched from the Dock usually does not inherit the terminal's PATH. Candidates are not executed until explicitly clicked. Over HTTP only fixed diagnostic commands are available, without passing arbitrary arguments; the local Origin and the window key are checked. One check runs at a time per window server.
