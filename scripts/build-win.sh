#!/usr/bin/env bash
# Cross-compiles coachly.exe from Mac and packages it as dist/coachly-win.zip.
# -H windowsgui suppresses the console window so double-click feels native.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

VERSION="${VERSION:-dev}"
DIST="$ROOT/dist"
mkdir -p "$DIST"

echo "→ Tailwind CSS"
"$ROOT/scripts/tailwind.sh"

echo "→ Build windows/amd64"
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
  go build -ldflags="-s -w -H windowsgui -X main.version=$VERSION" \
  -o "$DIST/coachly.exe" .

SIZE=$(du -sh "$DIST/coachly.exe" | cut -f1)
echo "→ coachly.exe ($SIZE)"

cd "$DIST"
rm -f coachly-win.zip
zip -q coachly-win.zip coachly.exe
echo "✓ Fertig: $DIST/coachly-win.zip"
