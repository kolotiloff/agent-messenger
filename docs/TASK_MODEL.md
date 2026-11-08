# Task model

One task, one worker session (D-021). This document describes what is implemented. [RETHINK.md](RETHINK.md) explains why.

## Projects and folders (D-025, D-026)

A **project** has a name and, on each computer, at most one folder (`project_folders`). The folder must be:
- an existing absolute directory;
- not a temporary folder;
- not the disk root.

A task records the folder it was started with, so changing the folder affects only new tasks. A computer without a folder for the project refuses the project's tasks. Each computer has a stable random ID (`meta.computer_id`); another computer of the network is addressed by its broker ID (below).

**Agents** are registered per computer (`computer_agents`): provider (`claude` or `codex`; Cursor stays off), the CLI path, an optional profile directory, and the worker profile:
- `worker`: normal tools and the project's settings (D-023);
- `messenger-only`: automated tests.

```sh
messenger --root DIR project-add --name Game --folder ~/Projects/Game
messenger --root DIR agent-set --provider claude --cli "$(command -v claude)"
messenger --root DIR agent-set --provider codex --cli "$(command -v codex)"
messenger --root DIR serve --allow-tasks
```

## Lifecycle

1. **Start.** `StartTask` records the task and the launch intent of its first run in one transaction. Nothing is launched at this point.
   - **Idempotent.** It is keyed by `(project, op_id)`: a repeated call returns the same task, and the same `op_id` with different arguments is refused.
   - **Session ID.** A Claude task gets a new session UUID here. A Codex task gets its thread ID, and a Cursor task its chat ID, from the first run's events.
   - **Model (D-030).** An optional model name chosen by the coordinator is stored on the task (`tasks.model`) and is part of the idempotency key. Every run of the task gets it: Claude `--model NAME`, Codex `-c model="NAME"`, Cursor `--model NAME`. A name must match `^[A-Za-z0-9][A-Za-z0-9._:/\[\]-]{0,79}$`, so it can never be read as a flag. No model means the CLI's default. An unknown name fails the run like any other failure; it is not retried. The owner lists the names each agent accepts (D-058, T-043); `list_agents` returns them, and Codex and Cursor take only a listed name, checked by the computer that runs the task.
2. **Launch.** The client launches recorded runs only with `serve --allow-tasks` and only while the queue is not paused. There is no limit on parallel workers (D-022).
   - `launching` is committed before the reservation in `active-runs.json` is written, and the reservation before exec.
   - The worker runs with the task's folder as its working directory.
   - Worker permissions: Claude runs with `--permission-mode acceptEdits` (D-028), Codex with `--sandbox workspace-write` (D-023), Cursor with `--force --trust --approve-mcps` (D-049).
3. **Identity.** The run's first conversation ID from the event stream is bound:
   - a Claude ID must equal the recorded UUID;
   - a new Codex or Cursor task binds the first thread or chat ID once, and the ID may not belong to another task;
   - a resumed run must report the recorded ID.

   The worker tools stay closed until the ID is bound.
4. **Worker tools** (`messenger task-mcp`, bound to the run and its receipt; for Cursor through `~/.cursor/mcp.json` and the `AGENT_MESSENGER_TASK` variable, D-049): `read_task`, then exactly one of `report`, `ask` or `ask_owner`. On a resumed run `read_task` returns the question and the answer without the task text, which the resumed conversation already has; `include_task: true` returns it (T-033). The disposition is stored provisionally.
5. **Verification** (`FinishTaskRun`, one transaction). After the exit, the client signals the process group and waits up to 3 s for it to be empty. Then:

| Evidence | Run | Task | Published |
|---|---|---|---|
| exit 0, empty group, identity bound, provider completed, one disposition | `finished` | `done` / `waiting_coordinator` / `waiting_owner` | the report or question |
| exit and empty group observed, anything else wrong | `failed` | `failed` | a `failed` event with the unverified text |
| exit or empty group not observed | `blocked` | `blocked` | a `blocked` event |
| cancelled, exit and empty group observed | `cancelled` | `cancelled` | a `cancelled` event; any report is dropped |

   The receipt is revoked in all four cases. The reservation is retired only for `finished`, `failed` and `cancelled`; a blocked run keeps it.
6. **Question and answer.** A question for the coordinator goes to the coordinator's events. A question for the owner goes only to the window (D-024).
   - `AnswerTask(task, question, op_id, text, by)` accepts only the newest open question, only from its addressee.
   - It is idempotent by `op_id`, and a late or second answer is refused.
   - It records the next run, which resumes the same conversation by exact ID: `claude --resume ID` or `codex exec … resume ID`.
7. **Cancel** (coordinator `cancel`, the window's stop button, or `messenger task-cancel --task ID`; all use `CancelTask`). A task that is waiting, or whose run is only recorded, is cancelled at once. A running worker's process group is signalled. The task is then `cancelled` when the exit and an empty group are observed, otherwise `blocked`.
8. **Restart.** A run that was `launching` or `running` when the client stopped becomes `interrupted`, and so does its task. Its reservation stays, and it is never relaunched. A recorded intent that never launched is launched after the restart.

Nothing is retried automatically. Starting again means a new task.

## Events

`task_events` is an append-only log per project. Its `seq` is a replayable cursor, and reading never advances anything. The audience is either `coordinator` (started, report, question, answered, failed, blocked, interrupted, cancelled) or `owner` (owner questions and their answers).

## Replies in the old paths

Replies of the persistent Claude, Codex and one-shot paths are published only after verification (T-022, [ROUTING.md](ROUTING.md)).

## Limits

- One computer. The broker (T-015) will carry tasks to other computers.
- There is no OS sandbox. A worker's leftover processes are handled on a best-effort basis (D-021), and a still-busy process group blocks the task.
- `worker` profile flags for real CLIs were not run against real models here (T-027).

## Coordinator (T-025)

The coordinator is the owner's chat with Messenger connected as an MCP server for one project (D-026). The window connects it by itself (D-050, T-039): right after a project is created, or on «Подключить к Claude», the client runs the command below in the project folder with the registered Claude Code program, and checks `~/.claude.json` for the result. The owner then opens a new chat in that folder. The same command, for doing it by hand:

```sh
cd ~/Projects/Game
claude mcp add --scope local messenger -- "/Applications/Agent Messenger.app/Contents/Resources/messenger" coordinator --project Game
```

`messenger coordinator --project NAME` is a stdio MCP server. It opens the data directory's database and writes task records. It never launches a worker itself; the running client launches the workers (`--allow-tasks`). The skill [skills/messenger-coordinator/SKILL.md](../skills/messenger-coordinator/SKILL.md) explains the tools and nothing else (D-022).

| Tool | Effect |
|---|---|
| `list_agents` | This computer: its project folder and its agents, with running and waiting task counts (across all projects). |
| `start_task(op_id, agent, text, title?, computer?, model?)` | `StartTask`; idempotent by `op_id`. `model` is optional (D-030); the server instructions and the skill say when to choose one and not to guess names. |
| `wait(after, timeout_seconds?)` | Returns coordinator events of this project after the cursor, at most 20 and about 24,000 characters of text per call (always at least one event), or `timed_out` after the timeout (default 600 s, maximum 3600 s). The returned `cursor` is the last returned event; `more: true` says further events are ready (T-031). |
| `tasks(state?, limit?, if_version?)` | Open tasks by default (not `done`, `failed`, `cancelled`); `state: "all"` or one state name. Default 50, at most 200, with `more: true` when cut. No folder per row; a `done` task has no `detail`. Returns the cursor and a `version` that changes with any new event, new task or task update; `if_version` equal to it returns only `unchanged: true` (T-032). Failed tasks carry `failure_reason` / `failure_reset` (T-034). |
| `answer(task_id, question_id, op_id, text)` | `AnswerTask` as the coordinator. |
| `cancel(task_id)` | `CancelTask`. |

How `wait` behaves (the contract of the [T-023 pilot](reports/2026-11-06-wait-pilot-claude-mac.md)):
- Each call runs in its own goroutine, so a blocked `wait` does not stop `ping`, other tools or cancellation.
- The cursor is replayable, and the server never moves it.
- A cancelled call is not answered and loses nothing.
- When the client sent a progress token, a progress notification goes out every 60 s.

Events the coordinator caused itself (`started`, its own `answered`) do not wake it. Neither does the owner's exchange with a worker.

Every tool refuses a project that does not exist. `answer` and `cancel` refuse tasks of another project. The first call records the connection (`coordinator_connections`); the window uses it to show whether a coordinator is connected.

## Window (T-029)

The window observes tasks; the owner talks to the coordinator in the Claude app (D-024). Layout, from the approved [mockup](mockups/2026-11-06-task-window.html), built from the existing `app.css` components and `app.js` structure:

- **Sidebar.**
  - A project switcher (`am-select`) whose last item is "Новый проект…". It opens a dialog for the name and the folder on this computer. In the Mac app, "Выбрать…" opens a standard folder panel through the `folder` bridge.
  - The computer card. It lists the coordinator (when connected, with the muted word "координатор") and each agent program, one line each, with a status dot:
    - blue: working;
    - amber: waits for the owner;
    - hollow: idle;
    - grey: no connection.

    The full status is in the tooltip. Agent states count all projects (D-026).
- **"Задачи" tab.**
  - **Task list** on the right: active tasks first, then finished ones, with an agent filter. Each row has three lines: the agent, the title and a coloured status ("Работает N мин", "Ждёт вашего ответа", "Завершил за N мин", "Прервался через N мин", …).
  - **Task feed.** The coordinator's text and answers, and the owner's answers, are shown as outgoing bubbles. The worker's text, its tool actions (collapsible lines from the CLI event stream, stored in `task_run_items`, at most 400 per run), its report or question with a delivery note, and the outcome of a failed, blocked, interrupted or cancelled run are shown in order.
  - **Header.** Title, then the agent with its model when one was chosen, the computer and the folder, then the status line.
  - **Input field and stop button.** While a task is active the field is shown with a square stop button, like a chat ("Остановить", Esc). Stop cancels the task at once (no second confirmation), through the same `CancelTask` path as the coordinator. The field accepts text only while the worker waits for the owner; otherwise it reads "Исполнитель работает…" or "Задача ждёт запуска…". Enter sends, Shift+Enter makes a new line. An answer uses a stable `op_id` stored per question, so a retry cannot answer twice.
  - **First steps.** Without a project, the window offers to create one. Without a connected coordinator and with no tasks, it shows whether Claude has the project (connected, not yet, Claude Code not set up under «Агенты», another `messenger` entry, or the last attempt's error) with a «Подключить к Claude» button; the `claude mcp add …` command stays available under «Подключить вручную через Терминал».
  - **Notices.** One says that tasks are recorded but the client runs without `--allow-tasks`; the client the window starts always allows tasks (D-029), so this shows only for a client started by hand. Another says that the queue is paused, with "Продолжить".
- **"Агенты" tab.**
  - The project folder bar: icon, label, path and "Изменить".
  - One card per agent. The role is a muted line ("Координатор", "Исполнитель", "Не подключён"). The rows Программа, Вход в аккаунт and Рабочая папка are coloured green, yellow, red or grey.
  - The trust row shows "Доверяет" or "Не доверяет" with a "?" hint and "Проверить снова".
  - The tasks button reads "Задачи: N идёт, M сегодня" or "Задач нет". "Настроить" checks the CLI and connects it as a worker.
- **Removed.** The general composer, the queue panel and the manual refresh button are gone. The window refreshes every 2 s. The Mac menu item "Обновить задачи и статусы" calls `window.messengerRefresh`.
- **No "·" separators** in the window and the Mac shell.

API (authenticated as before):
- reads: `GET /api/window?project=`, `GET /api/task?id=`;
- actions: `POST /api/project-create`, `/api/project-folder`, `/api/task-answer`, `/api/task-cancel`, `/api/agent-set`.

The client re-checks every rule. The old session and timeline endpoints stay until T-028.

Verified in headless Chromium (Linux) on a test data directory, in light and dark themes:
- the task list and feed;
- an owner question and the answer from the window;
- a new project from the switcher, with the connection command;
- the Agents tab.

The page logged no errors. Not verified: the Mac app (WKWebView, the folder panel bridge); that needs the next build.

## Tasks between computers (T-037)

A coordinator on computer A can give a task to an agent on computer B of the same network (D-041). `list_agents` lists B with the agents it reported and whether it is online; `start_task` takes B's ID (`host` or `dev-…`) as `computer`.

- **On A** the task is a *remote task*: it has no runs and launches nothing. It records `remote_broker` and `remote_peer`. Its state, questions, report and outcome are what B reports. Its feed is built from those events, B's acceptance and the worker's text and tool lines that B sends as `progress` (up to 200 per run, for the window only, D-055).
- **On B** the task is an ordinary task (a *mirror*, with `origin_broker`, `origin_peer`, `origin_task`) that runs in B's folder of the project with the same name. It is verified exactly like a local task. Only A answers its questions: B's window shows the question but does not offer to answer it, and B's own coordinator does not see the task (its events are history on B). B's owner may still stop it.
- **B accepts** a task unless its owner turned off «Задачи других компьютеров» on the «Сервер» tab (on by default, D-056), a project with that name has a folder on B, and the agent is set up there. Otherwise A's task fails with B's reason.
- **Messages** (`net_outbox`, written in the same transaction as the change they report): A→B `start`, `answer`, `cancel`; B→A `accepted` (with B's folder), `refused`, `progress`, `event` (an event for the coordinator or the owner with the state after it), `answer_refused`. The client sends the outbox in order and reads its inbox every 3 s. Every message is applied once (`net_applied`) together with the read cursor.
- **Delivery retries** (D-042). A message the broker did not confirm is sent again in the next round with the same operation ID. This repeats a delivery, never an action: the broker stores an operation once and the receiver applies it once. When the broker refuses a message because the other computer left the network, a `start` fails its task (it never reached B) and an `answer` or `cancel` blocks it (what happens on B is unknown). Nothing is retried after that.
- **Cancel.** A start that has not left A yet is withdrawn and the task is cancelled at once. Otherwise A asks B and shows the task as running until B reports the outcome.
- **Not covered.** A remote task whose network changed or whose computer is unreachable keeps its last state; there is no timeout. A computer that joined its own server receives tasks only under `host`.
