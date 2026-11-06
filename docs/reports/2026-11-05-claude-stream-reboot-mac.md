# Claude recovery after a real Mac reboot

Tested `a337acc20d9e4e8a6c29dfbc19092446ec035421` from PR #4; the product Go code matches `026131ce8392e377c3dc4ed059724c7c6330f3cf`. macOS arm64, Go 1.22.2, external linker and local ad-hoc signing. The preceding probe used Claude Code 2.1.318. This stage included a real reboot by the user, the standard recovery command without a model, and one new message to real Claude.

## Prepared state and preservation

Before the reboot, a separate test CLI session had already given a confirmed reply on exact resume: [previous report](2026-11-05-claude-stream-exact-resume-mac.md). After it was stopped, its channel remained `uncertain`, and the reservation contained pre-recorded boot and launch identifiers. Working and GUI chats were not used.

After the reboot, the same computer fingerprint and a different native `kern.bootsessionuuid` were confirmed. The identifier values were not published. The shared registry matched the saved checksum byte for byte; the supervisor/startup locks were free.

macOS cleared the temporary directory of the test database and the binary. Under both locks, the database was restored from a previously saved private copy to the exact original path, only because the directory was absent. The copy's hash and `PRAGMA integrity_check` were verified; no existing files were replaced. The registry was not restored or edited by hand. The binary was rebuilt from the verified source code. This is manual test preparation, not an implemented product backup feature.

## Recovery command without a model

`recover-stream` was run for the exact alias and ID of the previous stream launch with `--confirm preserve-uncertain`.

- Exit code 0; only the reservation of the selected launch was removed. Four earlier reservations remained unchanged.
- The old channel was archived as `uncertain`; records of attempts, messages, replies and verifications were preserved unchanged. The old linked reply and `provider_ok=1` were preserved.
- The owner of the selected session was released, the next launch was set to `resume`, the queue is paused, and recovery is in phase `done`.
- The command created no new messages or attempts; no models were started.
- Repeating the same command and restarting the client without model permission did not duplicate history, did not change the registry, and left the queue paused.

## One new message after recovery

One new control message was explicitly enqueued to the same separate session. While paused, no attempt was created. After a separate `resume`, one linked reply with `provider_ok=1` was received; the persistent channel went to `idle`.

The live process arguments confirmed `--resume` with the exact previous ID, without `--session-id`. The reply contained `ACK:REBOOT-one|` and the earlier private marker, which was not included in the new message. This verifies the preserved context after a real recovery, not just that resume mode was recorded in the database.

Exactly one new message, one attempt and one reply were created. All earlier messages, replies, attempts and verifications remained unchanged; there were no automatic retries. A new stream generation was created with a new reservation for the current boot.

After pausing and `shutdown`, the client was started without model permission to check the saved state, and stopped again. The new channel kept `uncertain`, `exit_observed=0`, `exit_code=null`; successful process exit is not claimed. The reservation and the owner were preserved.

Repeating the recovery command for the **old** generation did not affect the new reservation. Recovery of the **new** generation in the same boot was refused with `same boot`; the registry did not change. The four earlier reservations were also preserved. The final private copy of the closed database was verified and kept outside Git.

## Outcome and limitations

On one Mac, the following chain was accepted: saved test Claude → real reboot → explicit recovery → exact resume → one new linked message without retrying old ones.

The product code was not changed in this stage. The full test suite with the race detector was run earlier for this code; here a native integration check was performed, and a repeat of the full suite is not claimed. One user message may include several model and tool turns; this is not a claim about a single API call or a measurement of idle billing.

This is not a test of a crash interrupting an active reply: Messenger was stopped before the reboot. Recovery without a reboot, old reservations without metadata, Codex/Cursor in the persistent client, Windows/Linux in operation, the network, the installer and the interface remain outside this acceptance. The protective reservation after the new stop remains, as expected; everyday safe closing and reopening of a session without a reboot is not yet implemented.

Private session/launch IDs, the marker, profile paths, CLI logs, databases, backups and computer identifiers are not included in the repository.
