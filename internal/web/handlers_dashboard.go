package web

import (
	"net/http"
	"sort"
	"time"

	"coachly/internal/i18n"
	"coachly/internal/invoice"
	"coachly/internal/store"
)

type kpi struct {
	Label string
	Value string
	Sub   string
	Link  string // empty for non-clickable tiles
}

type recentRow struct {
	Number      int
	Display     string
	Athlete     string
	Date        string
	Amount      string
	StatusClass string
	StatusLabel string
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	d := s.Store.Snapshot()
	now := time.Now()

	var monthRev, monthOpen, monthPaid, total int64
	var paidCt, sentCt, issuedCt, pendingCt int64
	monthKey := invoice.FormatMonth(now.Year(), now.Month())

	// Per-month KPIs attribute the *line's* amount to its own month, so a
	// multi-month invoice contributes proportionally to each month it covers
	// instead of inflating every month with the full invoice total.
	for _, inv := range d.Invoices {
		total += int64(inv.Total)
		for _, l := range inv.Lines {
			if l.Month != monthKey {
				continue
			}
			monthRev += int64(l.Amount)
			switch inv.Status {
			case store.StatusIssued, store.StatusSent:
				monthOpen += int64(l.Amount)
			case store.StatusPaid:
				monthPaid += int64(l.Amount)
			}
		}
		switch inv.Status {
		case store.StatusPaid:
			paidCt++
		case store.StatusSent:
			sentCt++
		case store.StatusIssued:
			issuedCt++
		default:
			pendingCt++
		}
	}

	athleteByID := map[string]store.Athlete{}
	for _, a := range d.Athletes {
		athleteByID[a.ID] = a
	}

	sorted := append([]store.Invoice(nil), d.Invoices...)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].IssuedAt.After(sorted[j].IssuedAt)
	})
	if len(sorted) > 6 {
		sorted = sorted[:6]
	}
	recent := make([]recentRow, 0, len(sorted))
	for _, inv := range sorted {
		a := athleteByID[inv.AthleteID]
		cls, lbl := statusBadge(inv.Status)
		recent = append(recent, recentRow{
			Number:      inv.Number,
			Display:     displayNumberFallback(inv),
			Athlete:     a.FirstName + " " + a.LastName,
			Date:        inv.IssuedAt.Format("02.01.2006"),
			Amount:      invoice.FormatEUR(inv.Total),
			StatusClass: cls,
			StatusLabel: lbl,
		})
	}

	v := s.chrome("dashboard", i18n.T("app.greeting"), i18n.T("app.subgreeting"))
	monthLabel := invoice.MonthDE(now.Month())
	openLink := "/invoices?month=" + monthKey + "&status=open"
	paidLink := "/invoices?month=" + monthKey + "&status=paid"
	v["KPIs"] = []kpi{
		{Label: i18n.T("dash.kpi.month_revenue"), Value: invoice.FormatEUR(store.Money(monthRev)), Sub: monthLabel},
		{Label: i18n.T("dash.kpi.outstanding"), Value: invoice.FormatEUR(store.Money(monthOpen)), Sub: monthLabel, Link: openLink},
		{Label: i18n.T("dash.kpi.paid"), Value: invoice.FormatEUR(store.Money(monthPaid)), Sub: monthLabel, Link: paidLink},
		{Label: i18n.T("dash.kpi.athletes"), Value: formatInt(len(d.Athletes))},
	}
	v["RecentInvoices"] = recent
	v["Chart"] = map[string]any{
		"Paid":    paidCt,
		"Sent":    sentCt,
		"Issued":  issuedCt,
		"Pending": pendingCt,
	}
	s.renderPage(w, "dashboard.html", v)
}

func formatInt(n int) string {
	if n == 0 {
		return "0"
	}
	// small ints: stringify directly
	const digits = "0123456789"
	if n < 0 {
		return "-" + formatInt(-n)
	}
	out := ""
	for n > 0 {
		out = string(digits[n%10]) + out
		n /= 10
	}
	return out
}
