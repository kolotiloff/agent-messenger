# Codex app-server: configuration check without a model

A research probe for an installed Codex CLI. The product client remains in Go; this scenario is not wired into `serve` and is not a persistent adapter.

```sh
python3 experiments/codex_app_server/config_probe.py --cli /absolute/path/to/codex
```

Requires Python 3 and an installed Codex CLI. No account login is needed. Verified on macOS arm64 with Codex 0.155.1. On Windows this scenario has not been accepted.

Each case starts its own stdio app-server with empty temporary HOME/CODEX_HOME, an artificial profile and a separate working folder. Existing keys, settings and conversations are not read or copied. Only `initialize`, `initialized`, `config/read` are called; other methods are forbidden by the scenario itself. There is no `thread/start`, `turn/start` or connection to the GUI daemon. Temporary data is deleted; stdout contains only the check results.

Three checks:

1. `-c 'mcp_servers={}'` leaves a server from the file enabled. An empty table does not reset inherited MCP servers.
2. An explicit `mcp_servers.fixture.enabled=false` disables that specific server.
3. A clean profile contains no server from previous probes.

In all cases, the actual disabling of eight requested features is checked: shell, apps, plugins, hooks, multi-agent, browser, computer use and goals. A missing field counts as a failure, not as a disabled feature. An unexpected server, a protocol error, changed behavior or a timeout produce a non-zero exit. If there is an unexpected server, the probe does not proceed to creating a conversation.

**This is not proof of full Codex isolation.** The remaining features and system/project layers, MCP startup when a conversation is created, configuration changes after the check, and resuming a saved session require separate checks. The conclusion "a specific server is disabled" must not be extended to the user's whole profile. Until then, the product does not start persistent Codex. See the [adapter plan](../../docs/CODEX_ADAPTER_NEXT.md).
