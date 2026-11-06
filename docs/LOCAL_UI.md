# Local UI: messages and queue

This is a technical preview in the browser, connected to the real local API. It is not the native macOS shell and not an installer. The saved [design/v1](../design/README.md) styling has been carried over into the product; HTML, CSS and JavaScript are embedded in the Go binary, and no external CDN or Node is needed to run it.

## Running

```sh
messenger --root DATA_DIR ui
```

The command prints a private local link. Open it in a browser on the same computer. `--port 0` (the default) picks a free port. Only the `127.0.0.1` address is available; remote connections are not enabled.

A `serve`/`start` client with the same directory must already be running. `ui` does not start it itself, does not open SQLite directly and does not change the protective reservations. If the client is stopped, the window shows that data is unavailable. Do not start a different directory to get around a held session. To test without models, use a client without `--allow-model-calls`.

Closing the tab does not stop the client or the UI process. Ctrl+C in the `ui` terminal stops only the HTTP interface, not `serve` and not the CLI. No services or autostart are installed.

## What works

- Cards for registered sessions: aliases, recorded state, owner hold, queue. Login and readiness remain unverified; a discovered CLI is not presented as a ready agent.
- History of the selected session, linked replies and the check result. The last 50 messages, then loading of earlier pages. Message markup is rendered as plain text. Truncated content is clearly marked.
- A queue summary and passive discovery of CLI candidates in PATH.
- Explicit sending to the selected registered session, pausing and resuming the processing of new messages. A response confirming that a message was stored does not mean a model reply or model success.
- Manual refresh, observation time, an explicit unavailability error. The screen does not poll the model itself; there is no background refresh timer.
- The layout of the chosen design, the browser's light/dark theme and a narrow window. Scrolling of a long conversation is confined to its own area.

Not yet present: account login, creating a session from the window, server configuration, changing the coordinator, and other computers. These features are not presented as working. Outgoing messages are stored in the client database after Send is pressed; fully closing the window and reopening it has been verified on a Mac. [Preview packaging and local installation](INSTALL.md) have been added; the remaining adapter and distribution checks are not yet closed.

`status` and `overview` are read separately and are not a single atomic snapshot. This is an observation, not permission to take actions. When a command runs, the client's own checks apply; the state on screen may already be stale.

## Sending and transport retry

Select a session, type a message and press «Отправить» ("Send"). The sender is fixed in code as `local-user`. Before the HTTP request, the window creates a `ui-UUID` and writes the immutable address, text and ID to sessionStorage. The local HTTP server then calls `ui-prepare`: the client stores the envelope in `ui_outbox` in the same SQLite, bound exactly to the registered session. Only after this write is confirmed is `ui-submit` called. Preparation by itself does not create a job for the model. If the storage write/check fails, nothing is sent. A double click is blocked immediately.

On a positive ACK the field is cleared. The label confirms only the write to SQLite. If model runs are allowed on the running client and the queue is not paused, processing may start immediately; sending does not guarantee that no tokens will be spent. Merely showing the window and waiting do not call the model.

If the response is lost, it is unknown whether the message arrived. The window keeps the envelope, locks editing and changing the recipient, and offers «Повторить доставку» ("Retry delivery"). The retry sends the same ID, address and text. SQLite accepts such a retry without creating a second message; different text under the same ID is rejected. Reloading the same tab restores the envelope. Neither page load nor reconnection sends it automatically. A new model run, `recover-failed` and repeating already performed actions are not invoked here.

**Recovery after closing:** a new window reads `ui_outbox` from the same client, even if the UI port and key have changed or the browser tab was fully closed. `prepared` means the envelope was written but delivery to the queue was not confirmed; the button restores the original envelope into the field, and then an explicit «Повторить доставку» is required. `queued` means the message has already been written to the queue: it must not be sent again. «Понятно, убрать из списка» ("Got it, remove from list") acknowledges that it was seen and keeps the message itself and the history. This is not deletion and not a repeat of model work.

Inserting the job and moving the outgoing entry to `queued` happen in one transaction. If the process exits before it, a prepared envelope remains; after it, an already accepted message. `ui-ack` records the window's acknowledgement and does not affect processing. If this ACK is lost, the window shows the already accepted message again, without resending. If the text/route does not match, or the job is missing while `queued` is recorded, the client refuses; corruption does not permit a retry. SQLite uses WAL and an explicit `synchronous(FULL)`.

Until saved sends have been resolved, the window blocks creating new ones, so that the user does not create a new job in place of an unknown one. Up to 100 unacknowledged envelopes are kept at a time; older acknowledged records remain in the database but are not loaded into the recovery list. The address, text and ID of a saved envelope cannot be edited.

**Persistence boundary:** text that has only been typed and not yet sent is not a saved draft. If the client is unavailable and `ui-prepare` was not written, the envelope exists only in the current tab. Fully closing before the client's first confirmed write still does not guarantee it is saved. The interface does not promise or suggest otherwise. Loss or manual deletion of the database itself is also not covered by window recovery. There is no automatic replay of prepared messages.

## Pause

«Приостановить очередь» ("Pause queue") stops handing new messages out for work. Work that is already active continues. «Возобновить очередь» ("Resume queue") lifts this pause, but does not change permission to run models, does not release the protective reservations and does not resume a session on its own. If the confirmation is lost, the command is not retried automatically; the screen re-reads the actual state.

## Local access

The link contains a random key for the current run in the URL fragment. The page moves it into this tab's sessionStorage and removes it from the address bar. Data requests require this key in a request header. The link must not be published or committed to git; restarting the UI creates a new key.

The server checks the exact Host and Origin and rejects cross-site requests. Five read operations are available via GET: status, overview, timeline, passive discovery and outbox. Only `/api/enqueue`, `/api/pause`, `/api/resume`, `/api/outbox/ack` accept POST with the key, a mandatory exact Origin and JSON. Unknown fields, trailing JSON and oversized requests are rejected. There is no general proxy for control commands. Caching is disabled, embedding into another page is forbidden, and scripts and styles are loaded only from the same local address. Raw client errors and diagnostic details are not passed through. The trusted local user and their browser extensions remain inside the trust boundary.

The template works without a network; reading requires the local process. On Windows, limitations of the local client and IPC still remain: a successful build of the interface does not mean the full product is supported on Windows.

Implementation references: [Go net/http](https://pkg.go.dev/net/http#Server), [Content Security Policy](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Content-Security-Policy). Test results: [Mac report](reports/2026-11-06-local-ui-mac.md).

## Window command contract

| POST | JSON | Successful ACK |
|---|---|---|
| `/api/enqueue` | `message_id` (`ui-` + UUID in lowercase hex), `alias`, `text` | `ok: true`, the same `message_id`, `created: true/false` |
| `/api/pause` | `{}` | `ok: true`, `action: "pause"` |
| `/api/resume` | `{}` | `ok: true`, `action: "resume"` |
| `/api/outbox/ack` | `message_id` of an already accepted message | `ok: true`, `action: "ui-ack"` |

The body limit is 64 KiB and the text limit is 16000 bytes; the browser field is limited to 4000 UTF-16 code units. On a control API error/timeout, the server responds 503 with `state: "unconfirmed"`; this is not proof that nothing was written. The page does not receive raw error details. A missing mandatory `created` confirmation is also treated as an uncertain result.

Testing of sending, pausing, connection drops and retries: [Mac report](reports/2026-11-06-ui-actions-mac.md).

`GET /api/outbox` returns `schema_version: 1` and `items` with `message_id`, `alias`, `text`, `state`, `created_at`. Data is read through the control API; the UI does not open the database directly. An old client without these commands must be updated: the window does not bypass the outbox journal with the old enqueue. Testing of full closing and failure: [Mac report](reports/2026-11-06-ui-outbox-mac.md).
