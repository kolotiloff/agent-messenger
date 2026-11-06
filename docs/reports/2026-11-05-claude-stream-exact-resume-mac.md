# Exact Claude resume in the persistent client on Mac

Verified: product commit `026131ce8392e377c3dc4ed059724c7c6330f3cf`, Claude Code `2.1.318`, macOS arm64. The binary was built with Go 1.22.2 with the external linker and a local ad-hoc signature. This check used a real model.

## Isolation

A separate saved CLI session from the earlier resume test was chosen. It was absent from the registry of guarded runs; there were no active commands with its exact ID before the test. Its saved file and earlier working folder were located via private test artifacts. No GUI chat or working conversation was used.

The earlier test queue directory was used, because changing the directory must not bypass the shared reservations. Before the test, the only old message in the queue belonged to a busy session. First the client was started without model authorization, the queue was paused, and the separate session was registered in `resume` mode.

## Result of one message

- One user message was authorized. One attempt was created; there were no automatic retries.
- The live process's arguments confirmed `--resume` with the exact registered ID; `--session-id` was absent.
- Claude returned a marker from the earlier test conversation. The marker was not included in the new message. This further confirms that context was preserved.
- The reply is linked to the control message, addressed to the test coordinator; `provider_ok=1`. The persistent channel moved to `idle`.
- After the check the queue was paused again, and `shutdown` stopped Messenger. Starting without model authorization made it possible to check the persisted state without resending.
- Old messages, replies and attempts did not change; the earlier control message remained in the queue.
- The new run received the pre-recorded `stream_run_id` and a correct `recovery_epoch`.
- A real `recover-stream` within the same OS boot refused with the reason `same boot`. The reservation file stayed byte-for-byte unchanged.

After the stop, no active commands with the test ID were found. The process exit was not recorded in the database in time: `exit_observed=0`, `exit_code=null`. A zero exit code is not claimed; the reservation is kept.

## Test script correction

The script's first summary report stopped on an overly strict comparison of the JSON objects of the old reservations. Serialization with the new structure added an empty `recovery_epoch` field (`version=0`, empty `host` and `boot`) to four earlier records. None of the earlier values changed; these records still fail the recovery check.

A separate check confirmed that all old fields were unchanged and that this addition was the only difference. Then a negative `recover-stream` check was performed. The model was not called again. The overall result of the one-shot test was accepted.

## Preservation and limits

A private backup of the test database was made under the supervisor/startup locks after the stop, with no unfinished WAL. It is needed because the original test directory is temporary and may disappear on reboot. Private IDs, the marker, CLI logs, the database, backups and the computer identifier values are not included in Git.

At the time of this test there had been no real reboot and no positive acceptance of `recover-stream` after one. The follow-up check was done and is described in a [separate report](2026-11-05-claude-stream-reboot-mac.md). The verified real resume by itself does not replace that acceptance. Recovery within the same OS boot, working chats, other providers, Windows/Linux in operation and billing during long waiting were not tested by this test.

The PR also keeps two additional audit tests in `pr4_audit_test.go`: rollback of the final SQLite transaction after the reservation file is deleted, and several generations of one session with a partial reply. Together with the full suite they passed on Mac with the race detector during the audit of `026131c`; this report change does not modify product code.
