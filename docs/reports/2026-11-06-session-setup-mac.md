# Session setup from the window on Mac

November 6, 2026. Product implementation of preview.18, commit `5c8e4eb`. No real models or work chats were run.

## Window check

Separate temporary HOME, profile, data directory and local client in persistent Claude mode, without model calls allowed. The CLI was replaced by a small test executable that logs its commands.

Setup was opened from the Claude card. An explicit check showed the version and a confirmed login of the test CLI. Then a test session name was entered and the create button was pressed. The dialog closed; the conversation opened on the new session with an empty history.

Afterwards SQLite contained exactly one session in `new` mode and one `setup_new_claude` event. There were no messages, attempts, replies or persistent runs. The queue stayed paused and model calls were not allowed. The working folder was created inside the test data directory. The test CLI log contains only `--version` and `auth status --json` — one call for each of the two explicit diagnostic checks (HTTP and window). The diagnostic response does not contain the control email from the CLI output.

## Automated checks

Verified: atomic writes, eight simultaneous identical requests, refusal when an existing binding would be replaced, a retry after processing has started without changing state, and rejection of an external workspace and a symlinked directory. The HTTP checks cover authorization, Origin, unknown fields, corrupted and concatenated JSON, rejection of arbitrary arguments, a limit on concurrent diagnostics and hiding its internal errors.

The full local suite with race checking, formatting and vet passed on Mac. Checking a real CLI, sending a model message, persistent Codex/Cursor and updating a running background client are not part of this acceptance.

For this commit, all four GitHub Actions jobs also passed: Mac/Linux on Go 1.22.2 and stable. They checked race, vet, compilation of targets and test binaries; Mac stable additionally built and checked the preview packages and the shell lifecycle without a GUI. This does not confirm running the product on Windows. [CI result](https://github.com/kolotiloff/agent-messenger/actions/runs/42192877048).

## Installed app

The signed preview.18 was published to GitHub Releases and installed on this Mac. The version and revision of the bundled binary match the release. After the app update, a full database snapshot matched the previous one: 6 sessions, 13 messages, 10 attempts and 9 replies.

Then the background client was updated separately. Before that, `cli_launch=not-authorized` was confirmed, and a local SQLite backup and the protective reservations file were saved. The old process was terminated with the standard command; the new one was started the usual way via `desktop`, with the same data directory, without model permission. After startup, the pause, 6 held sessions and 3 pending messages were confirmed. Sessions, messages, attempts, replies, persistent runs and recovery records did not change; the reservations file is byte-for-byte the same. The log received restart and pause entries. The protective reservations were not released.

In the installed window, after refreshing statuses, the button for creating a new Claude test session is available. This manual update acceptance does not mean that automatic updating of a running client is already implemented. No new model session was created in the user database and no messages from the old queue were launched.
