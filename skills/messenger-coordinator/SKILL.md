---
name: messenger-coordinator
description: Use when this chat coordinates worker agents through the Agent Messenger MCP tools (list_agents, start_task, wait, tasks, answer, cancel).
---

# Agent Messenger: coordinator tools

This chat is connected to Agent Messenger for one project. Messenger starts worker agents (Claude Code, Codex) in the project's folder, one new session per task, and tells you when they report or ask.

How work is planned and split is decided with the owner under this project's own rules. This skill only explains the tools.

## Tools

- `list_agents` shows the computers, the project folder on each, and their agents with running and waiting task counts. A computer without the project's folder refuses the project's tasks.
- `start_task(op_id, agent, text, title?)` starts a task in a new worker session. Make `op_id` unique per task. Reuse it only to retry the same call after a lost response; Messenger then returns the same task instead of starting a second one. Write the task so a fresh session can do it: the goal, the files or documents to read, and what to report.
- `wait(after, timeout_seconds?)` blocks until something happens in this project: a report, a question for you, a failure, an interruption, a block or a cancellation. Always pass `after` = the `cursor` from the previous `wait` (0 the first time).
  - When it returns events, act on them, then call `wait` again with the new cursor.
  - When it times out with no events, call `wait` again without comment.
  - Waiting costs no tokens.
- `tasks` lists the project's tasks, their states and any open question for you, plus the current cursor.
- `answer(task_id, question_id, op_id, text)` answers a worker's question. The worker continues in the same session.
- `cancel(task_id)` cancels a task.

## Rules

- A report reaches you only after Messenger verified the run: the right session, a normal end and exit, and exactly one report.
- After `failed`, `interrupted` or `blocked`, do not start the task again by yourself. Tell the owner what happened and let them decide.
- Questions a worker asks the owner go to the Messenger window, not to you.
- To talk to the owner while waiting, the owner can interrupt the wait; call `wait` again afterwards with the same cursor. Nothing is lost.
