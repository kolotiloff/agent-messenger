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

The diagnostic commands were first verified on a Mac with these versions:

| Provider | Version | Status command |
|---|---|---|
| Claude Code | 2.1.318 | `auth status --json` |
| Codex CLI | 0.155.1 | `login status` |
| Cursor Agent CLI | 2026.11.05-86047df | `status --format json` |

The versions each release was last checked with are in `internal/diagnostics/tested.go` (now Claude Code 2.1.322 and Codex 0.167.2; none recorded for Cursor). When an agent's installed CLI is newer, its card adds «· не проверена» with a hint: a newer version usually works, but if its tasks start to fail, the new version may be why (T-139). Every day `.github/workflows/cli-watch.yml` installs the newest CLIs and runs `scripts/check-cli-surface.sh`, which checks the flags and features Messenger uses without calling a model ([CI.md](CI.md#other-workflows)).

The login request runs for any recognized version and on macOS, Windows and Linux (D-051); only the exact known answers count as logged in or not, anything else is unknown. For an unknown version format the login request is not run. On Windows the requests run without a console window. This is a deliberately limited diagnostic matrix; it does not mean that all these CLIs are accepted for persistent Messenger message handling.

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

Results are not written automatically into `doctor`: it keeps `not_checked`. In the window («Агенты»), each connected agent's card is checked once when the window opens; the card's «Проверить снова» icon repeats the check, and its «Настройки» gear holds the program path, profile and models. Changing the fields resets the previous result. The report is kept only in the current window and does not count as a check of another session.

On a Mac, passive discovery for the window additionally checks `~/.local/bin`, `/opt/homebrew/bin`, `/usr/local/bin`: an app launched from the Dock usually does not inherit the terminal's PATH. Candidates are not executed until explicitly clicked. Over HTTP only fixed diagnostic commands are available, without passing arbitrary arguments; the local Origin and the window key are checked. One check runs at a time per window server.

## Window setup

The agent card's Install button runs the official CLI installer in a visible terminal. Find in agent settings looks through that terminal's PATH, including custom prefixes, and connects/checks the returned executable. Choose file remains available. No terminal result means the outcome is unknown; Messenger does not automatically reinstall. Uninstall opens the official removal instructions.

Sign-in opens the CLI's own account flow. If terminal startup cannot be confirmed, the window reports it with a manual command. Trust on an agent card opens its CLI in the selected project folder so the owner can confirm there; no graphical agent app is required. Messenger does not automatically answer trust or account prompts.
