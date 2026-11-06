# Nightly CI and Mac preview check — November 6, 2026

## Verified state

PR [#15](https://github.com/kolotiloff/agent-messenger/pull/15), head `b31596c999f17fffa71bf4b336fd63f97c7f5457`. All four jobs of [run 42125645635](https://github.com/kolotiloff/agent-messenger/actions/runs/42125645635) completed successfully. The pull request check uses the merge revision `dc73edc21112bb81804f069430d85fbec9791730`; the archive records it as the source. This is not a public release and not a merge of the draft PR chain.

| Environment | Confirmed |
|---|---|
| macOS 15 arm64 / Go 1.22.2 | Formatting, module verification, vet and the whole native suite with race |
| Ubuntu 24.04 / Go 1.22.2 | The same checks; compiling the product and test binaries for macOS arm64/amd64, Linux amd64, Windows amd64 |
| macOS 15 arm64 / Go 1.27.3 | Native suite with race; two archives; install and update |
| Ubuntu 24.04 / Go 1.27.3 | Native suite with race |

Stable Go resolved to 1.27.3 in this run. The old Go 1.22.2 is kept to check source compatibility; ready-made CI archives are now built with a supported version. [Go support policy](https://go.dev/doc/devel/release#policy).

## What the real check revealed

The first Linux stable-Go run found a cleanup race in the test `TestSupervisorRunsExplicitSessionAndBlocksOccupiedGroup`: the client had already exited, but the test CLI could still be writing to SQLite. Deleting the temporary folder sometimes failed with `directory not empty`. This is a consequence of intentional shutdown behavior: it does not kill the CLI. The test now waits for its helper process to exit before deleting the temporary database; product shutdown and reservations are not weakened.

The fix passed 20 consecutive repetitions on the user's Mac with Go 1.22.2 and race, then all GitHub jobs. The logs contain no real sessions, since stand-in processes are used.

## Install and update

Mac CI verified the SHA-256 of the archive and the binary, the version command without creating user data, a clean install, refusal to update a live client, updating a stopped client with a non-empty queue, reinstalling the same file, preservation of the old binary, and a full match of the queue snapshot before/after. All test processes were stopped. New model calls: 0.

On the user's Mac, a regular Go 1.27.3 build and running version were verified separately. On the old Go 1.22.2, a regular external build could produce a signature macOS does not accept; the preview builder explicitly signs its own binary ad-hoc and verifies that it launches. This is not Developer ID/notarization and not a Gatekeeper bypass.

## Additional check of outgoing messages during an update

A real update of two local archives was performed on the user's Mac: `0.0.0-preview.1` (sources `468648f`, Go 1.22.2) → `0.0.0-preview.2` (sources `ecc8ff6`, Go 1.27.3). Temporary HOME, profile and queue were used; the CLI was a stand-in `/usr/bin/false`, models were not permitted.

Besides a regular message, two envelopes were kept in the queue: `prepared` and `queued` without window confirmation. After the update, their addresses, text, ID, state and time were fully preserved. The unconfirmed `prepared` did not turn into a job; repeating `ui-submit` for an already accepted ID returned `created=false`. The number of jobs remained two, attempts — zero. Test processes were stopped. The scenario was added to `scripts/check-package.py` for future CI runs.

The local preview.2 archive was checked against GitHub: all 137 present product/documentation files matched by Git blob SHA; only 28 files of the historical Python test bench, which do not take part in the build, are missing. Archive SHA-256: `c7affe854204dc7518a39c7da0537acf2f7cdb5e6caa1a7aadc4b2a19c5b3ed5`. The internal checksum of the binary and the manifest were verified. This is the `ecc8ff6` archive, not any later documentation commit.

## Instructions and interface

The README was brought in line with the actual state: the local window and installation already exist, but this is a limited Mac preview. Historical limitations of the one-shot client are marked as history. In the second terminal, the same ROOT must be set explicitly; the first terminal's variable is not carried over automatically. Links in the current instructions were checked against the repository tree.

The design/v1 styling did not change. The product keeps the previously verified interface with manual refresh. Improvements to drafts, background reading and keyboard focus were prepared separately but not included: the automatic permission check rejected opening the test local window. A JavaScript syntax check is not considered visual acceptance.

## What is not accepted yet

Windows runtime, persistent product Codex/Cursor adapters, all four devices, the network broker, coordinator role handover, safe final release of a CLI owner within the same Mac boot, backup/restore, clean distribution through Gatekeeper and a signed release. Coding and building do not replace these checks.

Real Claude, Codex and Cursor were not run tonight. Working chats and profiles were not used, protective reservations were not deleted, the Mac was not rebooted. Cursor CLI was not run. A full release has not been announced.
