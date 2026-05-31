package invoice

import (
	"errors"
	"fmt"
	"time"

	"coachly/internal/store"
)

// ErrInvoiceExists indicates that a single-invoice build (single-month) was
// called for an athlete who already has an invoice covering that month.
var ErrInvoiceExists = errors.New("invoice already exists for athlete in month")

// IsEditable reports whether an invoice in the given status may be deleted
// or overwritten. Sent and Paid invoices are protected.
func IsEditable(st store.InvoiceStatus) bool {
	return st == store.StatusPending || st == store.StatusIssued
}

// TippInput is the input shape for BuildBatch.
// Default is the month-wide tipp (FR-T-01). PerAthlete contains optional
// per-athlete overrides (FR-T-04) keyed by Athlete.ID.
type TippInput struct {
	Default    string
	PerAthlete map[string]string
}

// LineSpec describes one billable month for a multi-month single invoice
// build. The pro-rata period is derived from the athlete's StartDate/EndDate
// inside the builder using PeriodFor.
type LineSpec struct {
	Year  int
	Month time.Month
}

// Conflict reports an existing invoice that already covers a requested
// month. Used by the single-invoice form to render per-row hints and the
// submit-time validator.
type Conflict struct {
	Month         string // "2026-05"
	InvoiceNumber int
	DisplayNumber string
	Status        store.InvoiceStatus
}

// BuildBatch composes draft invoices (status=pending_pdf) for all athletes
// active in the given month. Numbers are consumed atomically inside Mutate.
// PDF generation runs *after* this commits, then the caller calls Finalize.
//
// Dedup: skips any athlete that already has *any* invoice (single- or
// multi-month) whose Lines cover the target month.
func BuildBatch(s *store.Store, year int, month time.Month, tipp TippInput) ([]store.Invoice, error) {
	var out []store.Invoice
	err := s.Mutate(func(d *store.Data) error {
		monthKey := FormatMonth(year, month)
		upsertTipp(d, monthKey, tipp)

		invoiced := map[string]bool{}
		for _, inv := range d.Invoices {
			if inv.CoversMonth(monthKey) {
				invoiced[inv.AthleteID] = true
			}
		}

		now := time.Now().UTC()
		for _, a := range d.Athletes {
			if invoiced[a.ID] || !a.Active(year, month) {
				continue
			}
			p := PeriodFor(a, year, month)
			line := store.InvoiceLine{
				Month:       monthKey,
				PeriodFrom:  p.From,
				PeriodTo:    p.To,
				Description: fmt.Sprintf("Coaching %s %d", monthDE(month), year),
				Amount:      AmountFor(a, p),
				ProRata:     p.ProRata,
			}
			effectiveTipp := tipp.Default
			if v, ok := tipp.PerAthlete[a.ID]; ok && v != "" {
				effectiveTipp = v
			}
			num := d.Counter.NextInvoiceNumber
			inv := store.Invoice{
				Number:        num,
				DisplayNumber: FormatDisplayNumber(d.Preferences.NumberingFormat, year, month, num, a, d.Coach),
				AthleteID:     a.ID,
				Tipp:          effectiveTipp,
				Lines:         []store.InvoiceLine{line},
				Total:         line.Amount,
				Status:        store.StatusPending,
				IssuedAt:      now,
			}
			d.Counter.NextInvoiceNumber++
			d.Invoices = append(d.Invoices, inv)
			out = append(out, inv)
		}
		return nil
	})
	return out, err
}

// DeleteForMonth removes invoices whose lines fall *entirely* within
// monthKey (i.e. single-month invoices for that month). Multi-month
// invoices that happen to also cover monthKey are left untouched — they're
// the user's choice and removing them silently would surprise. Returns the
// PDF paths the caller must unlink. When skipProtected is true, Sent/Paid
// invoices are kept and their numbers are returned in keptProtected.
//
// Counter.NextInvoiceNumber is *not* rewound — re-creation with fresh
// numbers keeps the sequence monotonic. Re-creation with the same numbers
// happens via RebuildBatchSameNumbers.
func DeleteForMonth(s *store.Store, monthKey string, skipProtected bool) (removedPDFs []string, keptProtected []int, err error) {
	err = s.Mutate(func(d *store.Data) error {
		kept := d.Invoices[:0]
		for _, inv := range d.Invoices {
			if isSingleMonthInvoice(inv, monthKey) {
				if skipProtected && !IsEditable(inv.Status) {
					kept = append(kept, inv)
					keptProtected = append(keptProtected, inv.Number)
					continue
				}
				if inv.PDFPath != "" {
					removedPDFs = append(removedPDFs, inv.PDFPath)
				}
				continue
			}
			kept = append(kept, inv)
		}
		d.Invoices = kept
		return nil
	})
	return removedPDFs, keptProtected, err
}

// isSingleMonthInvoice reports whether inv has exactly one line, and that
// line is for monthKey. Multi-month invoices that touch monthKey return
// false — DeleteForMonth/RebuildBatchSameNumbers only deal with the
// monthly-batch case.
func isSingleMonthInvoice(inv store.Invoice, monthKey string) bool {
	return len(inv.Lines) == 1 && inv.Lines[0].Month == monthKey
}

// RebuildBatchSameNumbers deletes single-month editable invoices for the
// month and rebuilds drafts under the *same* invoice numbers. Protected
// invoices (Sent/Paid) and multi-month invoices that happen to touch this
// month are left untouched; protected single-month numbers come back in
// skippedProtected.
//
// New athletes (active in the month, never invoiced for it via any invoice)
// are added with fresh numbers from Counter.NextInvoiceNumber, so calling
// this for a month where coverage has grown still works.
//
// Caller is responsible for unlinking the returned PDF paths from disk
// *before* the new drafts get finalised.
func RebuildBatchSameNumbers(s *store.Store, year int, month time.Month, tipp TippInput) (drafts []store.Invoice, removedPDFs []string, skippedProtected []int, err error) {
	monthKey := FormatMonth(year, month)
	err = s.Mutate(func(d *store.Data) error {
		upsertTipp(d, monthKey, tipp)

		// Capture, per athlete, the existing single-month editable number to
		// reuse and whether they are protected (so we leave them out of the
		// rebuild entirely). Also collect athletes already covered by a
		// multi-month invoice — they're skipped silently (no reuse, no
		// rebuild) since the multi-month invoice was an intentional bundle.
		reuseNumber := map[string]int{}
		protectedAthletes := map[string]bool{}
		multiMonthCovered := map[string]bool{}
		kept := d.Invoices[:0]
		for _, inv := range d.Invoices {
			switch {
			case isSingleMonthInvoice(inv, monthKey):
				if IsEditable(inv.Status) {
					reuseNumber[inv.AthleteID] = inv.Number
					if inv.PDFPath != "" {
						removedPDFs = append(removedPDFs, inv.PDFPath)
					}
					continue
				}
				protectedAthletes[inv.AthleteID] = true
				skippedProtected = append(skippedProtected, inv.Number)
			case inv.CoversMonth(monthKey):
				multiMonthCovered[inv.AthleteID] = true
			}
			kept = append(kept, inv)
		}
		d.Invoices = kept

		now := time.Now().UTC()
		for _, a := range d.Athletes {
			if protectedAthletes[a.ID] || multiMonthCovered[a.ID] || !a.Active(year, month) {
				continue
			}
			p := PeriodFor(a, year, month)
			line := store.InvoiceLine{
				Month:       monthKey,
				PeriodFrom:  p.From,
				PeriodTo:    p.To,
				Description: fmt.Sprintf("Coaching %s %d", monthDE(month), year),
				Amount:      AmountFor(a, p),
				ProRata:     p.ProRata,
			}
			effectiveTipp := tipp.Default
			if v, ok := tipp.PerAthlete[a.ID]; ok && v != "" {
				effectiveTipp = v
			}
			number, reused := reuseNumber[a.ID]
			if !reused {
				number = d.Counter.NextInvoiceNumber
				d.Counter.NextInvoiceNumber++
			}
			inv := store.Invoice{
				Number:        number,
				DisplayNumber: FormatDisplayNumber(d.Preferences.NumberingFormat, year, month, number, a, d.Coach),
				AthleteID:     a.ID,
				Tipp:          effectiveTipp,
				Lines:         []store.InvoiceLine{line},
				Total:         line.Amount,
				Status:        store.StatusPending,
				IssuedAt:      now,
			}
			d.Invoices = append(d.Invoices, inv)
			drafts = append(drafts, inv)
		}
		return nil
	})
	return drafts, removedPDFs, skippedProtected, err
}

// RecreateSingleSameNumber deletes the existing single-month editable
// invoice for (athleteID, month) and rebuilds it under the same number
// with fresh pro-rata / tipp values. The previous PDF path is returned so
// the caller can unlink it. Returns ErrInvoiceExists with no mutation if
// the existing invoice is Sent or Paid, or is part of a multi-month
// invoice (cannot reuse the number cleanly — the coach must delete the
// multi-month invoice manually first).
func RecreateSingleSameNumber(s *store.Store, year int, month time.Month, athleteID, tippText string) (draft store.Invoice, oldPDFPath string, err error) {
	monthKey := FormatMonth(year, month)
	err = s.Mutate(func(d *store.Data) error {
		var athlete *store.Athlete
		for i := range d.Athletes {
			if d.Athletes[i].ID == athleteID {
				athlete = &d.Athletes[i]
				break
			}
		}
		if athlete == nil {
			return fmt.Errorf("athlete %s not found", athleteID)
		}
		var oldNumber int
		foundIdx := -1
		for i, inv := range d.Invoices {
			if inv.AthleteID != athleteID || !inv.CoversMonth(monthKey) {
				continue
			}
			if !IsEditable(inv.Status) {
				return ErrInvoiceExists
			}
			if !isSingleMonthInvoice(inv, monthKey) {
				// Multi-month invoice covers this month. Can't safely
				// "recreate with same number" — surface as a conflict and
				// let the coach delete the multi-month one explicitly.
				return ErrInvoiceExists
			}
			oldNumber = inv.Number
			oldPDFPath = inv.PDFPath
			foundIdx = i
			break
		}
		if foundIdx == -1 {
			return fmt.Errorf("invoice for athlete %s in %s not found", athleteID, monthKey)
		}
		d.Invoices = append(d.Invoices[:foundIdx], d.Invoices[foundIdx+1:]...)

		p := PeriodFor(*athlete, year, month)
		line := store.InvoiceLine{
			Month:       monthKey,
			PeriodFrom:  p.From,
			PeriodTo:    p.To,
			Description: fmt.Sprintf("Coaching %s %d", monthDE(month), year),
			Amount:      AmountFor(*athlete, p),
			ProRata:     p.ProRata,
		}
		draft = store.Invoice{
			Number:        oldNumber,
			DisplayNumber: FormatDisplayNumber(d.Preferences.NumberingFormat, year, month, oldNumber, *athlete, d.Coach),
			AthleteID:     athleteID,
			Tipp:          tippText,
			Lines:         []store.InvoiceLine{line},
			Total:         line.Amount,
			Status:        store.StatusPending,
			IssuedAt:      time.Now().UTC(),
		}
		d.Invoices = append(d.Invoices, draft)
		return nil
	})
	return draft, oldPDFPath, err
}

// BuildBatchSingle creates a single-line draft invoice for one athlete in
// the given month. Thin wrapper around BuildSingleMulti for callers that
// still want the original single-month signature (e.g. legacy code paths
// and tests). If the athlete already has any invoice covering that month,
// ErrInvoiceExists is returned and nothing is mutated.
func BuildBatchSingle(s *store.Store, year int, month time.Month, athleteID, tippText string) (store.Invoice, error) {
	return BuildSingleMulti(s, []LineSpec{{Year: year, Month: month}}, athleteID, tippText)
}

// BuildSingleMulti creates a draft invoice for one athlete spanning N
// months — one InvoiceLine per (year, month) in lines. Consumes exactly one
// invoice number regardless of len(lines). Each line's pro-rata is computed
// independently via PeriodFor against the athlete's StartDate/EndDate.
//
// Validation (hard-block, no partial creation):
//   - lines must be non-empty.
//   - No duplicate months within the request.
//   - No requested month may already be covered by an existing invoice for
//     this athlete (returns ErrInvoiceExists).
//   - Every requested month must fall within the athlete's active range
//     (StartDate / EndDate) such that PeriodFor yields ≥1 billed day.
//
// On success returns the new draft; on validation failure nothing is mutated
// and Counter.NextInvoiceNumber is not advanced.
func BuildSingleMulti(s *store.Store, lines []LineSpec, athleteID, tippText string) (store.Invoice, error) {
	if len(lines) == 0 {
		return store.Invoice{}, errors.New("no months selected")
	}
	var out store.Invoice
	err := s.Mutate(func(d *store.Data) error {
		// Locate athlete.
		var athlete *store.Athlete
		for i := range d.Athletes {
			if d.Athletes[i].ID == athleteID {
				athlete = &d.Athletes[i]
				break
			}
		}
		if athlete == nil {
			return fmt.Errorf("athlete %s not found", athleteID)
		}

		// Duplicate-month check.
		seen := map[string]bool{}
		monthKeys := make([]string, 0, len(lines))
		for _, ls := range lines {
			k := FormatMonth(ls.Year, ls.Month)
			if seen[k] {
				return fmt.Errorf("duplicate month %s in request", k)
			}
			seen[k] = true
			monthKeys = append(monthKeys, k)
		}

		// Conflict check across all existing invoices for this athlete.
		for _, inv := range d.Invoices {
			if inv.AthleteID != athleteID {
				continue
			}
			for _, k := range monthKeys {
				if inv.CoversMonth(k) {
					return ErrInvoiceExists
				}
			}
		}

		// Per-line activity + pro-rata.
		built := make([]store.InvoiceLine, 0, len(lines))
		var total store.Money
		for _, ls := range lines {
			if !athlete.Active(ls.Year, ls.Month) {
				return fmt.Errorf("athlete not active in %s", FormatMonth(ls.Year, ls.Month))
			}
			p := PeriodFor(*athlete, ls.Year, ls.Month)
			if p.Days <= 0 {
				return fmt.Errorf("athlete not active in %s", FormatMonth(ls.Year, ls.Month))
			}
			amount := AmountFor(*athlete, p)
			line := store.InvoiceLine{
				Month:       FormatMonth(ls.Year, ls.Month),
				PeriodFrom:  p.From,
				PeriodTo:    p.To,
				Description: fmt.Sprintf("Coaching %s %d", monthDE(ls.Month), ls.Year),
				Amount:      amount,
				ProRata:     p.ProRata,
			}
			built = append(built, line)
			total += amount
		}

		// Allocate a number. DisplayNumber uses the first line's period —
		// FR-I-03 ties the format to the period; for multi-month we pick the
		// first line as the canonical "period of the invoice".
		first := lines[0]
		num := d.Counter.NextInvoiceNumber
		out = store.Invoice{
			Number:        num,
			DisplayNumber: FormatDisplayNumber(d.Preferences.NumberingFormat, first.Year, first.Month, num, *athlete, d.Coach),
			AthleteID:     athleteID,
			Tipp:          tippText,
			Lines:         built,
			Total:         total,
			Status:        store.StatusPending,
			IssuedAt:      time.Now().UTC(),
		}
		d.Counter.NextInvoiceNumber++
		d.Invoices = append(d.Invoices, out)
		return nil
	})
	return out, err
}

// ConflictingMonths returns, for each month in months that is already
// covered by an existing invoice for athleteID, a Conflict describing the
// covering invoice. Months without a conflict are absent from the result.
// The order of the result matches the order of months on input.
func ConflictingMonths(d store.Data, athleteID string, months []string) []Conflict {
	out := make([]Conflict, 0, len(months))
	for _, m := range months {
		for _, inv := range d.Invoices {
			if inv.AthleteID != athleteID {
				continue
			}
			if inv.CoversMonth(m) {
				out = append(out, Conflict{
					Month:         m,
					InvoiceNumber: inv.Number,
					DisplayNumber: inv.DisplayNumber,
					Status:        inv.Status,
				})
				break
			}
		}
	}
	return out
}

// InvoicedMonths returns the set of months ("YYYY-MM") that have any
// existing invoice for athleteID. Used to render the "already invoiced"
// summary on the single-invoice form.
func InvoicedMonths(d store.Data, athleteID string) map[string]store.Invoice {
	out := map[string]store.Invoice{}
	for _, inv := range d.Invoices {
		if inv.AthleteID != athleteID {
			continue
		}
		for _, l := range inv.Lines {
			out[l.Month] = inv
		}
	}
	return out
}

// SetStatus updates an invoice's status (e.g. issued ↔ paid).
func SetStatus(s *store.Store, number int, status store.InvoiceStatus) error {
	return s.Mutate(func(d *store.Data) error {
		for i := range d.Invoices {
			if d.Invoices[i].Number == number {
				d.Invoices[i].Status = status
				return nil
			}
		}
		return fmt.Errorf("invoice %d not found", number)
	})
}

// Finalize marks an invoice as issued and records the PDF path.
func Finalize(s *store.Store, number int, pdfPath string) error {
	return s.Mutate(func(d *store.Data) error {
		for i := range d.Invoices {
			if d.Invoices[i].Number == number {
				d.Invoices[i].Status = store.StatusIssued
				d.Invoices[i].PDFPath = pdfPath
				return nil
			}
		}
		return fmt.Errorf("invoice %d not found", number)
	})
}

func upsertTipp(d *store.Data, monthKey string, t TippInput) {
	for i := range d.Tipps {
		if d.Tipps[i].Month == monthKey {
			d.Tipps[i].Text = t.Default
			d.Tipps[i].PerAthlete = cloneStrMap(t.PerAthlete)
			return
		}
	}
	d.Tipps = append(d.Tipps, store.MonthlyTipp{
		Month:      monthKey,
		Text:       t.Default,
		PerAthlete: cloneStrMap(t.PerAthlete),
	})
}

func cloneStrMap(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		if v != "" {
			out[k] = v
		}
	}
	return out
}

func monthDE(m time.Month) string {
	names := []string{
		"", "Januar", "Februar", "März", "April", "Mai", "Juni",
		"Juli", "August", "September", "Oktober", "November", "Dezember",
	}
	return names[m]
}

// MonthDE is exported for templates / PDF.
func MonthDE(m time.Month) string { return monthDE(m) }
