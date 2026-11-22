#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")" && pwd)"
BIN="$ROOT/dist/signal-echo-linux-amd64"
if [[ ! -x "$BIN" ]]; then
  echo "Build first: ./build.sh" >&2
  exit 1
fi
sudo install -Dm755 "$BIN" /usr/local/bin/signal-echo
sudo install -Dm644 "$ROOT/assets/icon.png" /usr/share/icons/hicolor/256x256/apps/signal-echo.png
sudo install -Dm644 "$ROOT/linux/signal-echo.desktop" /usr/share/applications/signal-echo.desktop
if command -v gtk-update-icon-cache >/dev/null 2>&1; then
  sudo gtk-update-icon-cache -f /usr/share/icons/hicolor >/dev/null 2>&1 || true
fi
echo "Signal Echo installed."
