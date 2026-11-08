# Download and start

The buttons in the [README](../README.md) always download the newest release. Every release has the same file names:

| File | For |
|---|---|
| `AgentMessenger-Mac.zip` | Mac with Apple Silicon, macOS 15+ |
| `AgentMessenger-Windows-x64.exe` | Windows 10/11 on Intel or AMD |
| `AgentMessenger-Windows-ARM64.exe` | Windows 11 on ARM |
| `AgentMessenger-Linux-x64` | Linux on Intel or AMD |
| `AgentMessenger-Linux-ARM64` | Linux on ARM |
| `AgentMessenger-Windows-Linux-SHA256.txt` | SHA-256 of the four Windows and Linux files |
| `update-<os>-<arch>.json`, `.sig`, `<sha256>.zip` | Signed updates the programs download by themselves; not for manual download |

Only the Mac app comes in a zip: a Mac app is a folder, and a browser cannot download a folder. Safari unpacks the zip by itself. The same zip is also published under its checksum as a name; the app's own updater uses that one.

## Mac

1. Download `AgentMessenger-Mac.zip`. Safari unpacks it into `Agent Messenger.app`; otherwise double-click the zip.
2. Move `Agent Messenger.app` to `~/Applications` and open it. The app is not notarized yet, so the first time macOS may ask to confirm the opening in System Settings → Privacy & Security.
3. Later versions arrive in the app («Доступно обновление»). More: [MAC_APP.md](MAC_APP.md).

## Windows

1. Download `AgentMessenger-Windows-x64.exe` (or `-ARM64.exe` on ARM) and put it in any folder, for example `Documents\Agent Messenger`.
2. Double-click it. The program is not signed, so SmartScreen may warn: «Подробнее» → «Выполнить в любом случае».
3. Agent Messenger opens in a window of its own (Windows' built-in WebView2 engine, D-054); the console window that Windows opens for a double click closes. Closing the window closes only the window; the background client and its tasks keep running. To come back, start the program again. On a computer without WebView2 the page opens in the browser instead.
4. To stop the background client: in a console in that folder, `AgentMessenger-Windows-x64.exe shutdown`.

Data: `%LOCALAPPDATA%\AgentMessenger`. Later versions arrive in the window («Доступно обновление» → «Обновиться»); the program replaces its own file and opens the window again ([updates](UPDATES.md#windows-and-linux)). Keep the program in a folder you can write to, not in `Program Files`.

## Linux

```bash
chmod +x AgentMessenger-Linux-x64
```

```bash
./AgentMessenger-Linux-x64 open
```

A browser download does not keep the "may run" mark of a file, hence `chmod +x` once. `./AgentMessenger-Linux-x64 install` optionally puts the program in `~/.local/bin` as `messenger`. The window opens on its own, without tabs or an address bar, through Chrome, Chromium, Edge or Brave when one is installed (D-054); after the first start Agent Messenger is in the application menu. Without them the page opens in the default browser, or the program prints the path of a file to open.

Data: `$XDG_DATA_HOME/agent-messenger` or `~/.local/share/agent-messenger`. To stop the background client: `./AgentMessenger-Linux-x64 shutdown`. Later versions arrive in the window («Доступно обновление» → «Обновиться»), as on Windows; a program installed with `install` is updated there as well ([updates](UPDATES.md#windows-and-linux)).

## Agents

- **Claude Code**: the usual installation (`claude.exe` on Windows, usually in `%USERPROFILE%\.local\bin`; `claude` on Linux and Mac).
- **Codex**: on Windows the npm installation works; the program finds the `codex.exe` inside the npm package. `.cmd` and `.bat` files are not started (D-044).
- **Cursor**: `cursor-agent` on Mac and Linux (usually in `~/.local/bin`). Before the first Cursor task Messenger adds one entry, `messenger_task`, to `~/.cursor/mcp.json`; other entries stay as they are (D-049). Not on Windows yet.

## First steps

1. **Агенты:** connect Claude Code (and Codex or Cursor if you use them).
2. **Новый проект…:** a name and the project folder. Messenger then adds the project to Claude's settings by itself; no Terminal is needed (D-050). The window says «Claude подключён к проекту».
3. Open the project folder in Claude Code (the Claude app, Code tab, this computer; or `claude` in that folder) and start a **new** chat. Ask «Каких агентов ты видишь?», then give it work.

A web chat on claude.ai or the Chat tab does not see Messenger: the coordinator runs only in Claude Code on the computer where Messenger runs.

## Check a download (optional)

```bash
sha256sum -c AgentMessenger-Windows-Linux-SHA256.txt --ignore-missing
```
