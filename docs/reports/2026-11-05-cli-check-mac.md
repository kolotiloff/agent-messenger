# CLI version and login check on Mac

A change on top of `7db2ae55d01fa91b9170caba615d1c352163248c` (PR #6). Added an explicit `check-cli` for chosen absolute CLI paths and a profile. The passive `doctor` still does not execute the programs it finds.

## Verified natively

On the Mac, the following responses of the standard commands were obtained through the new Messenger implementation:

| CLI | Version | Login status |
|---|---|---|
| Claude Code | 2.1.318 | authenticated |
| Codex CLI | 0.155.1 | authenticated |
| Cursor Agent CLI | 2026.11.05-86047df | authenticated |

For each, only `--version` and a status query were run: Claude `auth status --json`, Codex `login status`, Cursor `status --format json`. Browser login, logout, prompts and session resume were not performed. No models were called. A detected login means a CLI response, not a check of billing/quota or of the model's network reachability.

A wrong provider-directory override was deliberately passed into each probe's environment; explicitly selecting the regular profile correctly removed it. All three commands saw the existing login. Re-authentication was not required.

The hashes of the Messenger database, the WAL/SHM if present, and the shared reservation registry matched the originals after each check. The local client was not started. Raw account stdout/stderr was not saved; the reports contain no email, userInfo, organization or keys. `session` and compatibility with Messenger execution remained `not_checked`.

## Automated checks

On macOS arm64, Go 1.22.2, the full suite passed with the race detector:

```sh
go test -race -tags streamprobe -ldflags=-linkmode=external -count=1 ./...
```

Additional checks cover the exact arguments of the three subcommands, the canonical profile and removal of a foreign override, a temporary working directory outside the project, stopping the check before auth for an unknown version, not publishing raw output, mandatory explicit paths, an unsupported OS, replacement of the executable between steps, positive/negative/ambiguous responses, the output memory limit and the timeout of a real fake process.

After renaming the JSON counter to `probe_command_attempts` and refining the doctor hint text, the diagnostics profile suite was rerun. The counter means attempts to execute diagnostic commands, not a proven number of successfully started processes. Product algorithms did not change between the full and profile runs.

`CGO_ENABLED=0` builds for linux/amd64 and windows/amd64 passed. There was no native CLI acceptance on those OSes; there check-cli does not launch programs and returns an unconfirmed platform.

## Limitations and next step

The exact versions in the table were verified. After a CLI update, the login query will stop at `version_not_qualified_for_auth_probe` until the new contract is verified. The version reported by the executable is not a check of the binary's provenance.

The code does not release session locks, does not claim to stop detached descendants, and does not grant readiness for working conversations. The provider may maintain its own cache/credentials inside the standard command; unchanged provider profile files are not claimed.

The next layer is status cards bound to the chosen path/profile, and a log for the future window. Browser authorization, installation, compatibility of persistent adapters and releasing a reservation within the same boot remain separate work.

Contracts, sources and commands: [CLI_CHECK.md](../CLI_CHECK.md).
