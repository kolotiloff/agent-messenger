# Sending and queue control from the window: Mac, November 6, 2026

Basis: the local UI from `ae9a3f4`, then the changes in this PR. macOS arm64, Go 1.22.2. This is acceptance of the local queue and the HTTP/browser interface, not a call to real models and not a network broker.

## Real checks

- The full `go test -race -count=1 -ldflags=-linkmode=external ./...` passed on Mac. The external linker is needed for this Go/macOS combination; `LC_DYSYMTAB` warnings remained, the tests completed successfully.
- New HTTP tests: required Origin/key/JSON, rejection of extra fields and arbitrary control commands, a size limit, the sender address set correctly by the code, a request deadline, and no passing off of a queue write as provider success.
- Real SQLite in the test: the message is written, then the caller simulates losing the response. HTTP reports `unconfirmed`. Twelve concurrent retries with the same ID give `created=false`; changed text with the same ID is not accepted. A second ownership of the session is rejected.
- The native binary was built and signed with a local ad-hoc signature to run the test. A separate HOME, profiles, working folder and SQLite were created. `serve` was started without model authorization; the CLI in the test bindings is `/usr/bin/false`. No working or GUI sessions were used.
- In the browser, a double click on «Отправить» ("Send") persisted one message. It is visible in the history as pending, with no model confirmation. Verified by a separate SQLite read.
- «Возобновить очередь» ("Resume queue") and «Приостановить очередь» ("Pause queue") changed the client status. `cli_launch=not-authorized` stayed unchanged, and no new model attempts appeared. There was no active model work in this test; this on-screen check does not prove that such work continues while paused. Existing tests verify that a live persistent process is kept between turns and that the next turn is blocked while paused.
- For the second message the client was stopped before sending, and the UI was left running. The window showed an unknown result, kept the text/address/ID and locked editing. Reloading the same tab restored the same ID. After starting the same client and reloading the window there was no automatic send: the database still had only the first message.
- An explicit double click on «Повторить доставку» ("Retry delivery") persisted the second message exactly once and with the original ID. In total, two new `local-user` records, both `queued`; no new model attempts or replies. Losing the response **after** the write was verified by the HTTP/SQLite test; the browser interruption was **before** the write.
- The new elements were checked in a normal window and a narrow 320 px window; there is no horizontal overflow of the root area. The original styling `design/v1/styles.css` was not changed.
- `CGO_ENABLED=0` builds for Linux/amd64 and Windows/amd64 passed. The application was not run on those OSes in this check.
- The test UIs and both test client runs were stopped. The protective reservations of working sessions were not touched.

## Limits of the result

An ACK means only that the message was persisted, not that the model executed successfully. A transport retry uses the same ID and does not repeat the model attempt. If the user deliberately creates a new message with a new ID, that is a different job.

An unconfirmed envelope is currently stored in the sessionStorage of one tab of one UI run. Fully closing the window/browser or restarting the UI is not accepted as safe recovery of this envelope. A persistent outbox log is the next separate stage, before native packaging. Messages already accepted into SQLite do not depend on the tab's lifetime.

Server functions, CLI authorization, session registration from the window and coordinator switching have not been added. Verification of persistent Codex/Cursor and of safe owner shutdown remains in the plan.
