# Windows and Linux (T-016)

Agent Messenger runs on Windows and Linux as one program, `messenger` (`messenger.exe` on Windows). There is no native window there yet. The window is the same page the Mac app shows, opened in the default browser (D-045).

## Download and start

Each release since preview.32 has these files next to the Mac app:

| File | For |
|---|---|
| `agent-messenger-VERSION-windows-amd64.zip` | Windows 10/11 on Intel or AMD |
| `agent-messenger-VERSION-windows-arm64.zip` | Windows 11 on ARM |
| `agent-messenger-VERSION-linux-amd64.tar.gz` | Linux on Intel or AMD |
| `agent-messenger-VERSION-linux-arm64.tar.gz` | Linux on ARM |
| `agent-messenger-VERSION-windows-linux.sha256` | SHA-256 of the four archives |

**Windows.** Unpack the zip anywhere and double-click `messenger.exe`. The program is not signed, so SmartScreen may warn: «Подробнее» → «Выполнить в любом случае». A console window opens and the browser shows Agent Messenger. The page works while the console is open; closing it closes only the page, and the background client keeps running. Data: `%LOCALAPPDATA%\AgentMessenger`.

**Linux.** `tar -xzf agent-messenger-VERSION-linux-amd64.tar.gz`, then `./messenger open`. Optionally `./messenger install` puts it in `~/.local/bin`. Data: `$XDG_DATA_HOME/agent-messenger` or `~/.local/share/agent-messenger`. If no browser opens (no `xdg-open`), the program prints the path of a file to open in a browser.

Each archive has a `README.txt` in Russian with the same steps.

## How the window opens

`messenger open` does what the Mac app does at start: it starts the background client if none answers, resumes it (D-029) and serves the page on `127.0.0.1` with a fresh key. The key is part of the page address. To keep it out of other programs' command lines, the browser is given a new private file in the data folder (`window-*.html`, mode 0600) that forwards to the address; the file is removed when the page closes. The page server accepts a cross-site *navigation* to its start page only (the browser treats the forwarding file as another site); the start page holds no data, and every API call still needs the same origin and the key.

On Windows, starting `messenger.exe` without arguments (a double click) runs `messenger open` and keeps an error on screen until Enter is pressed.

## Agents

- **Claude Code**: the installation that provides `claude.exe` on Windows (usually `%USERPROFILE%\.local\bin`), or `claude` in `PATH`, `~/.local/bin`, `/usr/local/bin` or `/usr/bin` on Linux.
- **Codex**: on Windows the npm command is a `.cmd` script. Messenger does not start `.cmd` or `.bat` files (D-044); it finds the `codex.exe` inside the npm package (`%APPDATA%\npm\node_modules\@openai\codex…\vendor\…\codex\codex.exe`). Any other `codex.exe` can be chosen by path.
- **Cursor** is not supported yet (T-012, Q-004).

## Process control on Windows (D-043)

Task runs follow D-021 on every system: a run is finished only after an observed exit, an empty process group after stopping it, and a revoked receipt. Windows has no process groups that a child cannot leave, so a run's group is a **named job object**:

- the worker starts suspended, is put in a new job named `Local\AgentMessenger-run-<pid>`, and only then runs; if anything fails before it runs, it is ended and the run counts as not started;
- every process it starts belongs to the job and cannot break away (the job does not allow it);
- the group is empty only when the client sees the job with no active process. Windows removes a job's name as soon as the last handle to it closes, even while its processes run, so the client keeps each job's handle open for the whole run. A job it cannot reach (for example after the client restarted while a run was going) is never taken as empty: that run stays *not proven*, and the task is interrupted or blocked, not done;
- "signalling the group" ends every process of the job (`TerminateJobObject`). Windows has no polite stop for a program without a console window, so a cancel is a hard stop;
- a job name that still exists from an earlier run is never reused; that start is refused and the task does not run;
- the start token of a process is its creation time; the boot identity is the registry `BootId` counter (or the time of the last orderly shutdown), never the clock.

A worker that leaves a child behind is therefore *blocked* on macOS and Linux (the child may have left the group) and *done* on Windows (the job's stop ended the child and the job is empty).

The single-instance lock uses `LockFileEx`; the background client starts with private pipes for its readiness handshake; `active-runs.json` is replaced with a write-through rename because Windows cannot flush a folder; while a reader has the file open Windows refuses that rename, so a refused rename or read is tried again for up to 2 seconds (a refused rename leaves the file unchanged; after that the error stands and the reservation is kept). The data folder is in `LocalAppData`, not the roaming profile, so the database is never synced to another computer.

## What is verified

- **Linux:** the whole test suite (CI on Ubuntu 24.04 with Go 1.22.2 and the newest Go, and here); `messenger open` with headless Chromium through the forwarding file; install and start from the Linux archive (CI and here).
- **Windows (CI runner, Windows Server 2025, `scripts/check-windows.sh`, required):** `go vet`; all tests of `platform`, `control`, `core`, `broker`, `localui`, `mcp`, `adapters`, `diagnostics` (tests whose fixtures use POSIX paths skip on Windows); in `supervisor` the task-run tests with the fake worker (report, question and answer, cancel, restart, failures, a leftover child ended by the job, tasks between computers through a real broker); in `cmd/messenger` the background client start and shutdown and the window file. Job objects: a live job is not empty, a child keeps the job alive after the first process exits, the job's stop ends it, a reused job name is refused, an unreachable job is not proven. The client also starts, answers `status` and shuts down from the Windows zip.
- **Windows, information only:** the whole suite also runs there and about 30 tests fail. They use POSIX fixtures (`/bin/sleep`, shell scripts as fake CLIs, roots like `/tmp/x` or `/old-data`) or the persistent Claude stream path, which the window does not use for tasks. With a POSIX root, `active-runs.json` reads as damaged and the client refuses to start runs, which is the safe direction.
- **Not verified:** a real Windows 10/11 desktop, SmartScreen, a browser opening on Windows, real Claude Code or Codex on Windows or Linux, the persistent Claude stream on Windows, ARM builds (compiled only), Windows and Linux joining a network with a Mac (the network code is the same as on the Mac).

## Not there yet

- Updates do not arrive by themselves on Windows and Linux: download the new archive and replace the program while the client is stopped (`messenger shutdown`).
- No installer, Start menu entry or autostart on Windows; no desktop entry on Linux.
- The programs are not code-signed.
