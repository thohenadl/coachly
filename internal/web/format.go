package web

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"coachly/internal/invoice"
	"coachly/internal/store"
)

type ym struct {
	y int
	m time.Month
}

// formatInvoicePeriod renders an invoice's covered months as a compact
// human-readable string. Examples:
//
//	1 line, May 2026:                  "Mai 2026"
//	3 contiguous lines, May–Jul 2026:  "Mai – Juli 2026"
//	gap (May + Jul 2026):              "Mai, Jul 2026"
//	cross-year (Nov 2026 + Jan 2027):  "Nov 2026 – Jan 2027" (when contiguous)
//	cross-year non-contiguous:         "Nov 2026, Jan 2027"
//
// When all months share a year the year is shown once at the end; when they
// span multiple years it is shown per-token.
func formatInvoicePeriod(lines []store.InvoiceLine) string {
	if len(lines) == 0 {
		return ""
	}
	ymList := make([]ym, 0, len(lines))
	for _, l := range lines {
		y, mo := parseYearMonth(l.Month)
		if y == 0 {
			continue
		}
		ymList = append(ymList, ym{y, mo})
	}
	if len(ymList) == 0 {
		return ""
	}
	// Preserve form order (which equals user input order); do not sort.
	contiguous := isContiguous(ymList)
	sameYear := true
	for i := 1; i < len(ymList); i++ {
		if ymList[i].y != ymList[0].y {
			sameYear = false
			break
		}
	}

	if contiguous && len(ymList) > 1 {
		first, last := ymList[0], ymList[len(ymList)-1]
		if first.y == last.y {
			return fmt.Sprintf("%s – %s %d",
				invoice.MonthDE(first.m), invoice.MonthDE(last.m), first.y)
		}
		return fmt.Sprintf("%s %d – %s %d",
			shortMonth(first.m), first.y, shortMonth(last.m), last.y)
	}

	if len(ymList) == 1 {
		return fmt.Sprintf("%s %d", invoice.MonthDE(ymList[0].m), ymList[0].y)
	}

	parts := make([]string, 0, len(ymList))
	if sameYear {
		for _, e := range ymList {
			parts = append(parts, shortMonth(e.m))
		}
		return strings.Join(parts, ", ") + " " + strconv.Itoa(ymList[0].y)
	}
	for _, e := range ymList {
		parts = append(parts, fmt.Sprintf("%s %d", shortMonth(e.m), e.y))
	}
	return strings.Join(parts, ", ")
}

func isContiguous(list []ym) bool {
	if len(list) < 2 {
		return true
	}
	for i := 1; i < len(list); i++ {
		want := list[i-1]
		want.m++
		if want.m == 13 {
			want.m = time.January
			want.y++
		}
		if list[i] != want {
			return false
		}
	}
	return true
}

func parseYearMonth(s string) (int, time.Month) {
	if len(s) != 7 || s[4] != '-' {
		return 0, 0
	}
	y, err := strconv.Atoi(s[:4])
	if err != nil {
		return 0, 0
	}
	mInt, err := strconv.Atoi(s[5:])
	if err != nil || mInt < 1 || mInt > 12 {
		return 0, 0
	}
	return y, time.Month(mInt)
}

func shortMonth(m time.Month) string {
	names := []string{
		"", "Jan", "Feb", "Mär", "Apr", "Mai", "Jun",
		"Jul", "Aug", "Sep", "Okt", "Nov", "Dez",
	}
	if m < 1 || int(m) > 12 {
		return ""
	}
	return names[m]
}

// invoiceYears returns the sorted, deduplicated list of years that any line
// of any invoice covers. Used by the invoice-list year filter dropdown.
func invoiceYears(invoices []store.Invoice) []int {
	set := map[int]struct{}{}
	for _, inv := range invoices {
		for _, l := range inv.Lines {
			if y, _ := parseYearMonth(l.Month); y != 0 {
				set[y] = struct{}{}
			}
		}
	}
	out := make([]int, 0, len(set))
	for y := range set {
		out = append(out, y)
	}
	sort.Sort(sort.Reverse(sort.IntSlice(out)))
	return out
}
