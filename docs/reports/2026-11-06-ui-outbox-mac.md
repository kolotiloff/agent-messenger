# Persistent outbox log: Mac, November 6, 2026

Change on top of `772398e` (PR #11). macOS arm64, Go 1.22.2. The full `go test -race -count=1 -ldflags=-linkmode=external ./...` suite passed. The external linker prints the same LC_DYSYMTAB warnings as before; the tests finish successfully. Linux/amd64 and Windows/amd64 were built with `CGO_ENABLED=0`; the runtime was not run on them in this check.

## What was verified

- The envelope is stored in the client's SQLite `ui_outbox`, with fixed IDs, text, alias and session key. `ui-prepare` does not create requests or attempts.
- Closing and reopening SQLite keeps the prepared envelope. After `ui-submit`, reopening keeps queued. Twelve concurrent submit retries do not create new requests.
- A substituted address and text with the same ID are rejected. A failed outbox update inside a transaction also rolls back the job insert. If a queued record exists but the job has been deleted, retry, a read claiming success, and ACK are forbidden. There is no automatic repair that re-runs work.
- View acknowledgement is idempotent, does not delete the job and does not hide a prepared message. The limit of 100 unacknowledged outbox entries is enforced; acknowledging a queued one frees a slot. A matching old UI message is accepted into the log without a second job. `PRAGMA synchronous = 2` (FULL) was verified.
- An HTTP test simulates a lost ACK after ui-prepare: no further submit happens; three outbox reads create no jobs. Another test loses the response after submit and checks retries. Host/Origin/key protection and bounded JSON are preserved; the new read and ACK are not available without the required method/key.

## Experiment with the real window on a Mac

Isolated HOME, profile, SQLite and working folder. A synthetic binding `codex-check` with `/usr/bin/false` was registered; real Codex was not called. The client ran without model permission, with the queue paused.

1. Two envelopes were saved through the real control API: one prepared, the second queued without a window ACK. The window showed different states and different actions.
2. The tab was closed completely. Only the test client was force-killed with SIGKILL; the UI process was stopped. This simulates an app crash, not a power loss or a Mac reboot.
3. The same directory was opened by a new client and a new UI process. A new tab, a new port and a new key were created; the old sessionStorage was not used. Both envelopes were restored.
4. Acknowledging the queued one removed it from the recovery list while keeping the history. «Восстановить отправку» ("Restore send") for the prepared one moved the original IDs/address/text into the editor. At this stage SQLite still had one job and zero attempts: recovery sent nothing.
5. An explicit double click on redelivery created a second job exactly once. Then a normal send of a new message created a third. After a page reload the recovery list is empty, and all three messages are visible in the history. SQLite: three requests, three acknowledged outbox entries, zero attempts.
6. Test processes were stopped. Working chats, profiles and protective reservations were not used. The Mac's stay-awake mode, requested separately by the user, was left running until it expires.

## Limits

Persistence is guaranteed after the client's first successful write of the envelope. Unsent text in the field and an envelope that did not reach an unavailable client are not yet persistent drafts. SessionStorage remains an additional safeguard for the current tab. Recovery does not repair a deleted/corrupted database and does not repeat model work. Real models were not run in this stage.
