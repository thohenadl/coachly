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

// PDFFilename returns "max_05_2026_2026042.pdf" style names (FR-P-09).
// The invoice number suffix guarantees uniqueness when multiple athletes
// share a first name.
func PDFFilename(firstName string, year int, month, number int) string {
	return fmt.Sprintf("%s_%02d_%04d_%d.pdf", sanitize(firstName), month, year, number)
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
		return "athlete"
	}
	return string(out)
}
