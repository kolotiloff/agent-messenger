#!/usr/bin/env bash
# Builds the Windows and Linux programs of one version, ready to download as
# they are (D-047): messenger with the version and revision built in, under a
# fixed name so a link to the newest release always works, plus a SHA-256
# list. With UPDATEDIR, also the update archives the window installs by
# itself (D-048): <os>-<arch>.zip with only messenger or messenger.exe
# inside; they are signed later, on the release job, never here.
# Usage: package-others.sh VERSION REVISION OUTDIR [UPDATEDIR]
set -euo pipefail

version="$1"
revision="$2"
out="$3"
updates="${4:-}"
go_bin="${GO_BIN:-go}"
[[ "$version" =~ ^[0-9A-Za-z.+-]+$ ]] || { echo "bad version" >&2; exit 1; }
mkdir -p "$out"
public_key="$(cat desktop/macos/update-public-key.txt)"
[[ "$public_key" =~ ^[A-Za-z0-9/+]{43}=$ ]] || { echo 'Invalid update public key' >&2; exit 1; }
flags="-X github.com/kolotiloff/agent-messenger/internal/buildinfo.Version=$version -X github.com/kolotiloff/agent-messenger/internal/buildinfo.Revision=$revision -X github.com/kolotiloff/agent-messenger/internal/buildinfo.UpdatePublicKey=$public_key"
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
  if [[ -n "$updates" ]]; then
    inner=messenger
    [[ "$os" == windows ]] && inner=messenger.exe
    mkdir -p "$updates"
    python3 -I - "$out/$name" "$inner" "$updates/$os-$arch.zip" <<'PY'
import sys, zipfile
source, inner, target = sys.argv[1:]
with zipfile.ZipFile(target, 'x', zipfile.ZIP_DEFLATED) as z:
    info = zipfile.ZipInfo(inner, date_time=(2020, 1, 1, 0, 0, 0))
    info.compress_type = zipfile.ZIP_DEFLATED
    info.external_attr = 0o100755 << 16
    info.create_system = 3
    with open(source, 'rb') as f:
        z.writestr(info, f.read())
PY
  fi
done
(cd "$out" && sha256sum "${names[@]}" > AgentMessenger-Windows-Linux-SHA256.txt)
ls -l "$out"
