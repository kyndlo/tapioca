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
curl -fL "$url.sha256" -o "$tmp_dir/$asset.sha256"
expected="$(awk '{print $1}' "$tmp_dir/$asset.sha256")"
if command -v shasum >/dev/null 2>&1; then
  actual="$(shasum -a 256 "$tmp_dir/$asset" | awk '{print $1}')"
elif command -v sha256sum >/dev/null 2>&1; then
  actual="$(sha256sum "$tmp_dir/$asset" | awk '{print $1}')"
else
  echo "tapioca: shasum or sha256sum is required" >&2
  exit 1
fi
if [ -z "$expected" ] || [ "$actual" != "$expected" ]; then
  echo "tapioca: downloaded archive checksum did not match" >&2
  exit 1
fi
mkdir -p "$install_root" "$bin_root"
tar -xzf "$tmp_dir/$asset" -C "$install_root"
ln -sf "$install_root/tapioca" "$bin_root/tapioca"

echo "Installed Tapioca at $install_root"
case ":$PATH:" in
  *":$bin_root:"*) ;;
  *)
    if [ "${TAPIOCA_UPDATE_PATH:-1}" = "1" ]; then
      shell_name="$(basename "${SHELL:-sh}")"
      case "$shell_name" in
        zsh) profile="$HOME/.zprofile" ;;
        bash) profile="$HOME/.bash_profile" ;;
        *) profile="$HOME/.profile" ;;
      esac
      path_line="export PATH=\"$bin_root:\$PATH\""
      touch "$profile"
      if ! grep -F "$path_line" "$profile" >/dev/null 2>&1; then
        printf '\n# Added by the Tapioca installer\n%s\n' "$path_line" >> "$profile"
      fi
      echo "Added Tapioca to PATH in $profile"
      echo "Open a new terminal, or run: export PATH=\"$bin_root:\$PATH\""
    else
      echo "Add this to your shell profile:"
      echo "  export PATH=\"$bin_root:\$PATH\""
    fi
    ;;
esac
"$install_root/tapioca" version
echo "Next: tapioca run qwen3:4b-q4_k_m"
