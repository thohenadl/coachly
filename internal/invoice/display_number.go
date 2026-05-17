package invoice

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"coachly/internal/store"
)

// FormatDisplayNumber renders the human-readable invoice identifier from
// Preferences.NumberingFormat. An empty format falls back to the legacy
// "{YYYY}{NNN}" so v1 stores keep their behaviour exactly.
//
// Supported tokens (case-sensitive, brace-delimited):
//
//	{YYYY}      — 4-digit year of the invoice period
//	{YY}        — 2-digit year
//	{MM}        — 2-digit month
//	{NNN}       — running counter, 3-digit zero-padded (n mod 1000)
//	{NNNN}      — running counter, 4-digit zero-padded (n mod 10000)
//	{N}         — running counter, no padding (n mod 1_000_000)
//	{firstname} — athlete first name, ASCII-sanitised
//	{lastname}  — athlete last name, ASCII-sanitised
//	{initials}  — coach initials from First + Last name (e.g. "TH")
//
// The running-counter tokens deliberately strip the year prefix that
// Counter.NextInvoiceNumber bakes in (year*1000 + seq), so a format like
// "{YYYY}{NNN}" reproduces the legacy "2026010" exactly.
func FormatDisplayNumber(format string, year int, month time.Month, number int, athlete store.Athlete, coach store.Coach) string {
	if format == "" {
		format = store.DefaultNumberingFormat
	}
	seq := number % 1000
	if strings.Contains(format, "{NNNN}") {
		seq = number % 10000
	}
	bigSeq := number % 1_000_000

	replacements := []struct{ k, v string }{
		{"{YYYY}", fmt.Sprintf("%04d", year)},
		{"{YY}", fmt.Sprintf("%02d", year%100)},
		{"{MM}", fmt.Sprintf("%02d", int(month))},
		{"{NNNN}", fmt.Sprintf("%04d", number%10000)},
		{"{NNN}", fmt.Sprintf("%03d", seq)},
		{"{N}", fmt.Sprintf("%d", bigSeq)},
		{"{firstname}", numberSanitize(athlete.FirstName)},
		{"{lastname}", numberSanitize(athlete.LastName)},
		{"{initials}", coachInitials(coach)},
	}

	out := format
	for _, r := range replacements {
		out = strings.ReplaceAll(out, r.k, r.v)
	}
	return out
}

// numberSanitize keeps letters and digits, drops everything else. We avoid
// underscores/hyphens here so the user can put their own separators inside
// the format template without doubling them up.
func numberSanitize(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func coachInitials(c store.Coach) string {
	first := firstLetter(c.FirstName)
	last := firstLetter(c.LastName)
	return strings.ToUpper(first + last)
}

func firstLetter(s string) string {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return string(r)
		}
	}
	return ""
}
