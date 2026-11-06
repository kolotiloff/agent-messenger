# Diagnostics: check on Mac

Change on top of `5a72aa3d851d07f3c9b5e24b096c39f6c1f1faa6` (PR #5). Added `doctor`, `doctor --json`, a context-bounded local status request and an up-to-date [plan to release](../RELEASE_PLAN.md).

## Scope

The command looks for CLI candidates in PATH, checks the data directory metadata and requests the status of the existing client. It does not execute the programs found, does not read profiles/secrets, does not create a database and does not change reservations. Version, CLI identity, authorization and readiness of specific sessions remain `not_checked`.

The control request is limited to 750 ms. Unavailability, a timeout and an unsuccessful status are not declared as readiness and do not permit a new session launch. The JSON contains no conversations, conversation IDs, raw server errors or `status.detail`. The `probe_cli_starts` field refers only to diagnostics.

## Checks

On macOS arm64, Go 1.22.2, the full suite passed:

```sh
go test -race -tags streamprobe -ldflags=-linkmode=external -count=1 ./...
```

Five new tests cover: finding fake CLIs without executing them or creating a directory; no false login conclusion when the program is absent; requesting only `status` and the absence of private fields; a bounded wait on an unresponsive socket with the reservation kept; refusing to treat a status error as client availability.

A native probe on Mac verified:

- With Messenger stopped, the report was produced; the hashes of the database, WAL/SHM if present, and the shared registry did not change.
- In a non-existent directory the report showed `missing`, and the directory was not created.
- Candidates for Claude, Codex and Cursor were found in PATH. This is not a check of versions or login; all three cards kept `authentication=not_checked`.
- After `start --claude-stream` without model permission, diagnostics saw the running client, the pause and `cli_launch=not-authorized`.
- The diagnostic request did not change sessions, messages, attempts, replies, checks or archives. The registry remained unchanged.
- The `--allow-model-calls` flag and an extra positional argument for `doctor` were rejected.
- The client was stopped; no CLIs or models were launched by this check.

Builds for linux/amd64 and windows/amd64 with `CGO_ENABLED=0` passed. Runtime on these OSes was not verified. The old Mac linker's LC_DYSYMTAB warnings did not interfere with tests/build.

## What this does not yet confirm

There is not yet a standard authorization check, version compatibility detection, manual selection of a CLI outside PATH, individual binding statuses, GUI or installer. Exit code 0 means a report was produced, not permission to launch an agent. Fully stopping the CLI and lifting the reservation within the same boot remain a separate unresolved task.

Raw local paths, PIDs and diagnostic snapshots are saved privately and not included in Git. The repository contains only code, synthetic tests and anonymized results.
