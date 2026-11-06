# First real Codex run from the window (T-004, part 1)

2026-11-06, the owner's Mac (Apple Silicon), Agent Messenger preview.21. Both preview.20 and preview.21 were built, signed and published by the Release Mac preview workflow.

## Steps

1. Updated the app to preview.21 through the window and stopped the old background client with `shutdown`. The window then started a new client.
2. Created a separate Codex profile, `~/AgentMessengerTest/codex-profile`, and logged in with `CODEX_HOME=… codex login -c 'cli_auth_credentials_store="file"'`. The default `~/.codex` was not used.
3. In the window: Agents → Codex → «Настроить» → «Создать сессию Codex и разрешить запуск». This created the session `codex-test-3a73811a`.
4. Sent two messages from the window.

## Result

- **Messages waited at first.** Both messages stayed «В очереди» ("In queue"), because the window's client starts paused and the Codex setup did not resume the queue. The Claude test-session launch does resume it. The owner pressed «Продолжить» ("Continue") in the queue panel. The fix is in the same PR as this report: setting up Codex from the window now resumes the queue, and model calls stay limited to the new alias.
- **Both turns completed** in the same live app-server, each shown as «Ответ подтверждён · ход завершён» ("Reply confirmed · turn completed"):
  - "Какая ты модель?" ("Which model are you?") was answered with "Я Codex, ИИ-ассистент на базе GPT-6." ("I'm Codex, an AI assistant based on GPT-6.");
  - "Ответь одним словом: ПРИВЕТ" ("Answer in one word: HELLO") was answered with "ПРИВЕТ".
- **Scope held.** The window showed «Запуск разрешён только для «codex-test-3a73811a»» ("Launch allowed only for codex-test-3a73811a").

**Verified on real Codex for the first time:**

- the managed profile login;
- creating a thread with dynamic tools;
- the config re-check before each turn;
- `turn/start`;
- the `read_current` → `reply` tool calls;
- turn completion and the bound replies.

## Not verified

- Exact resume of this thread after the client or app-server restarts. Resuming a Codex thread with a new owner is not implemented (T-009).
- Codex ↔ Claude exchange with real models.
- Behaviour when Codex asks for an approval or fails mid-turn with a real model.
