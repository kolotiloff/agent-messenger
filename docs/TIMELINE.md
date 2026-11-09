# Local message timeline v1

> **Historical: removed in T-028 ([D-104](DECISIONS.md)).** `timeline` and its window route are gone. This page describes code that no longer exists; it is kept as a record. Old reservations of this mode stay in `active-runs.json` and are listed by `messenger status` and `doctor`.

```sh
messenger --root DIR timeline --alias claude-test --limit 50
messenger --root DIR timeline --alias claude-test --limit 50 --before MESSAGE_ID
```

The command reads the existing client through the local socket, does not start a CLI/model, does not create attempts, does not acknowledge delivery and does not change locks. The response is JSON. Waiting is limited to five seconds; if the client is unavailable, an error is returned rather than an empty successful history.

IPC request: `{"cmd":"timeline","alias":"claude-test","limit":50,"before":"..."}`. `before` is optional; it is a Messenger message ID. A missing/zero `limit` in IPC means 50; the allowed explicit CLI limit is 1..100. Response: `{"ok":true,"timeline":{...}}`.

## Pages

Without a cursor, the latest page is returned. Within a page, messages go from oldest to newest in database write order. `has_more` and `next_before` give the next, older page, which the UI prepends at the top. The cursor must point to an existing message of the same session; an unknown or foreign cursor is rejected. Two aliases of the same session read the same history. New messages do not shift an already chosen page boundary.

This is a current snapshot of one page, not an immutable export of the whole history. A reply or result for a previously read message may appear later; the screen must refresh the corresponding page. Timestamps may coincide or go backwards, so they are not used as a cursor.

## Message and reply

`items` is always an array. A message has `id`, `sender`, `text`, `created_at`, `state`, `retry_of` for a retry, `attempt_alias` when there is an attempt, `attempt_state`, `reply`, `result` and `exit_code`.

The top-level `session_alias` is the viewing address chosen by the user. The old schema does not store the original alias used when enqueuing; we do not present the chosen name as the historical recipient. `attempt_alias` is the address recorded when the processing reservation was made. A stored reply has its own `id`, `reply_to`, `sender`, `recipient`, `text`.

The old schema does not store reply time, so the API does not invent it. This is the Messenger log, not the full history or reasoning of the CLI. Network ACKs are also not represented here: the broker is not yet implemented.

## Processing result

- `not_verified`: the result is not confirmed. This may be a message not yet started, a stored reply without a result, or incomplete information.
- `verified_turn`: the session identity, the provider result and the exact link to the reply are confirmed; the turn of the persistent process has completed. There is no exit code; the process may have kept running.
- `verified_exit`: the same information is confirmed and exit code 0 was observed. The attempt state may still remain `uncertain` because the protective reservation is held.
- `process_failed`: a non-zero exit code was observed. The reply text may be stored, but a green success is not shown.

The reply and attempt IDs, the original message, and the sender and recipient of the reply are cross-checked. Stored text, `provider_ok` without the other evidence, or an empty process group is not sufficient. No result gives the UI the right to release a reservation or to automatically retry a message.

## Limits and data

At most 100 messages per page. The text of each message/reply is limited to 16,000 characters by SQLite, with an explicit `text_truncated`; the original in the database is not changed. The UI must mark the truncation and must not use such a fragment as the full text for resending. A full export does not exist yet. This API does not limit the length of IDs and user addresses, so the row limit is not a strict limit on response bytes.

CLI IDs, the profile path, attempt tokens, PIDs and raw diagnostic details are not passed through. Message contents and addresses are deliberately passed through and may be confidential: they must not be automatically published or written to publicly accessible logs. The UI renders them as text, not executable HTML. Access remains local with the existing socket permissions.

Related material: [cards](UI_READ_API.md), [chosen style](../design/README.md), [plan](RELEASE_PLAN.md).
