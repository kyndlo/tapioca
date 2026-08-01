#!/bin/sh
set -eu

repo="kyndlo/tapioca"
install_root="${TAPIOCA_INSTALL_DIR:-$HOME/.local/tapioca}"
bin_root="${TAPIOCA_BIN_DIR:-$HOME/.local/bin}"

os="$(uname -s)"
arch="$(uname -m)"
case "$os:$arch" in
  Darwin:arm64) asset="tapioca-darwin-arm64.tar.gz" ;;
  Linux:x86_64|Linux:amd64) asset="tapioca-linux-amd64.tar.gz" ;;
  Linux:aarch64|Linux:arm64) asset="tapioca-linux-arm64.tar.gz" ;;
  *)
    echo "tapioca: unsupported platform $os/$arch" >&2
    exit 1
    ;;
esac

command -v curl >/dev/null 2>&1 || {
  echo "tapioca: curl is required" >&2
  exit 1
}
command -v tar >/dev/null 2>&1 || {
  echo "tapioca: tar is required" >&2
  exit 1
}

tmp_dir="$(mktemp -d "${TMPDIR:-/tmp}/tapioca-install.XXXXXX")"
trap 'rm -rf "$tmp_dir"' EXIT INT TERM
url="https://github.com/$repo/releases/latest/download/$asset"

echo "Downloading Tapioca for $os/$arch..."
curl -fL "$url" -o "$tmp_dir/$asset"
mkdir -p "$install_root" "$bin_root"
tar -xzf "$tmp_dir/$asset" -C "$install_root"
ln -sf "$install_root/tapioca" "$bin_root/tapioca"

echo "Installed Tapioca at $install_root"
case ":$PATH:" in
  *":$bin_root:"*) ;;
  *)
    echo "Add this to your shell profile:"
    echo "  export PATH=\"$bin_root:\$PATH\""
    ;;
esac
echo "Run: tapioca version"
