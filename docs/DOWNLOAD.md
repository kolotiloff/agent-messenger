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

Only the Mac app comes in a zip: a Mac app is a folder, and a browser cannot download a folder. Safari unpacks the zip by itself. The same zip is also published under its checksum as a name; the app's own updater uses that one.

## Mac

1. Download `AgentMessenger-Mac.zip`. Safari unpacks it into `Agent Messenger.app`; otherwise double-click the zip.
2. Move `Agent Messenger.app` to `~/Applications` and open it. The app is not notarized yet, so the first time macOS may ask to confirm the opening in System Settings → Privacy & Security.
3. Later versions arrive in the app («Доступно обновление»). More: [MAC_APP.md](MAC_APP.md).

## Windows

1. Download `AgentMessenger-Windows-x64.exe` (or `-ARM64.exe` on ARM) and put it in any folder, for example `Documents\Agent Messenger`.
2. Double-click it. The program is not signed, so SmartScreen may warn: «Подробнее» → «Выполнить в любом случае».
3. A console window opens and the browser shows Agent Messenger. The page works while the console is open; closing the console closes only the page, and the background client keeps running. To come back to the window, start the program again.
4. To stop the background client: in a console in that folder, `AgentMessenger-Windows-x64.exe shutdown`.

Data: `%LOCALAPPDATA%\AgentMessenger`. Updates do not arrive by themselves yet: stop the client, then replace the file with the new download.

## Linux

```bash
chmod +x AgentMessenger-Linux-x64
```

```bash
./AgentMessenger-Linux-x64 open
```

A browser download does not keep the "may run" mark of a file, hence `chmod +x` once. `./AgentMessenger-Linux-x64 install` optionally puts the program in `~/.local/bin` as `messenger`. If no browser opens (no `xdg-open`), the program prints the path of a file to open in a browser.

Data: `$XDG_DATA_HOME/agent-messenger` or `~/.local/share/agent-messenger`. To stop the background client: `./AgentMessenger-Linux-x64 shutdown`.

## Agents

- **Claude Code**: the usual installation (`claude.exe` on Windows, usually in `%USERPROFILE%\.local\bin`; `claude` on Linux and Mac).
- **Codex**: on Windows the npm installation works; the program finds the `codex.exe` inside the npm package. `.cmd` and `.bat` files are not started (D-044).
- **Cursor** is not supported yet.

## Check a download (optional)

```bash
sha256sum -c AgentMessenger-Windows-Linux-SHA256.txt --ignore-missing
```
