#!/usr/bin/env bash
# Builds the Windows and Linux programs of one version, ready to download as
# they are (D-047): messenger with the version and revision built in, under a
# fixed name so a link to the newest release always works, plus a SHA-256
# list. Pure Go, no signing. Usage: package-others.sh VERSION REVISION OUTDIR
set -euo pipefail

version="$1"
revision="$2"
out="$3"
go_bin="${GO_BIN:-go}"
[[ "$version" =~ ^[0-9A-Za-z.+-]+$ ]] || { echo "bad version" >&2; exit 1; }
mkdir -p "$out"
flags="-X github.com/kolotiloff/agent-messenger/internal/buildinfo.Version=$version -X github.com/kolotiloff/agent-messenger/internal/buildinfo.Revision=$revision"
names=()
for target in windows/amd64:AgentMessenger-Windows-x64.exe windows/arm64:AgentMessenger-Windows-ARM64.exe \
  linux/amd64:AgentMessenger-Linux-x64 linux/arm64:AgentMessenger-Linux-ARM64; do
  platform="${target%%:*}"
  name="${target#*:}"
  os="${platform%/*}"
  arch="${platform#*/}"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" "$go_bin" build -trimpath -buildvcs=false -ldflags "$flags" -o "$out/$name" ./cmd/messenger
  chmod 0755 "$out/$name"
  names+=("$name")
done
(cd "$out" && sha256sum "${names[@]}" > AgentMessenger-Windows-Linux-SHA256.txt)
ls -l "$out"
