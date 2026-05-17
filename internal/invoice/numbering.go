package invoice

import (
	"fmt"
	"time"

	"coachly/internal/store"
)

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
			inv := store.Invoice{
				Number:      d.Counter.NextInvoiceNumber,
				AthleteID:   a.ID,
				Month:       monthKey,
				Tipp:        effectiveTipp,
				Description: desc,
				PeriodFrom:  p.From,
				PeriodTo:    p.To,
				Amount:      AmountFor(a, p),
				ProRata:     p.ProRata,
				Status:      store.StatusPending,
				IssuedAt:    now,
			}
			d.Counter.NextInvoiceNumber++
			d.Invoices = append(d.Invoices, inv)
			out = append(out, inv)
		}
		return nil
	})
	return out, err
}

// DeleteForMonth removes every invoice belonging to monthKey and returns the
// PDF paths that the caller is responsible for unlinking from disk.
// Note: Counter.NextInvoiceNumber is *not* rewound — fresh recreation will use
// the next available numbers, which keeps the sequence monotonic.
func DeleteForMonth(s *store.Store, monthKey string) ([]string, error) {
	var removedPDFs []string
	err := s.Mutate(func(d *store.Data) error {
		kept := d.Invoices[:0]
		for _, inv := range d.Invoices {
			if inv.Month == monthKey {
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
	return removedPDFs, err
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
