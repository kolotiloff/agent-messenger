# Worker models and effort

Researched 2026-11-14, T-171. These are starting selections, not a live account inventory or a Unity benchmark. Choose by task scope; verify the result with the project's own tests. Availability depends on the account, provider deployment and CLI version. A refusal never causes an automatic model switch or retry.

Pass the exact value from `list_agents.computers[].agents[].models` as `start_task.model`. `model_options` explains recognized selections. Omit `model` to use the CLI's own default. Messenger stores the complete selection with the task and uses it on exact-session continuations too.

| Agent | Intended work | Messenger selection | CLI model arguments |
| --- | --- | --- | --- |
| Codex | Small focused edits | `gpt-6-luna[effort=high]` | `-c 'model="gpt-6-luna"' -c 'model_reasoning_effort="high"'` |
| Codex | Everyday implementation/tests | `gpt-6.1-sol[effort=medium]` | `-c 'model="gpt-6.1-sol"' -c 'model_reasoning_effort="medium"'` |
| Codex | Complex features/debugging | `gpt-6-astra[effort=medium]` | `-c 'model="gpt-6-astra"' -c 'model_reasoning_effort="medium"'` |
| Codex | Hard architecture/investigation | `gpt-6-astra[effort=high]` | `-c 'model="gpt-6-astra"' -c 'model_reasoning_effort="high"'` |
| Claude | Mechanical edits | `claude-haiku-5-5[effort=low]` | `--model claude-haiku-5-5 --effort low` |
| Claude | Everyday implementation/tests | `claude-sonnet-5-5[effort=medium]` | `--model claude-sonnet-5-5 --effort medium` |
| Claude | Complex features/debugging | `claude-opus-5-5[effort=medium]` | `--model claude-opus-5-5 --effort medium` |
| Claude | Hard, long investigations | `claude-fable-5-2[effort=high]` | `--model claude-fable-5-2 --effort high` |
| Cursor | Quick focused coding | `composer-2.5` | `--model composer-2.5` |
| Cursor | Everyday implementation/tests | `cursor-grok-4.6-medium` | `--model cursor-grok-4.6-medium` |
| Cursor | Complex features/debugging | `grok-4.7-medium` | `--model grok-4.7-medium` |
| Cursor | Hard investigations | `grok-4.7-high` | `--model grok-4.7-high` |

The bracketed effort form is Messenger syntax, not a native Codex or Claude model ID. Only `low`, `medium`, `high`, `xhigh` and `max` are accepted; a model must support the requested value. Claude also receives the matching process-local `CLAUDE_CODE_EFFORT_LEVEL` so an inherited value does not override the selection. Organization caps and provider policy still apply. Plain legacy/custom model IDs retain their existing behavior. Upgrade the server, coordinator computer and executing computers to Messenger 0.72 before using bracketed effort selections. Older brokers may omit these model lists through the presence compatibility fallback.

Cursor uses its published IDs directly. The installed CLI's read-only `agent --list-models` confirmed the four IDs above; no inference was run. Composer uses its own default effort. Cursor Plus fixes effort to Medium and Fast off, so High needs a supporting plan. Claude third-party models still exist in Cursor on eligible plans, but are deliberately excluded from the new starter list. Fable likewise needs eligible Claude account access. No account entitlement check is implied by a configured list.

For Unity, a small serialized-field correction can use the first option; ordinary gameplay/editor code the second; a difficult import or state bug the third; a cross-system architecture investigation the fourth. These are workflow suggestions, not measured rankings. The coordinator should pick the least expensive adequate option and explain a non-default choice.

Existing exact starter lists from 0.71 are upgraded once. Custom lists and intentionally empty lists stay unchanged, as do recorded tasks and running sessions. Owners can edit the list in the agent settings.

## Sources

- [OpenAI models and effort guidance](https://learn.chatgpt.com/docs/models), [Codex configuration reference](https://learn.chatgpt.com/docs/config-file/config-reference): current model IDs, intended work and separate `model_reasoning_effort` configuration.
- [Claude supported model IDs](https://support.claude.com/en/articles/11940350-claude-code-model-configuration), [Claude Code model configuration](https://code.claude.com/docs/en/model-config): pinned versions, `--effort`, environment precedence and account/provider differences.
- [Cursor models and plan restrictions](https://cursor.com/docs/models-and-pricing), [CLI parameters](https://cursor.com/docs/cli/reference/parameters): first-party pool, effort restrictions, `--list-models` and `--model`. Exact Cursor IDs were checked against the installed CLI on 2026-11-14.
