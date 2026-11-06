# Launching a test session from the window on Mac

November 6, 2026. Check of preview.19, macOS arm64, Go 1.27.3. First, a local fake of the CLI/MCP protocol was used with separate HOME, process registry and database. Then a check with real Claude from the installed window was performed; its result is given below. Codex/Cursor did not receive model tasks.

- A new Claude session was created through the window, then its launch button was pressed separately.
- Two consecutive messages got bound replies `ACK:WINDOW-ONE` and `ACK:WINDOW-TWO`, both turns confirmed. The database has two attempts with one PID, one persistent process.
- An older message for another session stayed in the queue, with no launch attempt.
- Both replies appeared without a manual refresh. Reading state locally does not call the model.
- The first run revealed that process observation depended on the terminal's PATH: `sysctl` could not be found in the window's environment. Absolute system paths on Mac were fixed; a separate test checks observing its own process with an empty user PATH. The failed attempt remained in a separate test directory; its reservation was not deleted to bypass the protection. The rerun was done in a new isolated environment.
- The full `scripts/check.sh` passed on Mac: formatting, dependencies, vet and race tests. The JavaScript parses without syntax errors.
- Separately, the whole local suite passed on Go 1.22.2. All four GitHub jobs for `90ce8b109342f40409efd97bdb9af8cd050126fe` passed in [run 42196900929](https://github.com/kolotiloff/agent-messenger/actions/runs/42196900929): Mac/Linux, Go 1.22.2/stable.
- Automated tests cover eight concurrent repeats of the permission grant (one event), refusal without confirmation and for an old session, no widening of the permission, the pause being kept on a repeated ACK, and the permission reset after restart. HTTP tests check the Origin, the fields and the exact control command.

Limitations: Windows/Linux runtime, persistent Codex/Cursor, the network broker and releasing the protective reservation within the same OS boot are not accepted by this report. Automatic delivery of a new background client version is still separate from the shell update. Secrets, real session IDs and user logs are not included in the report.

## Publication and the background client

[Preview.19 published](https://github.com/kolotiloff/agent-messenger/releases/tag/v0.0.0-preview.19). The Mac arm64 archive was built with Go 1.27.3 from the stated SHA; SHA-256 `4584caecdbb280d861d2d06c812c19c08cde385781b3d632ab7f863339242afa`. The package and metadata signatures were created with the channel's standard key. All product sources were checked against the commit tree; the early Python experiments are not part of the Mac build.

On the user's Mac, a controlled replacement of the stopped background client with preview.19 was performed separately. Before stopping, the pause and the absence of model permission were confirmed. A local backup of the database and registry was made. After launch, all sessions, messages, attempts, replies and persistent process data were compared — no changes; the protective reservation registry is byte-for-byte the same. The client stays paused, model launches are not permitted, three old messages remain in the queue, six reservations are preserved.

At the first check the Mac was locked and the shell remained at preview.18. After unlocking, the update via the native updater completed: the installed shell shows preview.19, and its revision matches `90ce8b109342f40409efd97bdb9af8cd050126fe`.

## Real Claude from the installed window

In preview.19, via «Агенты» ("Agents"), Claude Code 2.1.318 was discovered and the existing login was checked. A separate new test session was created through the settings, then launch was explicitly permitted for this session only. Work and GUI conversations were not used.

- Two consecutive messages were sent from the native window's input field.
- Bound replies `ACK:WINDOW-ONE` and `ACK:WINDOW-TWO` were received; both attempts have state `turn_completed` and one PID/PGID. These are two turns of one persistent process, not two CLI launches.
- The replies appeared automatically, with confirmation of the completed turn. No manual refresh was needed.
- Three old messages stayed in the queue. Comparison with the pre-update snapshot confirmed that all earlier messages and replies were preserved.
- After the check, the queue was paused from the window. The test session's owner remains alive; no forced termination or removal of the protective reservation was performed. The permission is still limited to the new test session.

The database snapshot and the window screenshot are saved locally. They are not included in the public repository: they contain identifiers and old test addresses. This acceptance confirms two real messages through the window, but not the owner's release after a full exit, nor a Claude ↔ Codex exchange.
