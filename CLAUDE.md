# Agent Messenger — notes for Claude Code

Agent Messenger lets coding agents (Claude Code, Codex, Cursor Agent CLI) exchange messages on macOS, Windows and Linux. It is a technical preview for one Mac, not a release. Read [docs/HANDOFF_CLAUDE.md](docs/HANDOFF_CLAUDE.md) for the current state and next task, and [docs/RELEASE_PLAN.md](docs/RELEASE_PLAN.md) for the overall plan.

## Layout

- `cmd/messenger`, `internal/` — the product, in Go (module `github.com/kolotiloff/agent-messenger`, Go >= 1.22.2, pure-Go SQLite via modernc).
  - `internal/core` — SQLite store, queue, outbox, stream recovery.
  - `internal/supervisor` — session ownership, reservations, launching CLIs, persistent Claude stream.
  - `internal/adapters` — provider adapters; `codex_appserver.go` / `codex_turn.go` are the in-progress Codex work (not wired into `serve` or the window yet).
  - `internal/localui` — local HTTP UI; assets in `internal/localui/assets` are embedded into the binary.
  - `internal/platform` — OS-specific process, lock and boot-identity code.
- `desktop/macos` — native Swift/AppKit shell with Sparkle updates.
- `design/v1` — frozen, user-approved design with checksums. Do not edit it.
- `experiments/` — early research stands (Python and Go). Not the product.
- `docs/` — design, contracts and plans; `docs/reports/` — dated acceptance reports.

## Checks

```bash
bash scripts/check.sh
```

Runs gofmt, `go mod verify`, `go vet` and race tests (macOS/Linux). `scripts/check-builds.sh` cross-compiles all targets. CI (`.github/workflows/checks.yml`) runs on Linux and macOS without real models. Run `scripts/check.sh` before every push.

This cloud session runs on Linux. Mac-only code (Swift shell, Sparkle, `*_darwin.go`) is verified by CI and by the user's Mac acceptance runs; say so explicitly instead of claiming it was tested here.

## Rules that must not be relaxed

- Never delete, empty or rewrite `active-runs.json`, and never release a reservation on a timeout, a single PID or an empty process group. A signal does not prove all descendants exited.
- On an uncertain result, never retry automatically. Unknown is not success.
- Do not bypass the shared registry to launch from another data directory.
- Never commit session IDs, tokens, auth files, SQLite dumps or full logs. The repository is public.
- Real model calls happen only in explicit acceptance runs with a separate new test session, never in normal tests. Never use the user's working or GUI chats as test sessions.
- Do not run Cursor CLI (the user asked to keep it off). Do not reboot the Mac.
- Do not present a preparatory component as a finished window feature. When a stage is done, update code, docs and verification results together; record what was actually verified and what remains.

## Conventions

- Code, docs, commit messages and PRs are in English. The app UI and CLI messages are still in Russian (the current user's language); changing that is a separate product decision.
- Talk to the user in Russian, in plain language.
- Work on a feature branch from current `main`; open a PR with the exact head SHA and a list of what was verified.
- Bundle UI tweaks into the next functional release instead of shipping a build per tweak. Keep the chosen light/dark themes and layout.
