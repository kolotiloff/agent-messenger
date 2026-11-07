#!/usr/bin/env bash
# Builds the Windows and Linux archives of one version: messenger with the
# version and revision built in, plus a short instruction file. Pure Go, no
# signing. Usage: package-others.sh VERSION REVISION OUTDIR
set -euo pipefail

version="$1"
revision="$2"
out="$3"
go_bin="${GO_BIN:-go}"
[[ "$version" =~ ^[0-9A-Za-z.+-]+$ ]] || { echo "bad version" >&2; exit 1; }
mkdir -p "$out"
stage="$(mktemp -d)"
trap 'rm -rf "$stage"' EXIT
flags="-X github.com/kolotiloff/agent-messenger/internal/buildinfo.Version=$version -X github.com/kolotiloff/agent-messenger/internal/buildinfo.Revision=$revision"
for target in windows/amd64 windows/arm64 linux/amd64 linux/arm64; do
  os="${target%/*}"
  arch="${target#*/}"
  name="agent-messenger-$version-$os-$arch"
  dir="$stage/$name"
  mkdir -p "$dir"
  exe="messenger"
  [[ "$os" == windows ]] && exe="messenger.exe"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" "$go_bin" build -trimpath -buildvcs=false -ldflags "$flags" -o "$dir/$exe" ./cmd/messenger
  if [[ "$os" == windows ]]; then
    # CRLF so Notepad shows the lines on any Windows version.
    sed 's/$/\r/' packaging/README-windows.txt > "$dir/README.txt"
    (cd "$stage" && python3 - "$name" <<'PY'
import os, sys, zipfile
name = sys.argv[1]
with zipfile.ZipFile(name + ".zip", "w", zipfile.ZIP_DEFLATED) as z:
    for f in sorted(os.listdir(name)):
        z.write(os.path.join(name, f), os.path.join(name, f))
PY
    )
    mv "$stage/$name.zip" "$out/"
  else
    cp packaging/README-linux.txt "$dir/README.txt"
    chmod 0755 "$dir/$exe"
    tar -C "$stage" --owner=0 --group=0 --numeric-owner -czf "$out/$name.tar.gz" "$name"
  fi
done
(cd "$out" && sha256sum agent-messenger-"$version"-*.zip agent-messenger-"$version"-*.tar.gz > "agent-messenger-$version-windows-linux.sha256")
ls -l "$out"
