package invoice

import (
	"fmt"

	"coachly/internal/store"
)

// FormatEUR renders cents as "1.234,56 €".
func FormatEUR(m store.Money) string {
	cents := int64(m)
	neg := cents < 0
	if neg {
		cents = -cents
	}
	euros := cents / 100
	rem := cents % 100

	// thousands separator (German: ".")
	s := fmt.Sprintf("%d", euros)
	out := ""
	for i, r := range reverse(s) {
		if i > 0 && i%3 == 0 {
			out = "." + out
		}
		out = string(r) + out
	}
	if neg {
		out = "-" + out
	}
	return fmt.Sprintf("%s,%02d €", out, rem)
}

func reverse(s string) string {
	r := []rune(s)
	for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
		r[i], r[j] = r[j], r[i]
	}
	return string(r)
}

// PDFFilename returns "max_05_2026_<displayNumber>.pdf" style names (FR-P-09).
// The display-number suffix guarantees uniqueness when multiple athletes
// share a first name and follows the configured invoice-number format.
// Both firstName and displayNumber are sanitised so the result is always a
// safe filename on macOS and Windows.
func PDFFilename(firstName string, year int, month int, displayNumber string) string {
	first := sanitize(firstName)
	if first == "" {
		first = "athlete"
	}
	num := sanitize(displayNumber)
	if num == "" {
		num = "0"
	}
	return fmt.Sprintf("%s_%02d_%04d_%s.pdf", first, month, year, num)
}

func sanitize(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			out = append(out, r)
		case r == ' ', r == '-', r == '_':
			out = append(out, '_')
		}
	}
	if len(out) == 0 {
		return ""
	}
	return string(out)
}
