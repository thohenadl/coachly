# Build coachly for macOS

This guide is for **you** (the developer), not end users. Output is a single `coachly-mac.zip` containing a universal `coachly.app` bundle that runs on both Apple Silicon and Intel Macs.

## Prerequisites

- **Go ≥ 1.22** — install via Homebrew: `brew install go`. No CGO, no Xcode required.
- Standard macOS command-line tools (`lipo`, `ditto`, `curl`) — already on every Mac.
- Internet on first build (`scripts/tailwind.sh` downloads the Tailwind standalone CLI to `./bin/`).

## One command

```bash
./scripts/build-mac.sh
```

That's it. The script:

1. Builds `ui/dist/app.css` via the Tailwind standalone CLI (auto-downloaded on first run).
2. Cross-compiles `darwin/arm64` and `darwin/amd64` binaries with `-ldflags="-s -w"` (strips symbols).
3. Combines them into a universal binary using `lipo`.
4. Assembles `dist/coachly.app/Contents/{MacOS, Info.plist, Resources}`.
5. Packages everything into `dist/coachly-mac.zip` with `ditto -c -k --sequesterRsrc --keepParent`.

Typical sizes: app bundle ~26 MB, zip ~10 MB.

## Versioning

```bash
VERSION=1.0.0 ./scripts/build-mac.sh
```

`VERSION` is baked into `Info.plist` (`CFBundleShortVersionString`) and into the binary via `-X main.version=$VERSION`. Default is `dev`.

## App icon

Drop a `AppIcon.icns` into `assets/icon/AppIcon.icns` before building. The script copies it into the bundle. Without it the app gets the macOS default icon — not pretty but harmless.

Generate `.icns` from a 1024×1024 PNG:

```bash
mkdir AppIcon.iconset
sips -z 16 16     icon.png --out AppIcon.iconset/icon_16x16.png
sips -z 32 32     icon.png --out AppIcon.iconset/icon_16x16@2x.png
sips -z 32 32     icon.png --out AppIcon.iconset/icon_32x32.png
sips -z 64 64     icon.png --out AppIcon.iconset/icon_32x32@2x.png
sips -z 128 128   icon.png --out AppIcon.iconset/icon_128x128.png
sips -z 256 256   icon.png --out AppIcon.iconset/icon_128x128@2x.png
sips -z 256 256   icon.png --out AppIcon.iconset/icon_256x256.png
sips -z 512 512   icon.png --out AppIcon.iconset/icon_256x256@2x.png
sips -z 512 512   icon.png --out AppIcon.iconset/icon_512x512.png
cp icon.png AppIcon.iconset/icon_512x512@2x.png
iconutil -c icns AppIcon.iconset -o assets/icon/AppIcon.icns
```

## Distribution

Send `dist/coachly-mac.zip` to your friends. The first-launch experience for them: unzip → drag `coachly.app` into Applications → **right-click → Open** → confirm. After that, normal double-click. Detailed end-user instructions are in [`USER_GUIDE.de.md`](USER_GUIDE.de.md).

---

## Appendix: when you eventually get an Apple Developer ID

Unsigned apps work but force users through the right-click → Open dance once and trip Gatekeeper warnings. If/when you sign up for the Apple Developer Program ($99/year), the build pipeline gains two extra steps:

```bash
# 1. Codesign (replace TEAMID with yours)
codesign --deep --force --options=runtime \
  --sign "Developer ID Application: Your Name (TEAMID)" \
  dist/coachly.app

# 2. Re-zip after signing
cd dist && ditto -c -k --sequesterRsrc --keepParent coachly.app coachly-mac.zip

# 3. Submit to Apple's notarization service
xcrun notarytool submit coachly-mac.zip \
  --apple-id you@example.com \
  --team-id TEAMID \
  --password "app-specific-password" \
  --wait

# 4. Staple the notarization ticket so it works offline
xcrun stapler staple dist/coachly.app
cd dist && ditto -c -k --sequesterRsrc --keepParent coachly.app coachly-mac.zip
```

After notarization, users just double-click — no right-click ceremony, no warnings.

## Troubleshooting

| Symptom | Fix |
|---|---|
| `go: command not found` | `brew install go` |
| `lipo: can't figure out the architecture` | One of the two arch binaries failed to build — re-run, watch the output for the first failing `go build`. |
| Bundle launches but the browser stays at "this site can't be reached" | The embedded port might collide with something else. Check `Console.app` for the `coachly läuft auf …` line. |
| Bundle launches, browser opens but assets 404 | `./scripts/tailwind.sh` wasn't run before building — the build script invokes it, so this only happens if you ran `go build` by hand. |
