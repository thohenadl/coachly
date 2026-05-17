#!/usr/bin/env bash
# Builds a universal coachly.app bundle for macOS and packages it as
# dist/coachly-mac.zip. Pure Go, no Xcode / signing required.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

VERSION="${VERSION:-dev}"
APP_NAME="coachly"
BUNDLE_ID="app.coachly"

DIST="$ROOT/dist"
APP="$DIST/$APP_NAME.app"

echo "→ Tailwind CSS"
"$ROOT/scripts/tailwind.sh"

rm -rf "$DIST"
mkdir -p "$APP/Contents/MacOS" "$APP/Contents/Resources"

echo "→ Build arm64"
GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 \
  go build -ldflags="-s -w -X main.version=$VERSION" -o "$DIST/${APP_NAME}-arm64" .

echo "→ Build amd64"
GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 \
  go build -ldflags="-s -w -X main.version=$VERSION" -o "$DIST/${APP_NAME}-amd64" .

echo "→ lipo universal"
lipo -create -output "$APP/Contents/MacOS/$APP_NAME" \
  "$DIST/${APP_NAME}-arm64" "$DIST/${APP_NAME}-amd64"
chmod +x "$APP/Contents/MacOS/$APP_NAME"
rm "$DIST/${APP_NAME}-arm64" "$DIST/${APP_NAME}-amd64"

cat > "$APP/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>CFBundleExecutable</key><string>$APP_NAME</string>
  <key>CFBundleIdentifier</key><string>$BUNDLE_ID</string>
  <key>CFBundleName</key><string>coachly</string>
  <key>CFBundleDisplayName</key><string>coachly</string>
  <key>CFBundlePackageType</key><string>APPL</string>
  <key>CFBundleVersion</key><string>$VERSION</string>
  <key>CFBundleShortVersionString</key><string>$VERSION</string>
  <key>LSMinimumSystemVersion</key><string>11.0</string>
  <key>NSHighResolutionCapable</key><true/>
  <key>LSUIElement</key><false/>
</dict>
</plist>
EOF

# Icon: use placeholder until a real one is dropped into assets/icon/AppIcon.icns
if [ -f "$ROOT/assets/icon/AppIcon.icns" ]; then
  cp "$ROOT/assets/icon/AppIcon.icns" "$APP/Contents/Resources/AppIcon.icns"
fi

SIZE=$(du -sh "$APP" | cut -f1)
echo "→ App bundle: $APP ($SIZE)"

cd "$DIST"
ditto -c -k --sequesterRsrc --keepParent "$APP_NAME.app" "$APP_NAME-mac.zip"
echo "✓ Fertig: $DIST/$APP_NAME-mac.zip"
