#!/usr/bin/env bash
set -euo pipefail
# Pin the official release, including the downloaded bytes. Build-time only.
[[ $# == 1 && "$1" == /* ]] || { echo 'Absolute output directory required' >&2; exit 1; }
cache="$1"
mkdir -p "$cache"
archive="$cache/Sparkle-2.10.1.tar.xz"
if [[ ! -f "$archive" ]]; then
  curl --fail --location --proto '=https' --tlsv1.2 --max-time 600 \
    'https://github.com/sparkle-project/Sparkle/releases/download/2.10.1/Sparkle-2.10.1.tar.xz' -o "$archive.part"
  mv "$archive.part" "$archive"
fi
[[ "$(shasum -a 256 "$archive" | cut -d ' ' -f 1)" == '1a856d2a9edb8e628de88121d7be04cf0e2167f8641650b78fca00b14244cf27' ]] || { echo 'Sparkle checksum mismatch' >&2; exit 1; }
tar -xJf "$archive" -C "$cache"
