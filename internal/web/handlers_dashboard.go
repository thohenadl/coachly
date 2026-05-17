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
}

type recentRow struct {
	Number      int
	Athlete     string
	Date        string
	Amount      string
	StatusClass string
	StatusLabel string
}

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	d := s.Store.Snapshot()
	now := time.Now()

	var monthRev, total, paidCt, issuedCt, pendingCt int64
	monthKey := invoice.FormatMonth(now.Year(), now.Month())

	for _, inv := range d.Invoices {
		total += int64(inv.Amount)
		if inv.Month == monthKey {
			monthRev += int64(inv.Amount)
		}
		switch inv.Status {
		case store.StatusPaid:
			paidCt++
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
			Athlete:     a.FirstName + " " + a.LastName,
			Date:        inv.IssuedAt.Format("02.01.2006"),
			Amount:      invoice.FormatEUR(inv.Amount),
			StatusClass: cls,
			StatusLabel: lbl,
		})
	}

	v := s.chrome("dashboard", i18n.T("app.greeting"), i18n.T("app.subgreeting"))
	v["KPIs"] = []kpi{
		{i18n.T("dash.kpi.month_revenue"), invoice.FormatEUR(store.Money(monthRev)), invoice.MonthDE(now.Month())},
		{i18n.T("dash.kpi.outstanding"), invoice.FormatEUR(store.Money(0)), ""},
		{i18n.T("dash.kpi.paid"), invoice.FormatEUR(store.Money(0)), ""},
		{i18n.T("dash.kpi.athletes"), formatInt(len(d.Athletes)), ""},
	}
	v["RecentInvoices"] = recent
	v["Chart"] = map[string]any{
		"Paid":    paidCt,
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
