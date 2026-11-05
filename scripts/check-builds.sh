#!/usr/bin/env bash
set -euo pipefail

go_bin="${GO_BIN:-go}"
build_tmp="$(mktemp -d /tmp/b.XXXXXX)"
trap 'rm -rf "$build_tmp"' EXIT
for target in darwin/arm64 darwin/amd64 linux/amd64 windows/amd64; do
  target_os="${target%/*}"
  target_arch="${target#*/}"
  printf 'Compile only: %s (binaries and tests; no execution)\n' "$target"
  CGO_ENABLED=0 GOOS="$target_os" GOARCH="$target_arch" "$go_bin" build -o "$build_tmp/messenger" ./cmd/messenger
  CGO_ENABLED=0 GOOS="$target_os" GOARCH="$target_arch" "$go_bin" list ./... > "$build_tmp/packages"
  index=0
  while IFS= read -r package; do
    index=$((index + 1))
    CGO_ENABLED=0 GOOS="$target_os" GOARCH="$target_arch" "$go_bin" test -c -o "$build_tmp/test-$index" "$package"
  done < "$build_tmp/packages"
done
