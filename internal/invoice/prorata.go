// Package invoice contains the business rules around invoice numbering,
// pro-rata calculation and the German period strings printed on the PDF.
package invoice

import (
	"fmt"
	"time"

	"coachly/internal/store"
)

// MonthBounds returns the first and last day of a given year-month, UTC midnight.
func MonthBounds(year int, month time.Month) (time.Time, time.Time) {
	start := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, -1)
	return start, end
}

// Period describes the billed timeframe for a single invoice.
type Period struct {
	From    time.Time
	To      time.Time
	Days    int  // billed days
	OfDays  int  // total days in month
	ProRata bool // true iff Days != OfDays
}

// PeriodFor computes the period to bill for `a` in the given month.
// Honors both the athlete's StartDate (mid-month start = pro-rata)
// and EndDate (mid-month end = pro-rata).
func PeriodFor(a store.Athlete, year int, month time.Month) Period {
	monthStart, monthEnd := MonthBounds(year, month)
	from := monthStart
	if a.StartDate.After(monthStart) {
		from = a.StartDate
	}
	to := monthEnd
	if a.EndDate != nil && a.EndDate.Before(monthEnd) {
		to = *a.EndDate
	}
	billedDays := int(to.Sub(from).Hours()/24) + 1
	totalDays := int(monthEnd.Sub(monthStart).Hours()/24) + 1
	return Period{
		From:    from,
		To:      to,
		Days:    billedDays,
		OfDays:  totalDays,
		ProRata: billedDays != totalDays,
	}
}

// AmountFor returns the gross amount (cents) for `a` in the given month.
// Pro-rata: (monthlyFee * billedDays / totalDays), rounded half-up.
func AmountFor(a store.Athlete, p Period) store.Money {
	if !p.ProRata {
		return a.MonthlyFee
	}
	// half-up rounding on cents
	num := int64(a.MonthlyFee) * int64(p.Days)
	q := num / int64(p.OfDays)
	r := num % int64(p.OfDays)
	if r*2 >= int64(p.OfDays) {
		q++
	}
	return store.Money(q)
}

// FormatMonth produces "2026-05" for a given (year, month).
func FormatMonth(year int, month time.Month) string {
	return fmt.Sprintf("%04d-%02d", year, int(month))
}

// FormatPeriodDE renders the period as "01.05.2026 – 31.05.2026".
func FormatPeriodDE(p Period) string {
	return fmt.Sprintf("%s – %s",
		p.From.Format("02.01.2006"),
		p.To.Format("02.01.2006"),
	)
}
