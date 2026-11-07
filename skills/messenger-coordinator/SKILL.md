---
name: messenger-coordinator
description: Use when this chat coordinates worker agents through the Agent Messenger MCP tools (list_agents, start_task, wait, tasks, answer, cancel).
---

# Agent Messenger: coordinator tools

This chat is connected to Agent Messenger for one project. Messenger starts worker agents (Claude Code, Codex) in the project's folder, one new session per task, and tells you when they report or ask.

How work is planned and split is decided with the owner under this project's own rules. This skill only explains the tools.

## Tools

- `list_agents` shows the computers, the project folder on each, and their agents with running and waiting task counts. A computer without the project's folder refuses the project's tasks.
- `start_task(op_id, agent, text, title?, model?)` starts a task in a new worker session. Make `op_id` unique per task. Reuse it only to retry the same call after a lost response; Messenger then returns the same task instead of starting a second one. Write the task so a fresh session can do it: the goal, the files or documents to read, and what to report.
- `start_task` also takes an optional `model`. Leave it out to use the agent's default model; see "Choosing a model" below.
- `wait(after, timeout_seconds?)` blocks until something happens in this project: a report, a question for you, a failure, an interruption, a block or a cancellation. Always pass `after` = the `cursor` from the previous `wait` (0 the first time).
  - When it returns events, act on them, then call `wait` again with the new cursor.
  - When it returns `more: true`, further events are ready: act on these, then call `wait` again at once with the new cursor. One call returns a bounded batch.
  - When it times out with no events, call `wait` again without comment.
  - Waiting costs no tokens.
- `tasks(state?, limit?, if_version?)` lists the project's open tasks (not done, failed or cancelled), newest first, with any open question for you, the current cursor and a `version`. `state: "all"` lists every task; a state name lists only that state. Pass `if_version` from the previous call to get `unchanged: true` instead of the same list. You rarely need `tasks`: `wait` already tells you what happened.
- `answer(task_id, question_id, op_id, text)` answers a worker's question. The worker continues in the same session.
- `cancel(task_id)` cancels a task.

## Choosing a model

Most tasks need no `model`: the agent's default is set by the owner in that CLI. Name a model only when the task clearly calls for it, or when the owner asked for one.

- Claude Code: `opus` for hard work (design, tricky bugs, large refactors, careful review), `sonnet` for ordinary coding, `haiku` for small mechanical work (renames, formatting, simple lookups). A full model id such as `claude-sonnet-5-5` also works.
- Codex: pass the exact model name the owner uses in Codex (for example a `-codex` model for coding). If you do not know which names this computer's Codex accepts, leave `model` out.
- Do not guess model names. A name the CLI does not know makes the run fail; Messenger then reports `failed` and does not retry.
- The model stays the same for the whole task, including runs that continue after an answer.
- Tell the owner which model you chose when it is not the default, and why, in one line.

## Rules

- A report reaches you only after Messenger verified the run: the right session, a normal end and exit, and exactly one report.
- After `failed`, `interrupted` or `blocked`, do not start the task again by yourself. Tell the owner what happened and let them decide.
- A failure caused by a usage or rate limit says so (`Reason: usage limit; it resets at …`). Tell the owner the reset time; do not start the task again before it.
- Questions a worker asks the owner go to the Messenger window, not to you.
- To talk to the owner while waiting, the owner can interrupt the wait; call `wait` again afterwards with the same cursor. Nothing is lost.
