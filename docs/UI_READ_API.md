# Data for the first window: overview v1

> **Historical: removed in T-028 ([D-104](DECISIONS.md)).** `overview` and its window route are gone; the window reads `window-view`. This page describes code that no longer exists; it is kept as a record. Old reservations of this mode stay in `active-runs.json` and are listed by `messenger status` and `doctor`.

`messenger --root DIR overview` talks only to an already running client through its local socket. The output is JSON. Waiting for a response is limited to five seconds. A missing client is not started; the CLI, the model and the login check are not called. A timeout does not prove that the client or agent has stopped.

IPC request: `{"cmd":"overview"}`. Response: `{"ok":true,"overview":{...}}`. An error must be shown as data being unavailable, keeping it distinct from an empty session list. The summary does not replace `doctor` diagnostics, and `check-cli` remains a separate explicit action.

## Fields

- `schema_version: 1`, `paused`: version and the global queue pause.
- `sessions`: registered sessions, one card per canonical binding. Several aliases of one session do not create several agents.
- `provider`, `aliases`: provider and user-facing mail names.
- `launch_configured`: the CLI path and working folder are recorded. This is **not** a check that the program exists, nor of its version, login or fitness to launch.
- `authentication`, `readiness`: always `not_checked`. Old successful checks do not turn into current readiness.
- `owner_retained`: the database holds a session owner. This is **not** a sign of a working model and **not** proof of a live process.
- `recorded_attempt_state`, `recorded_stream_state`: states from the database, `none` when there is no record. For example, an `idle` stream may hold the owner between messages; `uncertain` requires investigation. Unknown future states must not be shown as green.
- `decision_pending`: the database has a decision for the session.
- `queued`, `uncertain`: number of messages in the corresponding state in this session. An uncertain old attempt stays in the history and may be counted after recovery; this is not the number of available automatic retries.
- `truncated`, `aliases_truncated`: explicit truncation flags. Maximum 100 sessions and 100 aliases per session, stable session registration order and alphabetical alias order. There is no pagination yet; the interface must show that the list is incomplete.

The read is done in a single transaction without modifying data. The number of rows is limited, but the length of user-entered aliases is not limited by this API. CLI session IDs, canonical keys, profile paths, attempt tokens, message contents and raw database errors are not passed. User aliases may still be personal data; the summary is not intended for publication.

This is a local API with the same socket permissions as before. It does not add HTTP, remote access, discovery of other computers, a coordinator role or a broker. Connection readiness and permission to call models are read separately from `status`. Separate requests do not form a single atomic snapshot.

## Next part

A [timeline](TIMELINE.md) has been added — a bounded message log with pagination: the message, the stored reply and, separately, the confirmed provider result. The presence of reply text does not by itself mean successful execution. The [local window](LOCAL_UI.md) is connected to this data; send and queue-control commands are described separately in its contract.

## Proposed app layout

An interactive concept with demo data was shown in the chat: computers on the left, the conversation in the center, the coordinator role next to it; separate sections «Агенты» ("Agents"), «Очередь» ("Queue"), «Сервер» ("Server"). A card shows CLI installation, login, session and connection independently. The server is chosen on a computer or a VPS; the coordinator is a separate role. Sending and the mockup's toggles do not control real agents.

The user chose the visual style of this mockup; it is kept in [design/v1](../design/README.md) as the basis for implementation. It is not yet a finished product window. Demo success statuses for other OSes and servers are not acceptance results. Requirements remain in [APP.md](APP.md), the sequence in [RELEASE_PLAN.md](RELEASE_PLAN.md).
