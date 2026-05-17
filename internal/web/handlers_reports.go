package web

import (
	"net/http"
	"time"

	"coachly/internal/i18n"
	"coachly/internal/invoice"
	"coachly/internal/store"
)

type reportTile struct {
	Label string
	Value string
	Sub   string
}

func (s *Server) handleReports(w http.ResponseWriter, r *http.Request) {
	d := s.Store.Snapshot()
	now := time.Now()
	monthKey := invoice.FormatMonth(now.Year(), now.Month())

	var monthSum, total, yearSum, ytdSum store.Money
	for _, inv := range d.Invoices {
		total += inv.Amount
		if inv.Month == monthKey {
			monthSum += inv.Amount
		}
		if inv.IssuedAt.Year() == now.Year() {
			yearSum += inv.Amount
			if !inv.IssuedAt.After(now) {
				ytdSum += inv.Amount
			}
		}
	}

	v := s.chrome("reports", i18n.T("reports.title"), "")
	v["Tiles"] = []reportTile{
		{i18n.T("reports.month_revenue"), invoice.FormatEUR(monthSum), invoice.MonthDE(now.Month())},
		{i18n.T("reports.total_earned"), invoice.FormatEUR(total), ""},
		{i18n.T("reports.yearly"), invoice.FormatEUR(yearSum), ""},
		{i18n.T("reports.ytd"), invoice.FormatEUR(ytdSum), ""},
	}
	s.renderPage(w, "reports.html", v)
}
