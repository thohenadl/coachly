package invoice

import (
	"testing"
	"time"

	"coachly/internal/store"
)

func TestPeriodForFullMonth(t *testing.T) {
	a := store.Athlete{
		StartDate:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		MonthlyFee: 20000, // 200,00 €
	}
	p := PeriodFor(a, 2026, time.May)
	if p.ProRata {
		t.Fatal("expected full month")
	}
	if AmountFor(a, p) != 20000 {
		t.Fatalf("got %d", AmountFor(a, p))
	}
}

func TestPeriodForMidMonthStart(t *testing.T) {
	a := store.Athlete{
		// Start on May 16 → 16 of 31 billed days
		StartDate:  time.Date(2026, 5, 16, 0, 0, 0, 0, time.UTC),
		MonthlyFee: 31000, // 310,00 €
	}
	p := PeriodFor(a, 2026, time.May)
	if !p.ProRata {
		t.Fatal("expected pro-rata")
	}
	if p.Days != 16 || p.OfDays != 31 {
		t.Fatalf("got days=%d ofDays=%d", p.Days, p.OfDays)
	}
	// 31000 * 16 / 31 = 16000.0
	if got := AmountFor(a, p); got != 16000 {
		t.Fatalf("amount %d", got)
	}
}

func TestPeriodForEndedMidMonth(t *testing.T) {
	end := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	a := store.Athlete{
		StartDate:  time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:    &end,
		MonthlyFee: 31000,
	}
	p := PeriodFor(a, 2026, time.May)
	if !p.ProRata || p.Days != 10 {
		t.Fatalf("got %+v", p)
	}
}

func TestRoundingHalfUp(t *testing.T) {
	a := store.Athlete{
		StartDate:  time.Date(2026, 5, 17, 0, 0, 0, 0, time.UTC),
		MonthlyFee: 10000,
	}
	p := PeriodFor(a, 2026, time.May) // 15 of 31 days
	// 10000 * 15 / 31 = 4838.71 → 4839
	if got := AmountFor(a, p); got != 4839 {
		t.Fatalf("amount %d", got)
	}
}
