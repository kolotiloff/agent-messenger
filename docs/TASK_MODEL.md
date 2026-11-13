# Task model

One task, one worker session (D-021). This document describes what is implemented. [RETHINK.md](RETHINK.md) explains why.

## Projects and folders (D-025, D-026)

A **project** has a name and, on each computer, at most one folder (`project_folders`). The folder must be:
- an existing absolute directory;
- not a temporary folder;
- not the disk root.

A task records the folder it was started with, so changing the folder affects only new tasks. A computer without a folder for the project refuses the project's tasks. Each computer has a stable random ID (`meta.computer_id`); another computer of the network is addressed by its broker ID (below).

**Agents** are registered per computer (`computer_agents`): provider (`claude`, `codex` or `cursor`), the CLI path, an optional profile directory, and the worker profile:
- `worker`: normal tools and the project's settings (D-023);
- `messenger-only`: automated tests.

```sh
messenger --root DIR project-add --name Game --folder ~/Projects/Game
messenger --root DIR agent-set --provider claude --cli "$(command -v claude)"
messenger --root DIR agent-set --provider codex --cli "$(command -v codex)"
messenger --root DIR agent-set --provider cursor --cli "$(command -v cursor-agent)"
messenger --root DIR serve --allow-tasks
```

## Lifecycle

1. **Start.** `StartTask` records the task and the launch intent of its first run in one transaction. Nothing is launched at this point.
   - **Idempotent.** It is keyed by `(project, op_id)`: a repeated call returns the same task, and the same `op_id` with different arguments is refused.
   - **Session ID.** A Claude task gets a new session UUID here. A Codex task gets its thread ID, and a Cursor task its chat ID, from the first run's events.
   - **Model (D-030).** An optional model name chosen by the coordinator is stored on the task (`tasks.model`) and is part of the idempotency key. Every run of the task gets it: Claude `--model NAME`, Codex `-c model="NAME"`, Cursor `--model NAME`. A name must match `^[A-Za-z0-9][A-Za-z0-9._:/\[\]-]{0,79}$`, so it can never be read as a flag. No model means the CLI's default. An unknown name fails the run like any other failure; it is not retried. The owner lists the names each agent accepts (D-058, T-043); `list_agents` returns them, and Codex and Cursor take only a listed name, checked by the computer that runs the task. `list_agents` also gives each agent's last reported usage limit (D-059, T-044); it informs the coordinator and changes nothing about how runs are judged.
2. **Launch.** The client launches recorded runs only with `serve --allow-tasks` and only while the queue is not paused. There is no limit on parallel workers (D-022), except that a task with a `lock` waits while an earlier unfinished task with the same lock is on that computer.
   - `launching` is committed before the reservation in `active-runs.json` is written, and the reservation before exec.
   - The worker runs with the task's folder as its working directory.
   - Worker permissions: Claude runs with `--permission-mode acceptEdits` (D-028), Codex with `--sandbox workspace-write` (D-023), Cursor with `--force --trust --approve-mcps --sandbox enabled` (D-049). With «Любые команды» on for the agent (D-065): Claude `--permission-mode bypassPermissions`, Codex `--dangerously-bypass-approvals-and-sandbox`, Cursor also `--sandbox disabled`. A new task copies the setting; an answer or a follow-up takes the agent's current setting (D-067). A newly connected worker agent starts with it on (D-067).
3. **Identity.** The run's first conversation ID from the event stream is bound:
   - a Claude ID must equal the recorded UUID;
   - a new Codex or Cursor task binds the first thread or chat ID once, and the ID may not belong to another task;
   - a resumed run must report the recorded ID.

   The worker tools stay closed until the ID is bound.
4. **Worker tools** (`messenger task-mcp`, bound to the run and its receipt; for Cursor through `~/.cursor/mcp.json` and the `AGENT_MESSENGER_TASK` variable, D-049): `read_task`, then exactly one of `report`, `ask` or `ask_owner`. On a resumed run `read_task` returns the question and the answer without the task text, which the resumed conversation already has; `include_task: true` returns it (T-033). `read_task` also returns the owner's `rules` for the project's workers, copied into the task when it started (D-062). `report` takes optional `changes`, `tests` and `files`, stored as labelled lines of the report text. On a run of `continue_task` (D-063) `read_task` returns the coordinator's `follow_up` and leaves out the task text. The disposition is stored provisionally.
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
   - It records the next run, which resumes the same conversation by exact ID: `claude --resume ID`, `codex exec … resume ID` or Cursor `--resume=ID`.
   - The owner's message from the window to a task that waits for the coordinator is the answer (D-110): it answers the open question as the owner, the coordinator gets `owner_answered`, and its own answer is then refused.
7. **Notes** (D-096, T-114). While a task runs, the owner's window and the coordinator's `note_task` can write to the worker. A worker CLI takes no input while it works, so the note waits for the end of the current run. With a question, the note travels with the answer. With a report, the report is held (shown as an interim report) and the task goes on at once in the same conversation with a `note` run. A task that stops marks its waiting notes undelivered; nothing sends them again.
8. **Follow-up** (`continue_task`, D-063). A `done` or `failed` task gets a new `continue` run that resumes the same conversation with the coordinator's text.
9. **Cancel** (coordinator `cancel`, the window's stop button or «Закрыть», or `messenger task-cancel --task ID`; all use `CancelTask`). A task that is waiting, or whose run is only recorded, is cancelled at once. A running worker's process group is signalled. The task is then `cancelled` when the exit and an empty group are observed, otherwise `blocked`.
10. **Restart.** A run that was `launching` or `running` when the client stopped becomes `interrupted`, and so does its task. Its reservation stays, and it is never relaunched. A recorded intent that never launched is launched after the restart.

Nothing is retried automatically. Starting again means a new task.

By D-021 a run is finished only after an observed exit, an empty process group (after signalling it) and a revoked receipt; any unclear evidence keeps the task blocked. A `blocked` or `interrupted` task stays as it is until someone acts on it. Two explicit commands may close it, and only when its worker is certainly gone: the computer restarted since (another boot ID), or neither the recorded process (same ID and start time) nor any process of its group runs. If the process was never recorded, the check fails or anything still runs, the command is refused and the task stays as it was. Reservations and `active-runs.json` are not touched.
- **D-108.** The coordinator's `continue_task` on an interrupted task of this computer closes its interrupted runs as failed («closed for a follow-up») and continues the same conversation.
- **D-120.** An explicit cancel of a blocked or interrupted task (the owner's «Закрыть» on either computer, the coordinator's `cancel`, or the cancel the giving computer sends) closes its stuck runs and the task as cancelled on the computer that ran it. A refused cancel from another computer comes back to its coordinator as `cancel_refused`.

Nothing closes such a task on its own.

## Events

`task_events` is an append-only log per project. Its `seq` is a replayable cursor, and reading never advances anything. Each event has one audience:
- `coordinator`: what the coordinator must act on or know: `report`, `question`, `failed`, `blocked`, `interrupted`, `cancelled`; read-only copies `owner_question` and `owner_answered` of the owner's exchange with a worker; `owner_note`; and the network events `not_accepted`, `answer_refused`, `continue_refused`, `note_refused`, `cancel_refused` and `late_report` (see [Tasks between computers](#tasks-between-computers-t-037)).
- `owner`: a worker's question to the owner and the owner's answer (D-024).
- `log`: history the coordinator caused itself or that belongs to another computer: `started`, `accepted`, `continued`, the coordinator's own `answered`, and every event of a task another computer gave. It never wakes `wait`.

## Replies in the old paths

Replies of the persistent Claude, Codex and one-shot paths are published only after verification (T-022, [ROUTING.md](ROUTING.md)).

## Limits

- There is no OS sandbox. A worker's leftover processes are handled on a best-effort basis (D-021), and a still-busy process group blocks the task.
- `worker` profile flags for real CLIs were not run against real models here (T-027).
- Tasks can run on other computers of the same network ([below](#tasks-between-computers-t-037), [BROKER.md](BROKER.md)). There is no way to carry a task between computers outside a network.

## Coordinator (T-025)

The coordinator is the owner's chat with Messenger connected as an MCP server for one project (D-026). The window connects it by itself (D-050, T-039): right after a project is created, or on «Подключить к Claude», the client runs the command below in the project folder with the registered Claude Code program, and checks `~/.claude.json` for the result. The owner then opens a new chat in that folder. The same command, for doing it by hand:

```sh
cd ~/Projects/Game
claude mcp add --scope local messenger -- "/Applications/Agent Messenger.app/Contents/Resources/messenger" coordinator --project Game
```

`messenger coordinator --project NAME` is a stdio MCP server. It opens the data directory's database and writes task records. It never launches a worker itself; the running client launches the workers (`--allow-tasks`). The skill [skills/messenger-coordinator/SKILL.md](../skills/messenger-coordinator/SKILL.md) explains the tools and nothing else (D-022).

| Tool | Effect |
|---|---|
| `list_agents` | The project's computers: this one and the other computers of its network, the project folder on this one, and their agents with running and waiting task counts (across all projects), the model names the owner listed (D-058) and the last reported usage limit (D-059). Another computer also shows `online`, `reconnecting`, `accepts_tasks`, `has_project` and `outbox_delayed_minutes` when known. `idle_computers` names online computers that can take this project's tasks and run none now; `network_note` says this computer has not heard from the server for minutes. |
| `start_task(op_id, agent, text, title?, computer?, model?, lock?)` | `StartTask`; idempotent by `op_id`. `agent` is `claude`, `codex` or `cursor`. `model` is optional (D-030); the server instructions and the skill say when to choose one and not to guess names. `lock` names a resource of that computer: a later task with the same lock waits (`waiting_for_lock`) until the earlier one is done, failed or cancelled. Adds `limit_note` when the agent's last limit is close or exhausted. |
| `wait(after, timeout_seconds?)` | Returns coordinator events of this project after the cursor, at most 20 and about 24,000 characters of text per call (always at least one event), or `timed_out` after the timeout (default 60 s, maximum 120 s). The returned `cursor` is the last returned event; `more: true` says further events are ready (T-031). It may add `idle_computers`, the `watch` command and `superseded: true` (another chat took over, T-090). |
| `tasks(state?, limit?, if_version?)` | Open tasks by default (not `done`, `failed`, `cancelled`); `state: "all"` or one state name. Default 50, at most 200, with `more: true` when cut. No folder per row; a `done` task has no `detail`. Returns the cursor and a `version` that changes with any new event, new task or task update; `if_version` equal to it returns only `unchanged: true` (T-032). A row may carry `question`, `failure_reason` / `failure_reset` (T-034), `last_activity` / `last_activity_at`, `tokens`, `waiting_for_lock` and `full_access`. A task on another computer may also carry `contact_lost` with `computer_last_seen` (T-126), `computer_outbox_delayed_minutes` (T-150), and `worker` (`running` or `none`) with `worker_output_at` (T-149). |
| `answer(task_id, question_id, op_id, text)` | `AnswerTask` as the coordinator, also for a question to the owner when the owner asks for it (D-112). |
| `continue_task(task_id, op_id, text)` | A follow-up in the same worker conversation (D-063): a done or failed task here or on another computer of the network (D-113), or an interrupted task of this computer once its worker is certainly gone (D-108). |
| `note_task(task_id, op_id, text)` | A note to a working worker here or on another computer that runs 0.67 or newer and is in contact (T-114, D-122). The worker reads it when its current run ends. |
| `task_output(task_id, run?, offset?)` | The worker's full last message of a run (up to 60,000 characters, 20,000 per call; T-087). |
| `task_messages(task_id, last?)` | The last messages of a task's feed, as the window shows them (default 20, at most 100; T-112). Read only. |
| `cancel(task_id)` | `CancelTask`. |
| `report_feedback(text)` | Records a remark about Messenger in this computer's feedback journal (D-117). Nothing is sent anywhere. |
| `export_feedback()` | Writes the whole feedback journal to one zip file and returns its path. |
| `get_log(computer?, lines?, request_id?)` | Asks another computer of the network for the end of its Messenger log (D-083). It answers only when its owner turned on «Отдавать журнал по сети». Waits up to 20 s; otherwise call again with `request_id`. |

How `wait` behaves (the contract of the [T-023 pilot](reports/2026-11-06-wait-pilot-claude-mac.md)):
- Each call runs in its own goroutine, so a blocked `wait` does not stop `ping`, other tools or cancellation.
- The cursor is replayable, and the server never moves it.
- A cancelled call is not answered and loses nothing.
- When the client sent a progress token, a progress notification goes out every 60 s.

**Background watch (T-120).** A chat blocked in `wait` cannot hear the owner. So the coordinator runs this command in the background with its shell tool and ends its turn:

```sh
messenger --root DIR coordinator-watch --project NAME --after CURSOR [--minutes 30]
```

It exits when a coordinator event of the project arrives after the cursor, or after 30 minutes with none (the most allowed). The chat is woken with its output, reads the events with `wait` and starts the watch again with the new cursor. The watch only looks: it never moves the cursor. The server instructions and every `wait` and `tasks` result give the exact command line.

Events the coordinator caused itself (`started`, its own `answered`) do not wake it. The owner's exchange with a worker reaches it only as read-only copies (`owner_question`, `owner_answered`) and `owner_note`. Other tools' answers end with a «Messenger: … new events» note when events are waiting.

Every tool refuses a project that does not exist. Tools that take a task refuse tasks of another project. The first call records the connection (`coordinator_connections`); the window uses it to show whether a coordinator is connected. One chat is the project's coordinator at a time: the newest chat that waits or acts (T-090).

## Window (T-029)

The window observes tasks; the owner talks to the coordinator in the Claude app (D-024). Layout, from the approved [mockup](mockups/2026-11-06-task-window.html), built from the existing `app.css` components and `app.js` structure:

- **Sidebar.**
  - A project switcher (`am-select`) whose last item is «Новый проект». It opens a dialog for the name and the folder on this computer. In the Mac app, «Выбрать» opens a standard folder panel through the `folder` bridge.
  - The computer cards: this computer and the other computers of its network. A card lists the coordinator (when connected) and each agent program, one line each, with a status dot:
    - blue: working;
    - amber: waits for the owner;
    - hollow: idle;
    - grey: no connection.

    The full status is in the tooltip. Agent states count all projects (D-026). A click on a card shows only that computer's tasks and agents. Another computer's card says «Был на связи …» when it stopped reporting, offers «Обновить до …» when it runs an older version, and has a right-click menu to revoke it.
- **«Задачи» tab.**
  - **Task list** on the right, with an agent filter, in three sections:
    - «Активные»: running and waiting tasks;
    - «Требуют решения»: tasks whose outcome is unknown: blocked or interrupted tasks, own tasks on another computer with no contact or no answer, and other computers' open tasks while that computer is out of contact. Nothing closes them by itself. The section's «Закрыть все» closes every one of them after a confirmation;
    - «Завершённые».

    Each row has three lines: the agent and the computer, the title, and a short coloured status with the time and what the task spent («Работает N мин», «Ждёт вашего ответа», «Готово за N мин», «Связь потеряна, исход неизвестен», …). Other computers' tasks from the shared list are shown read-only (T-111). The right-click menu has «Удалить из списка» (T-131, D-114): it only hides a task that no longer runs; the task stays in the database.
  - **Task feed.** The coordinator's text, answers and notes, and the owner's answers and notes, are shown as outgoing bubbles. A note shows whether the worker got it. The worker's text, its tool actions (collapsible lines from the CLI event stream, stored in `task_run_items`, at most 400 per run), its report or question with a delivery note, and the outcome of a failed, blocked, interrupted or cancelled run are shown in order.
  - **Header.** Title, then the agent with its model, the computer and the folder, then the status line.
  - **Reply box and stop button.** While a task is active the box is shown with a square stop button, like a chat («Остановить», Esc). Stop cancels the task at once (no second confirmation), through the same `CancelTask` path as the coordinator. The box answers the worker's question to the owner («Ответить исполнителю…»). For a running task of this computer it writes a note to the worker («Написать исполнителю…», T-114, D-096), and for one that waits for the coordinator it answers in the coordinator's place («Ответить вместо координатора…», D-110). Otherwise it is disabled and says why: «Задача ждёт запуска…», «Исполнитель работает…», or that the other computer's replies are late or its worker is not running. Enter sends, Shift+Enter makes a new line. An answer uses a stable `op_id` stored per question, so a retry cannot answer twice.
  - **Lost or stuck task bar.** An own task on another computer that cannot be reached, or a blocked or interrupted task, has no reply box. A bar says what is known and offers «Закрыть» (after a confirmation). For another computer's task it closes the task here; the cancel goes there if contact returns. For a stuck task of this computer it follows D-120.
  - **First steps.** Without a project, the window offers to create one. Without a connected coordinator and with no tasks, it shows whether Claude has the project (connected, not yet, Claude Code not set up under «Агенты», another `messenger` entry, or the last attempt's error) with a «Подключить к Claude» button; the `claude mcp add …` command stays available under «Подключить вручную через Терминал».
  - **Notices.** One says that tasks are recorded but the client runs without `--allow-tasks`; the client the window starts always allows tasks (D-029), so this shows only for a client started by hand. Another says that the queue is paused, with «Продолжить».
- **«Агенты» tab.**
  - The «Любые команды» switch for every agent of this computer (D-065), and under it «И для задач с других компьютеров» (D-109).
  - One card per agent (Claude Code, Codex, Cursor). The header has «Проверить снова», «Настройки» and a switch that lets Messenger give the agent tasks. The rows: the CLI («Установить» or «Удалить»), the sign-in («Войти» or «Выйти»), «Работает с Messenger» (or «Не доверяет папке проекта»), and the last reported limit.
  - The tasks button reads «Задачи: N идёт, M сегодня» and filters the list by the agent. Another computer's agents are shown read-only.
- **«Проект» tab.** The project's name, its folder on this computer («Выбрать»), «Общие указания исполнителям» (the rules copied into each new task, D-062) and «Удалить проект».
- **«Сервер» tab.** This computer's name in the network, «Принимать задачи других компьютеров», «Отдавать журнал по сети», the server on this computer and the connection to another server ([BROKER.md](BROKER.md)).
- **«Статистика» tab.** Tokens for today, 7 or 30 days, by agent and by computer, and the most costly tasks.
- **Removed.** The general composer, the queue panel and the manual refresh button are gone. The window refreshes every 2 s. The Mac menu item «Обновить задачи и статусы» calls `window.messengerRefresh`.
- **No "·" separators** in the window and the Mac shell.

API (authenticated as before):
- reads: `GET /api/window?project=`, `GET /api/task?id=`;
- task actions: `POST /api/task-answer`, `/api/task-note`, `/api/task-cancel`, `/api/task-hide`;
- project actions: `POST /api/project-create`, `/api/project-folder`, `/api/project-rules`, `/api/project-title`, `/api/project-remove`;
- agent actions: `POST /api/agent-set`, `/api/agent-access`, `/api/agent-pause` and others.

The client re-checks every rule. The old session and timeline endpoints stay until T-028.

Verified in headless Chromium (Linux) on a test data directory, in light and dark themes:
- the task list and feed;
- an owner question and the answer from the window;
- a new project from the switcher, with the connection command;
- the Agents tab.

The page logged no errors. Not verified: the Mac app (WKWebView, the folder panel bridge); that needs the next build.

## Tasks between computers (T-037)

A coordinator on computer A can give a task to an agent on computer B of the same network (D-041). The network and its mailbox are described in [BROKER.md](BROKER.md). `list_agents` lists B with the agents it reported and whether it is online; `start_task` takes B's ID (`host` or `dev-…`) as `computer`.

- **On A** the task is a *remote task*: it has no runs and launches nothing. It records `remote_broker` and `remote_peer`, and B's lasting computer ID (`remote_computer`). Its state, questions, report and outcome are what B reports. Its feed is built from those events, B's acceptance, the worker's full last message (`output`) and the worker's text and tool lines that B sends as `progress` (up to 400 per task, for the window only, D-055).
- **On B** the task is an ordinary task (a *mirror*, with `origin_broker`, `origin_peer`, `origin_task`, `origin_computer`) that runs in B's folder of the project with the same name. It is verified exactly like a local task. Only A answers its questions: B's window shows the question but does not offer to answer it, and B's own coordinator does not see the task (its events are history on B). B's owner may still stop it.
- **B accepts** a task unless its owner turned off «Принимать задачи других компьютеров» on the «Сервер» tab (on by default, D-056), a project with that name has a folder on B, and the agent is set up there. Otherwise A's task fails with B's reason. Tasks from the network take the agent's «Любые команды» unless B's owner turned off «И для задач с других компьютеров» (D-109).
- **Messages** (`net_outbox`, written in the same transaction as the change they report):
  - A→B: `start`, `answer`, `continue` (D-113), `note` (D-122), `cancel`;
  - B→A: `accepted` (with B's folder), `refused`, `progress`, `output`, `event` (an event for the coordinator or the owner with the state after it), `answer_refused`, `continue_refused`, `note_refused`;
  - either way, not about a task: `log_request` / `log` (D-083) and `update_request` / `update` (remote update, T-078).

  The client sends the outbox in order and reads its inbox every 3 s. Every message is applied once (`net_applied`, by its operation ID) together with the read cursor.
- **Delivery retries** (D-042). A message the broker did not confirm is sent again in the next round with the same operation ID. This repeats a delivery, never an action: the broker stores an operation once and the receiver applies it once. When the broker refuses a message because the other computer is not in the network:
  - a `start` fails its task (it never reached B);
  - a `continue` is not run, and the task goes back to done or failed;
  - an `answer` or `cancel` blocks it (what happens on B is unknown);
  - B's replies (`accepted`, `refused`, `event`, `output` and the refusals) are parked, not dropped (D-121): they stay in the outbox and go again when A is listed again under the same ID or moves to a new one.

  Nothing else is retried.
- **A new network** (D-115). Every computer keeps a lasting random ID and puts it on its presence and messages. When a computer shows up in a new network, its tasks' links move to its new peer ID, and queued messages go there. A `start` is never re-sent through the new network.
- **Acceptance.** For 5 minutes A shows «Ждёт, когда «B» примет». A task not accepted by then shows «Нет ответа от «B»», and the coordinator gets one `not_accepted` event with the likely reason (T-134, D-116). Nothing is retried.
- **Contact.** While B has not reported for 45 s, or is no longer in A's network, an open task there shows «Связь потеряна, исход неизвестен», and `tasks` marks it `contact_lost` (T-126). It keeps its last state; when B is back, it goes on reporting. When B is in contact but has not handed its messages to the server for minutes, A shows that its replies are late (`computer_outbox_delayed_minutes`, T-150). B also reports whether a worker runs each task of A's now, with the time of its last output (`worker`, `worker_output_at`, T-149). These only inform: nothing is closed, cancelled or retried because of them.
- **Notes and answers.** The coordinator's `note_task` reaches B's worker as a `note` message when B runs 0.67 or newer and is in contact; a refusal comes back as `note_refused` (D-122). The owner's window writes notes only to tasks of its own computer. When B refuses an answer and says the question is still open, the question opens again on A (`answer_refused`); any other refusal blocks the task (D-116).
- **Cancel and close.** A start that has not left A yet is withdrawn and the task is cancelled at once. Otherwise A asks B and shows the task as running until B reports the outcome. A task out of contact, never accepted, without an answer to its cancel for 2 minutes, or blocked or interrupted, closes on A at once («Закрыть», the coordinator's `cancel`; T-131, D-114); the cancel is still sent to B if it is in the network. On B an explicit cancel closes a stuck task when its worker is certainly gone (D-120); otherwise B answers `cancel_refused`.
- **Late reports.** A report or failure that arrives after the task was closed on A is shown to the coordinator as `late_report` and does not reopen the task (T-150).
- **Not covered.** A computer that joined its own server receives tasks only under `host`. Tasks from networks before 0.61 have no computer ID and are closed by hand (D-115).
