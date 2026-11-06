# Preview package and install: Mac, November 6, 2026

Change on top of `29ee58b` (PR #12). macOS arm64, Go 1.22.2. `go test -race -count=1 -ldflags=-linkmode=external ./...` passed. After adding the check for a foreign link/corrupted binary, the distribution package was rerun separately with race. Linux/amd64 and Windows/amd64 were compiled; their installer refuses on purpose: the runtime has not been accepted.

## Automated checks

- Install into a separate user prefix, repeat with the same build, switch to a different version. The old binary is kept; a control file of user data does not change.
- Install refused when the client or launch lock was held, with an empty/truncated/`null` registry, and with any saved reservation. No changes to the target link on refusal.
- A foreign regular file and a foreign symbolic link are not replaced. A checksum mismatch of the installed binary causes a refusal, not an overwrite.

## Verifying real archives

Two **test** packages, `0.0.0-preview.test` and `0.0.0-preview.test2`, were built with an explicitly placeholder zero revision. These are verification artifacts, not release versions. The builder ran Go with the external linker, produced native binaries, signed them locally ad-hoc, ran `version` and generated the archives/manifest/checksums.

In a separate HOME and prefix:

1. The SHA-256 of the archives and of the binaries inside were verified. The archives were unpacked without external links.
2. The `version` command printed the expected version without creating a data directory.
3. The first package was installed. Its binary started `serve` without model permission. A synthetic session with `/usr/bin/false` was registered in a separate database, the queue was paused and one message was saved.
4. Installing the second version with a live client refused; the link to the first version did not change.
5. After a standard shutdown, the new version was installed. The old binary remained, and the new `version` command showed the new version. Reinstalling the same version succeeded without replacing data.
6. The new binary opened the existing database. A full `inspect` before and after the update matched: the message, session, events and pause state were preserved. Model permission remained `not-authorized`.
7. The test client was stopped. Work chats, profiles, reservations and the real user installation were not changed. Real Claude/Codex/Cursor were not launched.

The verification script initially read the text-format status command as JSON by mistake; this was fixed, and the full install experiment was rerun and passed. This was a test-rig bug, not a change to the product format.

## What is not accepted

This is an install from a local archive on this Mac, not an install of a browser-downloaded file on a clean, unfamiliar Mac. Developer ID, notarization and the user-facing Gatekeeper path have not been verified. The builder does not publish a release and does not attest that an arbitrary tree matches the claimed SHA. The overall PR chain is still a draft. The installer does not release reservations to make updating easier; a safe daily CLI exit remains a separate task. SQLite rollback and arbitrary downgrades are not supported.
