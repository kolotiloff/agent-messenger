# Agent Messenger — instructions for coding agents

This file is shared by all coding agents working on this repo (Claude Code reads it via `CLAUDE.md`, Cursor and Codex read `AGENTS.md`). Keep the rules in this one file.

Agent Messenger lets coding agents (Claude Code, Codex, Cursor Agent CLI) exchange messages on macOS, Windows and Linux. It is a technical preview for one Mac, not a release. Read [docs/HANDOFF.md](docs/HANDOFF.md) for the current state and next task, and [docs/RELEASE_PLAN.md](docs/RELEASE_PLAN.md) for the overall plan.

## Layout

- `cmd/messenger`, `internal/` — the product, in Go (module `github.com/kolotiloff/agent-messenger`, Go >= 1.22.2, pure-Go SQLite via modernc).
  - `internal/core` — SQLite store, queue, outbox, stream recovery.
  - `internal/supervisor` — session ownership, reservations, launching CLIs, persistent Claude stream.
  - `internal/adapters` — provider adapters; `codex_appserver.go` / `codex_turn.go` are the in-progress Codex work (not wired into `serve` or the window yet).
  - `internal/localui` — local HTTP UI; assets in `internal/localui/assets` are embedded into the binary.
  - `internal/platform` — OS-specific process, lock and boot-identity code.
- `desktop/macos` — native Swift/AppKit shell with Sparkle updates.
- `design/v1` — frozen, user-approved design with checksums. Do not edit it.
- Early research stands (Python and Go) were removed from the tree; they stay in git history and are linked from [docs/README.md](docs/README.md).
- `docs/` — design, contracts and plans; `docs/reports/` — dated acceptance reports.

## Checks

```bash
bash scripts/check.sh
```

Runs gofmt, `go mod verify`, `go vet` and race tests (macOS/Linux). `scripts/check-builds.sh` cross-compiles all targets. CI (`.github/workflows/checks.yml`) runs on Linux and macOS without real models. Run `scripts/check.sh` before every push.

Cloud sessions may run on Linux. Mac-only code (Swift shell, Sparkle, `*_darwin.go`) is verified by CI and by the user's Mac acceptance runs; say so explicitly instead of claiming it was tested here.

## Rules that must not be relaxed

- Never delete, empty or rewrite `active-runs.json`. Never release a reservation on a timeout or a single PID. Persistent owners (Claude stream, Codex app-server) keep the old rule: an empty process group does not release them either. Task runs follow D-021: a run is finished only after an observed exit, an empty process group (after signalling it) and a revoked receipt; any unclear evidence keeps that task blocked.
- On an uncertain result, never retry automatically. Unknown is not success.
- Do not bypass the shared registry to launch from another data directory.
- Never commit session IDs, tokens, auth files, SQLite dumps or full logs. The repository is public.
- Real model calls happen only in explicit acceptance runs with a separate new test session, never in normal tests. Never use the user's working or GUI chats as test sessions.
- Do not run Cursor CLI (the user asked to keep it off). Do not reboot the Mac.
- Never put an acceptance run's data directory in a temporary folder (`/tmp`, `/private/tmp`, `/var/folders`). macOS empties them on reboot, which strands the reservation and loses the history. Use a directory under `~/AgentMessengerTest/`.
- Do not present a preparatory component as a finished window feature. When a stage is done, update code, docs and verification results together; record what was actually verified and what remains.

## Conventions

- Code, docs, commit messages and PRs are in English. The app UI and CLI messages are still in Russian (the current user's language); changing that is a separate product decision.
- Talk to the user in Russian, in plain language.
- Work on a feature branch from current `main`; open a PR with the exact head SHA and a list of what was verified.
- Window styles follow [docs/UI_STYLE.md](docs/UI_STYLE.md): tokens only, no raw spacing or radii.
- Bundle UI tweaks into the next functional release instead of shipping a build per tweak. Keep the chosen light/dark themes and layout.

## Working documents and switching between tools

The user may stop one agent (Claude Code, Cursor, Codex) and continue in another. Nothing is shared between them except the repository. The working documents are listed in [docs/README.md](docs/README.md).

**At the start of a session:**
1. `git fetch` and start from current `main`, or from the branch named in [docs/HANDOFF.md](docs/HANDOFF.md).
2. Read `docs/HANDOFF.md` ("Current state"), then [docs/TASKS.md](docs/TASKS.md).
3. Check [docs/DECISIONS.md](docs/DECISIONS.md) before changing direction, and [docs/QUESTIONS.md](docs/QUESTIONS.md) for answers.

**While working:**
- Take tasks from `TASKS.md` and put the task ID in branch names, commits and PR titles.
- Record a new decision in `DECISIONS.md`.
- Put a question for the user in `QUESTIONS.md` instead of guessing.

**Before stopping, even if the work is unfinished:**
1. Update "Current state" in `HANDOFF.md`.
2. Update the task status in `TASKS.md`.
3. Add an entry to [docs/LOG.md](docs/LOG.md).
4. Commit and push. Mark unfinished work as WIP.

Do not assume local state from an earlier session.
