# T-004: first real Codex run on the Mac

This runbook covers two real messages in a new, separate Codex test session, served by one live app-server. It uses the owner's Codex account and spends a little of its quota. Nothing else is touched: no working chats, no default `~/.codex` profile, no Cursor.

Exact resume of the saved thread after a restart is **not** part of this run. Resuming a Codex thread with a new owner is not implemented yet.

## What you need

- The Codex CLI installed (`command -v codex` prints a path).
- A build of `messenger` from `main` that includes Codex delivery (PR #29 or later). preview.19 does not have it. Go is not needed, because the build comes from GitHub Actions.
- 10–15 minutes and a browser for the Codex login.

Data stays in the app's normal data directory, so the window shows the Codex session. Never use `/tmp` for data.

## 1. Get the build

1. Open the latest green run of "Client checks" for `main` in GitHub Actions.
2. Download the artifact `mac-preview-<sha>`; you must be logged in to GitHub.
3. Unpack it:

```bash
mkdir -p ~/AgentMessengerTest/bin && cd ~/AgentMessengerTest/bin
unzip -o ~/Downloads/mac-preview-*.zip
tar -xzf agent-messenger-*-darwin-arm64.tar.gz
xattr -dr com.apple.quarantine ~/AgentMessengerTest/bin
M=$(find ~/AgentMessengerTest/bin -name messenger -type f -perm -u+x | head -1); echo "$M"; "$M" version
```

`xattr` removes the download quarantine. Without it macOS refuses the unsigned test binary.

## 2. Stop the app's client

Quit Agent Messenger (Cmd+Q). Then stop its background client; only one client may run per user:

```bash
R="$HOME/Library/Application Support/AgentMessenger"
"$M" --root "$R" shutdown
```

"client unavailable" is fine: it means the client is already stopped.

## 3. A separate Codex profile with its own login

```bash
mkdir -p ~/AgentMessengerTest/codex-profile
CODEX_HOME=~/AgentMessengerTest/codex-profile codex login -c 'cli_auth_credentials_store="file"'
CODEX_HOME=~/AgentMessengerTest/codex-profile codex login status -c 'cli_auth_credentials_store="file"'
codex --version
```

Log in through the browser that opens. The login is stored in that folder only. Do not point `CODEX_HOME` at `~/.codex`.

## 4. Start a client that may call only this session

```bash
"$M" --root "$R" start --allow-model-calls --claude-stream --only-session codex-test
"$M" --root "$R" create-codex-session --environment ~/AgentMessengerTest/codex-profile --alias codex-test --cli "$(command -v codex)"
```

The client starts paused. `create-codex-session` checks the Codex config and creates a new thread; it does not call the model. If it reports an uncertain setup, stop here and send the output.

## 5. Two messages

```bash
"$M" --root "$R" enqueue --alias codex-test --id t004-1 --from tester --text "Reply with exactly: ACK:ONE"
"$M" --root "$R" resume
```

Wait about a minute, then check:

```bash
"$M" --root "$R" timeline --alias codex-test
```

When the reply is there, send the second message:

```bash
"$M" --root "$R" enqueue --alias codex-test --id t004-2 --from tester --text "Reply with exactly: ACK:TWO"
```

## 6. Summary

```bash
curl -fsSL https://raw.githubusercontent.com/kolotiloff/agent-messenger/main/scripts/codex-acceptance-summary.py -o ~/AgentMessengerTest/summary.py
"$M" --root "$R" inspect | python3 ~/AgentMessengerTest/summary.py codex-test
```

The summary prints states and reply texts only, with no thread IDs or tokens. Send it back.

**Expected result:**

- `setup: state=created`;
- both messages `completed`, with their replies;
- `distinct_processes=1` and `verified_turns=2`;
- `RESULT: PASS`.

## 7. Finish

```bash
"$M" --root "$R" pause
"$M" --root "$R" shutdown
```

Then open Agent Messenger as usual.

**What remains after the run.** Shutdown closes the Codex owner, so its setup becomes uncertain on the next start. Its reservation stays in the same data directory, which does not block the app. That reservation can be retired with `recover-codex --alias codex-test --confirm preserve-uncertain` after the next reboot. The `codex-test` alias stays taken until then.
