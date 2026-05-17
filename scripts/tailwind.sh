#!/usr/bin/env bash
# Builds ui/dist/app.css from ui/src/input.css using the Tailwind standalone CLI v4.
# No Node required. The CLI binary is downloaded to ./bin/ on first run.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BIN_DIR="$ROOT/bin"
mkdir -p "$BIN_DIR"

case "$(uname -s)-$(uname -m)" in
  Darwin-arm64) ASSET="tailwindcss-macos-arm64" ;;
  Darwin-x86_64) ASSET="tailwindcss-macos-x64" ;;
  Linux-x86_64) ASSET="tailwindcss-linux-x64" ;;
  Linux-aarch64) ASSET="tailwindcss-linux-arm64" ;;
  *) echo "unsupported host: $(uname -s)-$(uname -m)" >&2; exit 1 ;;
esac

CLI="$BIN_DIR/tailwindcss"
if [ ! -x "$CLI" ]; then
  echo "Lade Tailwind standalone CLI ($ASSET)…"
  URL="https://github.com/tailwindlabs/tailwindcss/releases/latest/download/$ASSET"
  curl -fsSL "$URL" -o "$CLI"
  chmod +x "$CLI"
fi

mkdir -p "$ROOT/ui/dist"
cd "$ROOT/ui"
"$CLI" -i src/input.css -o dist/app.css --minify "$@"
echo "✓ ui/dist/app.css gebaut ($(wc -c < dist/app.css | tr -d ' ') Byte)."
