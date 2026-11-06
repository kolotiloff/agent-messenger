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
