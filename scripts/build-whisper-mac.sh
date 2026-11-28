#!/usr/bin/env bash
set -euo pipefail

[[ $# == 2 && "$1" == /* ]] || { echo 'Usage: build-whisper-mac.sh ABSOLUTE_WORK_DIRECTORY ARCH' >&2; exit 1; }
work="$1"
arch="$2"
cmake_bin="${CMAKE_BIN:-cmake}"
command -v "$cmake_bin" >/dev/null || { echo 'CMake is required to build local dictation' >&2; exit 1; }

# whisper.cpp v1.9.3. Fetch the exact commit so mutable tags cannot change a build.
revision=371b5a7561823ab2bb32142d2751e35e7534727b
source_dir="$work/source"
mkdir -p "$source_dir"
git -C "$source_dir" init -q
if ! git -C "$source_dir" cat-file -e "$revision^{commit}" 2>/dev/null; then
  git -C "$source_dir" fetch --quiet --depth 1 https://github.com/ggml-org/whisper.cpp.git "$revision"
fi
git -C "$source_dir" checkout --quiet --detach "$revision"

"$cmake_bin" -S "$source_dir" -B "$work/build" \
  -DCMAKE_BUILD_TYPE=Release -DCMAKE_OSX_ARCHITECTURES="$arch" -DCMAKE_OSX_DEPLOYMENT_TARGET=15.0 \
  -DBUILD_SHARED_LIBS=OFF -DGGML_NATIVE=OFF -DGGML_METAL=OFF -DWHISPER_COREML=OFF \
  -DWHISPER_BUILD_TESTS=OFF -DWHISPER_BUILD_EXAMPLES=ON -DWHISPER_BUILD_SERVER=OFF
"$cmake_bin" --build "$work/build" --config Release --target whisper-cli --parallel
