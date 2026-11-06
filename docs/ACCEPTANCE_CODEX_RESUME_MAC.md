# T-009: resume the Codex test thread on the Mac

This runbook resumes the Codex test session from T-004, `codex-test-3a73811a`, with a new app-server after a reboot. It then sends one real message to the same thread. It uses the owner's Codex account and spends a little of its quota. It touches nothing else: no working chats, no default `~/.codex` profile, no Cursor.

The message asks Codex which word it wrote in its previous answer. That answer was "ПРИВЕТ". A correct answer shows that the earlier history survived the resume.

## Before you start

- **A build with T-009.** You need preview.22 or later. Once a newer client opens the data directory, preview.21 refuses it with "written by a newer Agent Messenger", so update the app before you start.
- **A restart since the T-004 run.** The recovery needs proof that the Mac has booted again since that app-server started. If you have not restarted since then, restart the Mac yourself first. Nothing here restarts it.
- **The Codex profile.** `~/AgentMessengerTest/codex-profile` from T-004 must still be logged in.

## 1. Stop the app and its client

Quit Agent Messenger (Cmd+Q). Then:

```bash
M="/Applications/Agent Messenger.app/Contents/Resources/messenger"
R="$HOME/Library/Application Support/AgentMessenger"
"$M" version
"$M" --root "$R" shutdown
```

"client unavailable" is fine: it means the client is already stopped.

## 2. Check the current state

```bash
curl -fsSL https://raw.githubusercontent.com/kolotiloff/agent-messenger/main/scripts/codex-acceptance-summary.py -o ~/AgentMessengerTest/summary.py
"$M" --root "$R" start
"$M" --root "$R" inspect | python3 ~/AgentMessengerTest/summary.py codex-test-3a73811a
"$M" --root "$R" shutdown
```

Expected: `setup: state=uncertain`. If it is `created`, a client still owns the thread: stop here and send the output.

## 3. Recover with resume

```bash
"$M" --root "$R" recover-codex --alias codex-test-3a73811a --confirm preserve-uncertain --resume
```

Expected: "резерв сессии Codex снят после доказанной перезагрузки; сессия оставлена открытой…" ("the Codex session reservation was removed after a proven reboot; the session is kept open…"). If the command says the reboot is not proven, restart the Mac and run it again. On any other refusal, stop and send the output.

## 4. Start a client and resume the thread

```bash
"$M" --root "$R" start --allow-model-calls --claude-stream --only-session codex-test-3a73811a
"$M" --root "$R" resume-codex-session --alias codex-test-3a73811a --cli "$(command -v codex)"
```

`resume-codex-session` checks the Codex config and resumes the saved thread; it does not call the model. If it reports an uncertain setup, stop here and send the output. Do not repeat the command.

## 5. One message

```bash
"$M" --root "$R" enqueue --alias codex-test-3a73811a --id t009-1 --from tester --text "Одним словом: какое слово ты написал в своём предыдущем ответе?"
"$M" --root "$R" resume
```

The message asks, in one word, which word Codex wrote in its previous answer. Wait about a minute, then check:

```bash
"$M" --root "$R" timeline --alias codex-test-3a73811a
```

## 6. Summary

```bash
"$M" --root "$R" inspect | python3 ~/AgentMessengerTest/summary.py codex-test-3a73811a --resume
```

The summary prints states and reply texts only, with no thread IDs or tokens. Send it back.

**Expected result:**

- `setup: state=created`;
- `t009-1` is `completed`, and its reply is "ПРИВЕТ" or close to it;
- `resumed=True` and `processes_with_completed_turns=2`;
- `RESULT: PASS`.

## 7. Finish

```bash
"$M" --root "$R" pause
"$M" --root "$R" shutdown
```

Then open Agent Messenger as usual. As after T-004, the owner becomes uncertain on the next start. It can be resumed again after the next reboot, or closed with `recover-codex` without `--resume`.
