#!/bin/sh
# Isolated vision runtime: never replaces the shared text-model runtime.
set -eu
[ "$(uname -s)/$(uname -m)" = Darwin/arm64 ] || { echo 'Vision runtime is only qualified for macOS arm64' >&2; exit 1; }
version=b11696
sha=6aac007fbe153efefe06250d2cf05d0487583aa0706379082ebef2b05579f649
root=${1:-bin/runtime}
mkdir -p "$root"
# Versioned install avoids reusing the old shared runtime on upgrades.
staging=$(mktemp -d "$root/.vision-XXXXXX")
trap 'rm -rf "$staging"' EXIT HUP INT TERM
curl -fL --retry 3 "https://github.com/ggml-org/llama.cpp/releases/download/$version/llama-$version-bin-macos-arm64.tar.gz" -o "$staging/archive.tar.gz"
printf '%s  %s\n' "$sha" "$staging/archive.tar.gz" | shasum -a 256 -c -
mkdir "$staging/runtime"
tar -xzf "$staging/archive.tar.gz" -C "$staging/runtime" --strip-components=1
"$staging/runtime/llama-server" --version 2>&1 | grep 'build 11696, commit ceefd16da'
rm -rf "$root/llama-vision-$version"
mv "$staging/runtime" "$root/llama-vision-$version"
