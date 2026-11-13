# Windows and Linux (T-016)

Agent Messenger runs on Windows and Linux as one program, `messenger` (`messenger.exe` on Windows). The window is the same page the Mac app shows. On Windows it is the program's own window through WebView2; on Linux the program's own GTK window through the system's WebKitGTK (D-078). Without those, a Chromium-based browser in app mode (D-054), then the default browser (D-045).

## Download and start

Every release since preview.33 has the Windows and Linux programs as plain files with fixed names, next to the Mac app (D-047): `AgentMessenger-Windows-x64.exe`, `AgentMessenger-Windows-ARM64.exe`, `AgentMessenger-Linux-x64`, `AgentMessenger-Linux-ARM64`, and their SHA-256 list. The download buttons in the README link to the newest release. How to start them: [DOWNLOAD.md](DOWNLOAD.md). (preview.32 had the same programs inside zip and tar.gz archives.)

## How the window opens

`messenger open` does what the Mac app does at start: it starts the background client if none answers, resumes it (D-029) and serves the page on `127.0.0.1` with a fresh key. The key is part of the page address. The program's own window (WebView2, WebKitGTK) loads it directly. To keep it out of other programs' command lines, a browser is given a new private file in the data folder (`window-*.html`, mode 0600) that forwards to the address; the file is removed when the page closes. The page server accepts a cross-site *navigation* to its start page only (the browser treats the forwarding file as another site); the start page holds no data, and every API call still needs the same origin and the key.

On Windows, starting the program without arguments (a double click) runs `messenger open` and keeps an error on screen until Enter is pressed.

## Agents

- **Claude Code**: the installation that provides `claude.exe` on Windows (usually `%USERPROFILE%\.local\bin`), or `claude` in `PATH`, `~/.local/bin`, `/usr/local/bin` or `/usr/bin` on Linux.
- **Codex**: on Windows the npm command is a `.cmd` script. Messenger does not start `.cmd` or `.bat` files (D-044); it finds the `codex.exe` inside the npm package (`%APPDATA%\npm\node_modules\@openai\codex…\vendor\<target>\codex\codex.exe`, or `…\bin\codex.exe` in newer packages). Any other `codex.exe` can be chosen by path.
- **Cursor**: on Linux `cursor-agent` in `PATH` or `~/.local/bin` (D-049). On Windows the command is a `.cmd` script (D-044), so Messenger finds `%LOCALAPPDATA%\cursor-agent\versions\<newest>\node.exe` and starts it with its `index.js` (D-052).

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

- **Linux:** the whole test suite (CI on Ubuntu 24.04 with Go 1.22.2 and the newest Go, and here); `messenger open` with headless Chromium through the forwarding file; install and start from the Linux download (CI) and from the preview.32 archive (here).
- **Windows (CI runner, Windows Server 2025, `scripts/check-windows.sh`, required):** `go vet`; all tests of `platform`, `control`, `core`, `broker`, `localui`, `mcp`, `adapters`, `diagnostics` (tests whose fixtures use POSIX paths skip on Windows); in `supervisor` the task-run tests with the fake worker (report, question and answer, cancel, restart, failures, a leftover child ended by the job, tasks between computers through a real broker); in `cmd/messenger` the background client start and shutdown and the window file. Job objects: a live job is not empty, a child keeps the job alive after the first process exits, the job's stop ends it, a reused job name is refused, an unreachable job is not proven. The client also starts, answers `status` and shuts down from the Windows zip.
- **Windows, information only:** the whole suite also runs there and about 30 tests fail. They use POSIX fixtures (`/bin/sleep`, shell scripts as fake CLIs, roots like `/tmp/x` or `/old-data`) or the persistent Claude stream path, which the window does not use for tasks. With a POSIX root, `active-runs.json` reads as damaged and the client refuses to start runs, which is the safe direction.
- **Updates:** on Linux here, preview.900 → preview.901 built with a test key: the window showed «Доступно обновление», «Обновиться» replaced the program and the window opened again from preview.901, which then offered to restart the older background client. Tests of the update package refuse a foreign signature, a changed archive, an unexpected file, a wrong version or revision, and an older build. On the Windows CI runner: replacing a program that is running, and a second update while the first old program still runs. The release signs the four update archives with the real key on the macOS job; CI checks the signing script with a throwaway key.
- **In daily use (owner's computers, by hand):** Windows 10/11 desktops and a Linux laptop in one network with a Mac, real Claude Code, Codex and Cursor workers, updates from published releases, including updates started from another computer (T-078, T-136).
- **Not verified:** the persistent Claude stream on Windows (the window does not use it for tasks), ARM builds (compiled only).

## Updates (D-048)

From preview.33 the window updates the program from the same signed channel as the Mac app: every 30 minutes it looks for `update-<os>-<arch>.json` in the newest releases, checks its Ed25519 signature and downloads the signed archive. «Обновиться» checks the archive's checksum and signature again, unpacks the one `messenger` / `messenger.exe` inside next to the program, runs its `version` and requires the exact signed version, revision, system and architecture, and only then replaces the program file. On Windows the running file is renamed to `<name>.old` (a running program cannot be overwritten) and removed at a later start; on Linux the new file is renamed over the old one. A program installed with `messenger install` gets the new version as another installation instead. The window then closes and opens again from the new program. The window then restarts an older background client by itself; when tasks run it asks first (D-091, T-123), as on the Mac. Any failed check leaves the old program as it was. Details: [UPDATES.md](UPDATES.md#windows-and-linux).

On Windows the background client always runs from `runtimes\messenger.exe` in the data folder, whatever the version (D-118). Windows Firewall remembers the permission by that path, so it asks «Разрешить доступ» once, not after every update; this matters for a computer that hosts the server and is updated from another one.

A Windows or Linux computer can also be updated from another computer of the network: an update icon on its card in the left panel appears when its version is older ([UPDATES.md](UPDATES.md#from-another-computer-of-the-network-t-078-d-100)).

## Not there yet

- No autostart. On Windows the first start installs the program for the current user with a Start menu entry and an «Приложения» entry that removes it (D-082, [DOWNLOAD.md](DOWNLOAD.md#windows)). On Linux the first `messenger open` adds Agent Messenger with its icon to the application menu (`~/.local/share/applications`); `messenger install` does too. For an installation made by `messenger install` the entry starts its `bin/messenger` link, which every update moves to the new version, and the background client fixes an existing entry when it starts, so an update from another computer or by hand no longer leaves the menu starting the old version (T-138). There is no package (deb, snap, flatpak).
- The programs are not code-signed.
