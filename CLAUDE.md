# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`coachly` is an encrypted, offline-first invoicing app for a single coach. The whole product ships as one Go binary per OS: friends download it, double-click, and get a local web UI in their browser. No installers, no Node/Python on the user's machine. Data lives in an AES-256-GCM-encrypted JSON blob under the OS app-data dir. The product spec — every feature, every requirement ID we reference in commits and PRs — is [`.requirements/project_requirements.md`](.requirements/project_requirements.md). Read it first.

## Run & build

```bash
./scripts/tailwind.sh     # build ui/dist/app.css (only needed after CSS changes)
go run .                  # dev: starts server, opens browser
go test ./...             # all tests
go test ./internal/invoice -run TestProRata   # one test
./scripts/build-mac.sh    # universal .app + zip in dist/
./scripts/build-win.sh    # coachly.exe + zip in dist/
```

The Tailwind standalone CLI is downloaded to `./bin/` on first run — no Node needed.

## Architecture in 60 seconds

`main.go` resolves the data dir, picks a free port, starts the chi router, opens the browser. The router lives in `internal/web/server.go`. All authenticated routes pass through `requireAuth`, which checks an in-memory session cookie. On `/login` POST, `internal/auth` runs Argon2id over the password + a per-file salt to derive an AES-256-GCM key, then `internal/store` decrypts `store.enc` into an in-memory `Data` struct. All writes go through `Store.Mutate(func(*Data) error)` which holds a mutex, mutates in memory, JSON-marshals, re-seals with the cached session key+salt (no re-running Argon2id per write), and atomically writes via `tmp + fsync + rename`.

Invoice generation is a two-phase ceremony to keep numbering and PDF write decoupled: `invoice.BuildBatch` consumes invoice numbers and persists drafts with `status: "pending_pdf"`; `web.handleInvoicesConfirm` renders PDFs via `pdf.Render` and calls `invoice.Finalize` to flip each invoice to `"issued"` with its PDF path. If PDF rendering crashes after numbers were consumed, the drafts are still in the store as `pending_pdf` and can be reconciled rather than re-issued under new numbers.

UI: each handler builds a `view` (a `map[string]any`), calls `renderPage(w, "athletes.html", v)`. Because Go's `html/template` forbids duplicate `{{define "content"}}` blocks across one set, we parse `layout.html + <contentFile>` fresh per request. Cheap and unambiguous.

## Where things live

- `internal/store/model.go` — the schema. Bump `SchemaVersion` if you change shapes; add a case in `migrate()`.
- `internal/store/store.go` — encrypted load/save, atomic write, `ChangePassword`.
- `internal/auth/crypto.go` — Argon2id + AES-GCM. The file format header (`CLY1` magic + version byte) is documented in the package doc; don't break it without bumping the version byte.
- `internal/invoice/prorata.go` — day-based pro-rata, half-up rounding on cents. Money is `int64` cents everywhere — never floats.
- `internal/invoice/numbering.go` — `BuildBatch` / `Finalize`, the `pending_pdf → issued` lifecycle.
- `internal/pdf/template.go` — single-page invoice (FR-P-*). Edit here when the invoice layout changes.
- `internal/web/templates/` — HTML templates. `layout.html` defines the chrome (sidebar + topbar). Each page file defines its own `{{define "content"}}`.
- `internal/i18n/de.go` — all user-facing strings (NFR-L-02). Never hardcode German in handlers or templates; always go through `i18n.T("key")`.

## Conventions

- **Money is `store.Money` (= `int64` cents).** Format with `invoice.FormatEUR`. Never use floats for amounts.
- **Invoice numbers come from `Counter.NextInvoiceNumber` via `BuildBatch` only.** Never increment that field by hand.
- **All user-facing strings via `i18n.T(...)`** — even in templates (`{{t "key"}}`). New strings go into `internal/i18n/de.go`.
- **Date format `02.01.2006`** in PDFs and UI (NFR-L-03). The browser-native `<input type="date">` uses `YYYY-MM-DD` for round-tripping with handlers; convert at the boundary.
- **Mutations** happen inside `Store.Mutate(func(*Data) error)`. Handlers never read `store.enc` or call `Seal`/`Open` directly.
- **PDF filenames** are produced by `invoice.PDFFilename(firstName, year, month, number)` — see FR-P-09. Keep them stable; users will reference these on disk.

## Requirements

`.requirements/project_requirements.md` is the source of truth. Each requirement has a stable ID (`FR-A-01`, `NFR-S-04`, etc.). When adding features:

1. Add or update the requirement first (use the next free ID in the group; never renumber).
2. Add an entry to the **Änderungsprotokoll** (§9) — date + ID + one-line change description.
3. Reference the ID in commits and PR descriptions.

## UI

Reference: [`.requirements/ui-standard.png`](.requirements/ui-standard.png). The product is intentionally a *simplified* version of that mockup — only Dashboard / Athleten / Rechnungen / Auswertungen / Einstellungen. Design tokens: dark sidebar (`slate-900`), teal accent (`teal-500`), cards `rounded-2xl`, inputs `rounded-lg`, primary CTAs `rounded-full`. Tailwind v4 — config is CSS-first inside `ui/src/input.css` (no `tailwind.config.js`).

## Common gotchas

- **First launch state:** if `store.enc` doesn't exist, `/login` shows the setup form (password + repeat). If it exists, it shows the unlock form. Tests that wipe state should delete the file under `~/Library/Application Support/coachly/`.
- **Embedded assets:** `main.go` embeds `assets/` and `ui/dist/` via `//go:embed`. If you add a new vendored JS file, drop it under `assets/` and reference it from `layout.html` — no code change needed.
- **Argon2id is intentionally slow** (~300 ms on M-series). Only runs on unlock/setup, never on per-mutation saves (those reuse the cached key+salt).
