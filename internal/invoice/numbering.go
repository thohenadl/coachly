package invoice

import (
	"errors"
	"fmt"
	"time"

	"coachly/internal/store"
)

// ErrInvoiceExists indicates that BuildBatchSingle was called for an athlete
// who already has an invoice (editable or protected) in the target month.
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

// BuildBatch composes draft invoices (status=pending_pdf) for all athletes
// active in the given month. Numbers are consumed atomically inside Mutate.
// PDF generation runs *after* this commits, then the caller calls Finalize.
func BuildBatch(s *store.Store, year int, month time.Month, tipp TippInput) ([]store.Invoice, error) {
	var out []store.Invoice
	err := s.Mutate(func(d *store.Data) error {
		monthKey := FormatMonth(year, month)
		upsertTipp(d, monthKey, tipp)

		invoiced := map[string]bool{}
		for _, inv := range d.Invoices {
			if inv.Month == monthKey {
				invoiced[inv.AthleteID] = true
			}
		}

		now := time.Now().UTC()
		for _, a := range d.Athletes {
			if invoiced[a.ID] || !a.Active(year, month) {
				continue
			}
			p := PeriodFor(a, year, month)
			desc := fmt.Sprintf("Coaching %s", monthDE(month))
			effectiveTipp := tipp.Default
			if v, ok := tipp.PerAthlete[a.ID]; ok && v != "" {
				effectiveTipp = v
			}
			num := d.Counter.NextInvoiceNumber
			inv := store.Invoice{
				Number:        num,
				DisplayNumber: FormatDisplayNumber(d.Preferences.NumberingFormat, year, month, num, a, d.Coach),
				AthleteID:     a.ID,
				Month:         monthKey,
				Tipp:          effectiveTipp,
				Description:   desc,
				PeriodFrom:    p.From,
				PeriodTo:      p.To,
				Amount:        AmountFor(a, p),
				ProRata:       p.ProRata,
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

// DeleteForMonth removes invoices belonging to monthKey and returns the PDF
// paths that the caller is responsible for unlinking from disk. When
// skipProtected is true, invoices whose status is Sent or Paid are left
// untouched and their numbers are returned in keptProtected so the caller can
// surface them in the UI.
//
// Counter.NextInvoiceNumber is *not* rewound — re-creation with fresh numbers
// keeps the sequence monotonic. Re-creation with the same numbers happens via
// RebuildBatchSameNumbers, which records the deleted invoices' numbers and
// reuses them without touching the counter.
func DeleteForMonth(s *store.Store, monthKey string, skipProtected bool) (removedPDFs []string, keptProtected []int, err error) {
	err = s.Mutate(func(d *store.Data) error {
		kept := d.Invoices[:0]
		for _, inv := range d.Invoices {
			if inv.Month == monthKey {
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

// RebuildBatchSameNumbers deletes editable invoices for the month and rebuilds
// drafts under the *same* invoice numbers. Protected invoices (Sent/Paid) are
// left untouched; their numbers come back in skippedProtected.
//
// New athletes (active in the month but never invoiced) are also added with
// fresh numbers from Counter.NextInvoiceNumber, so calling this for a month
// where coverage has grown still works.
//
// Caller is responsible for unlinking the returned PDF paths from disk
// *before* the new drafts get finalised (otherwise the new PDFs would land on
// top of the old paths only if filenames collide — which they would, since
// FR-P-09 derives the filename from athlete+month+number).
func RebuildBatchSameNumbers(s *store.Store, year int, month time.Month, tipp TippInput) (drafts []store.Invoice, removedPDFs []string, skippedProtected []int, err error) {
	monthKey := FormatMonth(year, month)
	err = s.Mutate(func(d *store.Data) error {
		upsertTipp(d, monthKey, tipp)

		// Capture, per athlete, the existing number to reuse and whether
		// they are protected (so we leave them out of the rebuild entirely).
		reuseNumber := map[string]int{}
		protectedAthletes := map[string]bool{}
		kept := d.Invoices[:0]
		for _, inv := range d.Invoices {
			if inv.Month == monthKey {
				if IsEditable(inv.Status) {
					reuseNumber[inv.AthleteID] = inv.Number
					if inv.PDFPath != "" {
						removedPDFs = append(removedPDFs, inv.PDFPath)
					}
					continue
				}
				protectedAthletes[inv.AthleteID] = true
				skippedProtected = append(skippedProtected, inv.Number)
			}
			kept = append(kept, inv)
		}
		d.Invoices = kept

		now := time.Now().UTC()
		for _, a := range d.Athletes {
			if protectedAthletes[a.ID] || !a.Active(year, month) {
				continue
			}
			p := PeriodFor(a, year, month)
			desc := fmt.Sprintf("Coaching %s", monthDE(month))
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
				Month:         monthKey,
				Tipp:          effectiveTipp,
				Description:   desc,
				PeriodFrom:    p.From,
				PeriodTo:      p.To,
				Amount:        AmountFor(a, p),
				ProRata:       p.ProRata,
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

// RecreateSingleSameNumber deletes the existing editable invoice for
// (athleteID, month) and rebuilds it under the same number with fresh
// pro-rata / tipp values. The previous PDF path is returned so the caller
// can unlink it. Returns ErrInvoiceExists with no mutation if the existing
// invoice is Sent or Paid.
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
		var foundIdx = -1
		for i, inv := range d.Invoices {
			if inv.Month == monthKey && inv.AthleteID == athleteID {
				if !IsEditable(inv.Status) {
					return ErrInvoiceExists
				}
				oldNumber = inv.Number
				oldPDFPath = inv.PDFPath
				foundIdx = i
				break
			}
		}
		if foundIdx == -1 {
			return fmt.Errorf("invoice for athlete %s in %s not found", athleteID, monthKey)
		}
		d.Invoices = append(d.Invoices[:foundIdx], d.Invoices[foundIdx+1:]...)

		p := PeriodFor(*athlete, year, month)
		desc := fmt.Sprintf("Coaching %s", monthDE(month))
		draft = store.Invoice{
			Number:        oldNumber,
			DisplayNumber: FormatDisplayNumber(d.Preferences.NumberingFormat, year, month, oldNumber, *athlete, d.Coach),
			AthleteID:     athleteID,
			Month:         monthKey,
			Tipp:          tippText,
			Description:   desc,
			PeriodFrom:    p.From,
			PeriodTo:      p.To,
			Amount:        AmountFor(*athlete, p),
			ProRata:       p.ProRata,
			Status:        store.StatusPending,
			IssuedAt:      time.Now().UTC(),
		}
		d.Invoices = append(d.Invoices, draft)
		return nil
	})
	return draft, oldPDFPath, err
}

// BuildBatchSingle creates a single draft invoice for one athlete in the
// given month. It honours pro-rata and consumes one invoice number. If the
// athlete already has *any* invoice for that month (editable or protected),
// ErrInvoiceExists is returned and nothing is mutated.
func BuildBatchSingle(s *store.Store, year int, month time.Month, athleteID, tippText string) (store.Invoice, error) {
	var out store.Invoice
	err := s.Mutate(func(d *store.Data) error {
		monthKey := FormatMonth(year, month)

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
		if !athlete.Active(year, month) {
			return fmt.Errorf("athlete %s not active in %s", athleteID, monthKey)
		}
		for _, inv := range d.Invoices {
			if inv.Month == monthKey && inv.AthleteID == athleteID {
				return ErrInvoiceExists
			}
		}

		p := PeriodFor(*athlete, year, month)
		desc := fmt.Sprintf("Coaching %s", monthDE(month))
		num := d.Counter.NextInvoiceNumber
		out = store.Invoice{
			Number:        num,
			DisplayNumber: FormatDisplayNumber(d.Preferences.NumberingFormat, year, month, num, *athlete, d.Coach),
			AthleteID:     athleteID,
			Month:         monthKey,
			Tipp:          tippText,
			Description:   desc,
			PeriodFrom:    p.From,
			PeriodTo:      p.To,
			Amount:        AmountFor(*athlete, p),
			ProRata:       p.ProRata,
			Status:        store.StatusPending,
			IssuedAt:      time.Now().UTC(),
		}
		d.Counter.NextInvoiceNumber++
		d.Invoices = append(d.Invoices, out)
		return nil
	})
	return out, err
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
