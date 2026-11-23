#!/usr/bin/env bash
set -euo pipefail

go_bin="${GO_BIN:-go}"
goroot="$("$go_bin" env GOROOT)"
ci_tmp="$(mktemp -d /tmp/a.XXXXXX)"
trap 'rm -rf "$ci_tmp"' EXIT
export TMPDIR="$ci_tmp"

"$go_bin" version
unformatted="$("$goroot/bin/gofmt" -l cmd internal)"
if [[ -n "$unformatted" ]]; then
  printf 'Go files need formatting:\n%s\n' "$unformatted" >&2
  exit 1
fi
sh -n scripts/install-server.sh
"$go_bin" mod verify
"$go_bin" vet ./...
# The timeout catches a hung test. With -race on a shared CI machine the core
# package already takes 520-600 seconds and supervisor about 480, so the
# budget is fifteen minutes per package.
case "$("$go_bin" env GOOS)" in
  darwin) CGO_ENABLED=1 "$go_bin" test -race -count=1 -timeout=15m -ldflags=-linkmode=external ./... ;;
  linux) CGO_ENABLED=1 "$go_bin" test -race -count=1 -timeout=15m ./... ;;
  *) printf 'Native race checks are currently accepted only on macOS/Linux.\n' >&2; exit 1 ;;
esac
