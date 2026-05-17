# Build coachly for Windows

Output: a single `coachly.exe` (≈14 MB) packaged into `coachly-win.zip`. The build cross-compiles from macOS — you do **not** need a Windows machine.

## Prerequisites (on Mac)

- **Go ≥ 1.22** (`brew install go`)
- `zip` (preinstalled on macOS)

## One command

```bash
./scripts/build-win.sh
```

The script:

1. Builds `ui/dist/app.css` via Tailwind.
2. Cross-compiles with `GOOS=windows GOARCH=amd64 -ldflags="-H windowsgui -s -w"`.
   - `-H windowsgui` suppresses the black console window that would otherwise pop up on double-click. This is the single most important flag for the "feels native" experience.
3. Zips `coachly.exe` into `dist/coachly-win.zip`.

Versioning works the same way as on Mac: `VERSION=1.0.0 ./scripts/build-win.sh`.

## App icon

To embed an `.ico` into the executable, install [`rsrc`](https://github.com/akavel/rsrc) once:

```bash
go install github.com/akavel/rsrc@latest
```

Then before `go build`:

```bash
~/go/bin/rsrc -ico assets/icon/coachly.ico -o cmd/winres.syso
```

The `.syso` file is automatically linked when building for Windows. Add it to `.gitignore`. (This is optional — without it the exe just gets the default Windows icon.)

## Distribution & first-launch experience

Send `dist/coachly-win.zip`. End-user steps:

1. Unzip.
2. Double-click `coachly.exe`.
3. Windows SmartScreen may show **„Windows hat Ihren PC geschützt"** because the binary is unsigned and unknown to Microsoft. Users click **„Weitere Informationen"** → **„Trotzdem ausführen"**. After that one-time confirmation, normal double-click works.

This is the Windows equivalent of macOS's right-click → Open ceremony. SmartScreen reputation builds up automatically after a few hundred downloads — no action needed from us.

## Code signing (optional, much later)

For unsigned binaries the SmartScreen warning is the price of doing business. If you want to skip it from day one, you'd buy an OV code-signing certificate (~$200/year from Sectigo/DigiCert) and run `signtool` on `coachly.exe`. Not worth it for a small friend-distribution app.

## Troubleshooting

| Symptom | Fix |
|---|---|
| Build fails with `unable to find a Windows SDK` | Shouldn't happen with `CGO_ENABLED=0` (the build script sets this). If you removed that flag, put it back. |
| Double-click flashes a black window then closes | The `-H windowsgui` flag is missing or the binary is panicking on startup. Run from a `cmd` window to see the stack trace. |
| Browser doesn't auto-open | Windows default-browser handler is broken (rare). The console-less binary can't tell the user, so we accept this: users can manually go to `http://127.0.0.1:<port>/`, port shown in the app's log file at `%APPDATA%\coachly\coachly.log` (if logging is enabled). |
