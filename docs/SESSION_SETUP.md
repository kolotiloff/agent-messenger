# Setup from the window

> **Superseded.** The buttons described here («Настроить», «Проверить версию и вход», «Создать тестовую сессию») are gone; agent cards are described in [CLI_CHECK.md](CLI_CHECK.md) and [TASK_MODEL.md](TASK_MODEL.md).

Agent card → «Настроить» ("Set up") shows the CLI program and the profile directory. Program discovery is passive. «Проверить версию и вход» ("Check version and login") runs only the diagnostic commands of the selected CLI; the limitations are described in [CLI_CHECK.md](CLI_CHECK.md).

For Claude, a «Создать тестовую сессию» ("Create test session") button has been added. It is available only if the background client reports support for the new operation and for Claude's persistent mode. An old client is not sent a substitute registration command: the window states plainly that it needs to be updated.

Creation records a new exact UUID, the session address, the selected program/profile, and a separate `workspaces/<uuid>` folder inside the data directory. It does not run a model, does not send a message and does not change the permission for CLI calls. Working chats and GUI chats are not used. Choosing an existing session, login/logout, and persistent adapters for the other providers remain separate stages.

The setup is saved in a single transaction. Invalid paths leave no partial record. Repeating the same request confirms the same record; different settings under the existing address, or a new binding to an already registered session, are rejected. If the response is lost, the window keeps the original request in memory for an explicit retry. After the window has been fully closed, check the session list first: the same address cannot create a second session. The technical UUID does not need to be entered or published in the interface.

A ready profile and a created record do not mean the model is already running. Until model launches are allowed, messages stay in the queue. In preview.19, a new test session is enabled with a separate button on its card in the session list. Safely terminating the owner remains a separate stage. Installing a new shell does not by itself replace a background process that is already running.

The permission rules and limitations are described in [launching only the selected session](SCOPED_LAUNCH.md). The preview.19 background client is required: an old client is not sent a nonexistent command. A regular application launch still does not allow model calls.

## Codex from the window

Agents → Codex → «Настроить» ("Set up") offers a new Codex test session.

1. **Prepare a profile.** It needs a separate profile, never `~/.codex`. The field suggests `~/AgentMessengerTest/codex-profile`. Log in to it once in Terminal:

   ```sh
   CODEX_HOME="PROFILE" codex login -c 'cli_auth_credentials_store="file"'
   ```

   «Проверить версию и вход» ("Check version and login") then shows the login state.
2. **Create the session.** «Создать сессию Codex и разрешить запуск» ("Create Codex session and allow launch") does the following in one step:
   - grants model calls for this new alias only, as the Claude test-session launch does;
   - runs `create-codex-session`: it starts the managed app-server, checks its config and creates the thread.

   A refusal shows the client's reason. If nothing was launched, the permission is withdrawn again. A second session cannot get permission in the same client run.
3. **Send messages.** The session then appears under Messages, and messages are delivered as described in [CODEX_ADAPTER_NEXT.md](CODEX_ADAPTER_NEXT.md).

