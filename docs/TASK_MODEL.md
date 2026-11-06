# Task model

One task, one worker session (D-021). This document describes what is implemented. [RETHINK.md](RETHINK.md) explains why.

## Projects and folders (D-025, D-026)

A **project** has a name and, on each computer, at most one folder (`project_folders`). The folder must be:
- an existing absolute directory;
- not a temporary folder;
- not the disk root.

A task records the folder it was started with, so changing the folder affects only new tasks. A computer without a folder for the project refuses the project's tasks. Until the network broker exists, only this computer runs tasks; it has a stable random ID (`meta.computer_id`).

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
   - **Session ID.** A Claude task gets a new session UUID here. A Codex task gets its thread ID from the first run's events.
2. **Launch.** The client launches recorded runs only with `serve --allow-tasks` and only while the queue is not paused. There is no limit on parallel workers (D-022).
   - `launching` is committed before the reservation in `active-runs.json` is written, and the reservation before exec.
   - The worker runs with the task's folder as its working directory.
3. **Identity.** The run's first conversation ID from the event stream is bound:
   - a Claude ID must equal the recorded UUID;
   - a new Codex task binds the first thread ID once, and the ID may not belong to another task;
   - a resumed run must report the recorded ID.

   The worker tools stay closed until the ID is bound.
4. **Worker tools** (`messenger task-mcp`, bound to the run and its receipt): `read_task`, then exactly one of `report`, `ask` or `ask_owner`. The disposition is stored provisionally.
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
7. **Cancel.** A task that is waiting, or whose run is only recorded, is cancelled at once. A running worker's process group is signalled. The task is then `cancelled` when the exit and an empty group are observed, otherwise `blocked`.
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

The coordinator is the owner's chat with Messenger connected as an MCP server for one project (D-026):

```sh
cd ~/Projects/Game
claude mcp add --scope local messenger -- "/Applications/Agent Messenger.app/Contents/Resources/messenger" coordinator --project Game
```

`messenger coordinator --project NAME` is a stdio MCP server. It opens the data directory's database and writes task records. It never launches a worker itself; the running client launches the workers (`--allow-tasks`). The skill [skills/messenger-coordinator/SKILL.md](../skills/messenger-coordinator/SKILL.md) explains the tools and nothing else (D-022).

| Tool | Effect |
|---|---|
| `list_agents` | This computer: its project folder and its agents, with running and waiting task counts (across all projects). |
| `start_task(op_id, agent, text, title?, computer?)` | `StartTask`; idempotent by `op_id`. |
| `wait(after, timeout_seconds?)` | Returns coordinator events of this project after the cursor, at most 20 per call, or `timed_out` after the timeout (default 600 s, maximum 3600 s). |
| `tasks(state?)` | This project's tasks, open questions for the coordinator, and the current cursor. |
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
  - **Answer field.** It appears only while the worker waits for the owner. An answer uses a stable `op_id` stored per question, so a retry cannot answer twice.
  - **First steps.** Without a project, the window offers to create one. Without a connected coordinator and with no tasks, it shows the `claude mcp add …` command with a copy button.
  - **Notices.** One says that tasks are recorded but the client runs without `--allow-tasks` (Q-009). Another says that the queue is paused, with "Продолжить".
- **"Агенты" tab.**
  - The project folder bar: icon, label, path and "Изменить".
  - One card per agent. The role is a muted line ("Координатор", "Исполнитель", "Не подключён"). The rows Программа, Вход в аккаунт and Рабочая папка are coloured green, yellow, red or grey.
  - The trust row shows "Доверяет" or "Не доверяет" with a "?" hint and "Проверить снова".
  - The tasks button reads "Задачи: N идёт, M сегодня" or "Задач нет". "Настроить" checks the CLI and connects it as a worker.
- **Removed.** The general composer, the queue panel and the manual refresh button are gone. The window refreshes every 2 s. The Mac menu item "Обновить задачи и статусы" calls `window.messengerRefresh`.
- **No "·" separators** in the window and the Mac shell.

API (authenticated as before):
- reads: `GET /api/window?project=`, `GET /api/task?id=`;
- actions: `POST /api/project-create`, `/api/project-folder`, `/api/task-answer`, `/api/agent-set`.

The client re-checks every rule. The old session and timeline endpoints stay until T-028.

Verified in headless Chromium (Linux) on a test data directory, in light and dark themes:
- the task list and feed;
- an owner question and the answer from the window;
- a new project from the switcher, with the connection command;
- the Agents tab.

The page logged no errors. Not verified: the Mac app (WKWebView, the folder panel bridge); that needs the next build.
