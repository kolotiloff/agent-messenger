# Launching only the selected session

> **Superseded.** The preview.19 test session from the window; that button and the session path are gone ([TASK_MODEL.md](TASK_MODEL.md)).

In preview.19, launching a new test Claude session is available from the window: «Агенты» ("Agents") → Claude Code → «Сессии» ("Sessions") → «Запустить тестовую сессию» ("Launch test session"). It can be created via «Настроить» ("Set up"). The button explicitly permits message processing for the selected session only and resumes the queue. If messages are already queued for this session, they will start being processed. Other recipients stay in the queue. Replies spend tokens; idle waiting sends no model requests.

The button is available only for a new test session created by Messenger that has no owner. Older registered and working conversations are not connected by this action. While one permission is active in the client, it cannot be switched to another session. Repeating the confirmation returns the same result and does not resume a subsequent pause. The permission is not kept after the background client stops; closing one window is not the same as stopping the client. After an attempt, a protective reservation may remain — the button does not bypass it.

For the terminal there is the `--only-session ADDRESS` restriction for `serve` and `start` in `--claude-stream` mode. It does not by itself permit model calls. `--allow-model-calls` is required, and background `start` begins paused.

The client picks messages only for the exact registered session of the given address. Its other aliases refer to the same conversation. Messages for other sessions stay in the queue even if the selected address does not exist, its queue is empty, or a protective reservation is held on it. In that case there is no moving on to the next agent. A wrong provider and incomplete setup are rejected before an attempt is created. The adapter additionally checks the selected address before launching the process.

`status` returns `launch_session`; the window shows the restriction and does not promise to process messages of other sessions. A regular `desktop` launch enables neither this permission nor the model. Changing the launch scope requires a normal stop of the client and a new explicit launch; it does not clear earlier protective reservations. A pause keeps the live process and the session lock.

Isolated checks with a test CLI: two sequential messages for one session were processed by one PID; an older message for another session was not taken; a restart does not bypass the held selected session. Passing the restriction to the background process and the absence of automatic selection of another queue were verified. Real models are not launched by these tests.

On Mac the whole path through the window was completed, first with the fake CLI and then with real Claude Code 2.1.318: creation, permission, two messages, two linked confirmed replies in one process. Older messages of other sessions stayed in the queue. Replies and the queue refresh automatically; reading local data does not invoke the model. When viewing older history, the scroll position is not reset; a fresh page is loaded when the session is selected again or on manual refresh. After the real check, the queue was paused with the live owner kept. [Report](reports/2026-11-06-window-launch-mac.md).
