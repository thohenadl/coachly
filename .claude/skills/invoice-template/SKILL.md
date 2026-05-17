---
name: invoice-template
description: Modify the PDF invoice layout. Use when the user says "change the invoice layout", "add X to the PDF", "move the logo", "the invoice should also show Y", or any request affecting how the rendered invoice PDF looks. Keeps changes traceable to FR-P-* and FR-T-* requirements and preserves the single-page constraint.
---

# Skill: invoice-template

The invoice PDF is rendered by `internal/pdf/template.go` using `maroto/v2`. Layout changes must stay consistent with the requirements in §4.3 (FR-P-*) and §4.4 (FR-T-*) of `.requirements/project_requirements.md`.

## Constraints — do not break

- **FR-P-01: single page.** Adding sections increases row count. After your change, render a test PDF and confirm it's exactly one page. Maroto auto-paginates silently — you have to look.
- **FR-P-09: filename format** `firstname_month_year_invoicenumber.pdf` is set by `invoice.PDFFilename`. Don't change it here.
- **Money formatting** always via `invoice.FormatEUR(...)`. Date formatting via `.Format("02.01.2006")`.

## Layout overview

In order, top to bottom (each function in `template.go`):

1. `addHeader` — "Rechnung" title + invoice number top-right
2. `addAddresses` — sender (Coach) left, recipient (Athlete) right, plus Finanzamt line
3. `addMeta` — Datum / Rechnungsnummer / Steuernummer / UID block (FR-P-02)
4. `addTipp` — Coaches Tipp (FR-P-03, FR-T-*) — italic, before the line item
5. `addLineItems` — Zeitraum / Beschreibung / Kosten table + Total (FR-P-04, FR-P-05)
6. `addPaymentText` — fixed text "Bitte überweise…" (FR-P-06)
7. `addBankBlock` — Bank / Kontoinhaber / IBAN / BIC (FR-P-07)
8. `addClosing` — Danke + Grußformel (FR-P-08)

## Maroto cheatsheet

- Grid is 12 columns wide.
- `m.AddRow(height, cols...)` adds one row. `height` is in millimeters.
- Empty filler col: `col.New(N)` where N is the number of columns to skip.
- Text props: `props.Text{Size, Style, Align, Top, Color}` where Style is from `consts/fontstyle`, Align from `consts/align`.

## Workflow

1. Read the requirement IDs involved (`grep "FR-P-" .requirements/project_requirements.md`). If the user is asking for something not in a requirement, add or amend one first (`.requirements/project_requirements.md` + Änderungsprotokoll).
2. Edit `internal/pdf/template.go` in the relevant `addXxx` function. Prefer adding a row to an existing helper over inventing a new one — keep the section order stable.
3. Build + render a test PDF:
   ```bash
   go test ./internal/pdf/... 2>/dev/null || true   # no tests yet, OK
   go run .                                          # then walk the create-invoice flow in the browser
   ```
4. Open the produced PDF in `~/Library/Application Support/coachly/invoices/` and verify:
   - Single page.
   - All required fields per FR-P-02 still present.
   - German formatting (date `02.01.2026`, money `1.234,56 €`).

## Things that have caught us before

- **Long Coaches Tipp** pushes content onto page 2. If the user wants long tipps, shorten other rows or implement a smarter height. Don't silently let it become a 2-pager.
- **Empty fields** (e.g. missing UID for a coach without one) leave awkward gaps. Wrap optional fields in `if value != ""` checks like `addTipp` does.
- **Currency / locale leakage:** never `fmt.Sprintf("%.2f €", ...)` — always `invoice.FormatEUR`.
