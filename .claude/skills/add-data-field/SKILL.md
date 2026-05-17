---
name: add-data-field
description: Add a new field to one of coachly's core data types (Athlete, Coach, Finanzamt, Invoice, SMTP). Use when the user says "add a field for X to athletes", "store X on the coach record", "track X on invoices", or any variant. Handles the coordinated change across schema, migration, form, table, and i18n so nothing is forgotten. Asks where the field belongs and whether it's required.
---

# Skill: add-data-field

Adding a field touches several files. This skill prevents the classic "added to struct, forgot the form" bug. Follow the steps in order.

## 1. Clarify

Before editing, confirm with the user:

- **Which type?** Athlete / Coach / Finanzamt / Invoice / SMTP.
- **Field name + Go type.** Use existing patterns: strings for free text, `store.Money` for amounts (cents, int64), `time.Time` for dates, `*time.Time` for optional dates.
- **JSON tag.** Snake_case (e.g. `birth_date`). Required vs optional drives the `omitempty` on the JSON tag.
- **UI label (German).** This will go into `internal/i18n/de.go`.

## 2. Schema

Open `internal/store/model.go`:

- Add the field to the right struct with its JSON tag.
- **If existing stores in the wild might lack this field**, bump `SchemaVersion` and add a case to `migrate()` in `internal/store/store.go` that sets a sensible default. New fields with zero values usually don't need a migration — Go's JSON unmarshal fills them with the zero value.

## 3. Persistence

No work needed — `Store.Mutate` already round-trips the new field via JSON.

## 4. Form

Find the form template for that type:

- `internal/web/templates/athlete_form.html` — Athlete
- `internal/web/templates/settings.html` — Coach, Finanzamt, SMTP (per-tab)
- (Invoices are auto-generated; rarely accept user input fields directly.)

Add an input following the existing rounded-lg pattern. Use the i18n key, e.g. `{{t "athletes.birth_date"}}`.

## 5. Form parser

Find the matching `parseXxxForm` or inline handler:

- Athletes: `parseAthleteForm` in `internal/web/handlers_athletes.go`
- Coach/Finanzamt/SMTP: handlers in `internal/web/handlers_settings.go`

Read the field from `r.FormValue("...")` and assign. For dates use `time.Parse("2006-01-02", v)`. For money: `strconv.ParseFloat(strings.Replace(v, ",", ".", 1), 64) * 100` cast to `store.Money`.

## 6. Table / list view

If the field should also appear in the athlete table or another list:

- Add a column header to `internal/web/templates/athletes.html` (or the relevant list template).
- Add a column cell in the `{{range}}` block.
- Update the corresponding row VM in the handler (e.g. `athleteRow` in `handlers_athletes.go`).

## 7. i18n

Add the German label to `internal/i18n/de.go` under the right group (e.g. `"athletes.birth_date": "Geburtsdatum"`).

## 8. PDF (only if visible on the invoice)

If the field should appear on the invoice PDF, edit `internal/pdf/template.go`. The layout uses `m.AddRow(height, text.NewCol(width, ...))`. Keep the single-page constraint (FR-P-01).

## 9. Requirement

Add or update the requirement in `.requirements/project_requirements.md`. New IDs go to the **end of the group**, never renumber existing ones. Add a row to the Änderungsprotokoll (§9).

## 10. Verify

```bash
go build ./...
go test ./...
go run .
```

Manually exercise the form: create + edit + delete a record with the new field, restart the app, confirm the value survived encryption round-trip.
