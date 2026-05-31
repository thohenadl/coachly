package web

import (
	"testing"

	"coachly/internal/store"
)

func TestFormatInvoicePeriod(t *testing.T) {
	cases := []struct {
		name   string
		months []string
		want   string
	}{
		{"single month", []string{"2026-05"}, "Mai 2026"},
		{"contiguous same year", []string{"2026-05", "2026-06", "2026-07"}, "Mai – Juli 2026"},
		{"gap same year", []string{"2026-05", "2026-07"}, "Mai, Jul 2026"},
		{"contiguous cross year", []string{"2026-11", "2026-12", "2027-01"}, "Nov 2026 – Jan 2027"},
		{"gap cross year", []string{"2026-11", "2027-02"}, "Nov 2026, Feb 2027"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			lines := make([]store.InvoiceLine, 0, len(tc.months))
			for _, m := range tc.months {
				lines = append(lines, store.InvoiceLine{Month: m})
			}
			got := formatInvoicePeriod(lines)
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
