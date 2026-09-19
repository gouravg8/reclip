#!/usr/bin/env bash
# Fetch static ffmpeg + yt-dlp binaries into sidecar/<os>/.
# Usage: bash scripts/fetch-sidecars.sh [windows|linux|darwin|all]
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OS="${1:-all}"

YTDLP_VER="2025.01.01"

fetch_yt_dlp() {
  local dir="$1" asset="$2"
  mkdir -p "$dir"
  echo "-> yt-dlp $asset"
  curl -fsSL -o "$dir/$asset" "https://github.com/yt-dlp/yt-dlp/releases/download/${YTDLP_VER}/${asset}"
  chmod +x "$dir/$asset" || true
}

fetch_linux() {
  local dir="$ROOT/sidecar/linux"
  mkdir -p "$dir"
  echo "-> ffmpeg static (johnvansickle)"
  curl -fsSL -o /tmp/ffmpeg-linux.tar.xz "https://johnvansickle.com/ffmpeg/releases/ffmpeg-release-amd64-static.tar.xz"
  tar -xf /tmp/ffmpeg-linux.tar.xz -C /tmp
  cp /tmp/ffmpeg-*-amd64-static/ffmpeg "$dir/"
  rm -rf /tmp/ffmpeg-*-amd64-static /tmp/ffmpeg-linux.tar.xz
  fetch_yt_dlp "$dir" "yt-dlp_linux"
  mv "$dir/yt-dlp_linux" "$dir/yt-dlp"
}

fetch_windows() {
  local dir="$ROOT/sidecar/windows"
  mkdir -p "$dir"
  echo "-> ffmpeg static (gyan.dev essentials)"
  curl -fsSL -o /tmp/ffmpeg-win.zip "https://www.gyan.dev/ffmpeg/builds/ffmpeg-release-essentials.zip"
  rm -rf /tmp/ffmpeg-win && mkdir -p /tmp/ffmpeg-win
  unzip -q -o /tmp/ffmpeg-win.zip -d /tmp/ffmpeg-win
  cp /tmp/ffmpeg-win/ffmpeg-*-essentials_build/bin/ffmpeg.exe "$dir/"
  rm -rf /tmp/ffmpeg-win /tmp/ffmpeg-win.zip
  fetch_yt_dlp "$dir" "yt-dlp.exe"
}

fetch_darwin() {
  local dir="$ROOT/sidecar/darwin"
  mkdir -p "$dir"
  echo "-> ffmpeg static (evermeet)"
  curl -fsSL -o /tmp/ffmpeg-mac.zip "https://evermeet.cx/ffmpeg/ffmpeg-7.1.zip"
  rm -rf /tmp/ffmpeg-mac && mkdir -p /tmp/ffmpeg-mac
  unzip -q -o -j /tmp/ffmpeg-mac.zip -d /tmp/ffmpeg-mac
  cp /tmp/ffmpeg-mac/ffmpeg "$dir/"
  rm -rf /tmp/ffmpeg-mac /tmp/ffmpeg-mac.zip
  fetch_yt_dlp "$dir" "yt-dlp_macos"
  mv "$dir/yt-dlp_macos" "$dir/yt-dlp"
}

case "$OS" in
  windows) fetch_windows ;;
  linux) fetch_linux ;;
  darwin) fetch_darwin ;;
  all) fetch_windows; fetch_linux; fetch_darwin ;;
  *) echo "usage: $0 [windows|linux|darwin|all]"; exit 1 ;;
esac

echo "Done. Binaries are in sidecar/$OS/ (git-ignored)."
