#!/bin/sh
# tmax installer — downloads the latest release binary for your OS/arch.
#
#   curl -fsSL https://raw.githubusercontent.com/o1x3/tmax/main/install.sh | sh
#
# Environment:
#   TMAX_VERSION   install a specific tag (e.g. v0.1.0). Default: latest.
#   TMAX_BIN_DIR   install location. Default: /usr/local/bin if writable, else ~/.local/bin.
set -eu

REPO="o1x3/tmax"
BIN="tmax"

info() { printf '\033[36m%s\033[0m\n' "$*"; }
warn() { printf '\033[33m%s\033[0m\n' "$*"; }
err()  { printf '\033[31merror:\033[0m %s\n' "$*" >&2; exit 1; }

# --- detect platform ---
os=$(uname -s)
case "$os" in
  Darwin) os=darwin ;;
  Linux)  os=linux ;;
  *) err "unsupported OS: $os (tmax ships darwin and linux builds)" ;;
esac

arch=$(uname -m)
case "$arch" in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *) err "unsupported architecture: $arch" ;;
esac

asset="${BIN}_${os}_${arch}.tar.gz"

# --- downloader ---
if command -v curl >/dev/null 2>&1; then
  dlo() { curl -fsSL "$1" -o "$2"; }
elif command -v wget >/dev/null 2>&1; then
  dlo() { wget -qO "$2" "$1"; }
else
  err "need curl or wget to download"
fi

# --- resolve release URL ---
version="${TMAX_VERSION:-latest}"
if [ "$version" = "latest" ]; then
  base="https://github.com/${REPO}/releases/latest/download"
else
  base="https://github.com/${REPO}/releases/download/${version}"
fi

tmp=$(mktemp -d 2>/dev/null || mktemp -d -t tmax)
trap 'rm -rf "$tmp"' EXIT

info "Downloading ${asset} (${version})…"
dlo "${base}/${asset}" "$tmp/$asset" || err "download failed: ${base}/${asset}"

# --- verify checksum (best effort) ---
if dlo "${base}/checksums.txt" "$tmp/checksums.txt" 2>/dev/null; then
  if command -v sha256sum >/dev/null 2>&1; then sum=$(sha256sum "$tmp/$asset" | awk '{print $1}')
  elif command -v shasum   >/dev/null 2>&1; then sum=$(shasum -a 256 "$tmp/$asset" | awk '{print $1}')
  else sum=""; fi
  if [ -n "$sum" ]; then
    want=$(awk -v f="$asset" '$2==f {print $1}' "$tmp/checksums.txt")
    if [ -n "$want" ] && [ "$sum" != "$want" ]; then
      err "checksum mismatch for $asset — aborting"
    fi
  fi
fi

# --- extract ---
tar -xzf "$tmp/$asset" -C "$tmp" || err "could not extract $asset"
[ -f "$tmp/$BIN" ] || err "binary '$BIN' not found in archive"
chmod +x "$tmp/$BIN"

# --- choose install dir ---
if [ -n "${TMAX_BIN_DIR:-}" ]; then
  dir="$TMAX_BIN_DIR"
elif [ -d /usr/local/bin ] && [ -w /usr/local/bin ]; then
  dir="/usr/local/bin"
else
  dir="$HOME/.local/bin"
fi
mkdir -p "$dir" 2>/dev/null || true

if [ -w "$dir" ]; then
  mv "$tmp/$BIN" "$dir/$BIN"
elif command -v sudo >/dev/null 2>&1; then
  info "Installing to $dir (requires sudo)…"
  sudo mv "$tmp/$BIN" "$dir/$BIN"
else
  err "cannot write to $dir; set TMAX_BIN_DIR to a writable directory"
fi

info "Installed $BIN → $dir/$BIN"
case ":$PATH:" in
  *":$dir:"*) ;;
  *) warn "Note: $dir is not on your PATH. Add this to your shell profile:"
     printf '  export PATH="%s:$PATH"\n' "$dir" ;;
esac
"$dir/$BIN" version 2>/dev/null || true
