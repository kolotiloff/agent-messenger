#!/usr/bin/env bash
# Windows runtime checks (no real models). Runs on a Windows runner under
# Git Bash. The packages and tests listed here must pass on Windows; the rest
# of the suite still relies on POSIX shell fake CLIs and is run separately
# for information only (see .github/workflows/checks.yml).
set -euo pipefail

go_bin="${GO_BIN:-go}"
"$go_bin" version
"$go_bin" vet ./...
"$go_bin" test -count=1 -timeout=10m \
  ./internal/platform ./internal/control ./internal/core ./internal/broker \
  ./internal/localui ./internal/mcp ./internal/adapters ./internal/diagnostics
"$go_bin" test -count=1 -timeout=10m ./internal/supervisor \
  -run 'TestTask|TestCancelStopsARunningTask|TestRestartInterruptsARunningTask|TestCoordinatorStartsATask|TestTasksNeed'
"$go_bin" test -count=1 -timeout=5m ./cmd/messenger \
  -run 'TestWindows|TestWindowFile|TestWindowOpener|TestOpen'
