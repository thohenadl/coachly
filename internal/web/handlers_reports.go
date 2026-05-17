package web

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"coachly/internal/i18n"
	"coachly/internal/invoice"
	"coachly/internal/store"
)

type reportLine struct {
	Label string
	Value string
}

type reportTile struct {
	Label   string
	Sub     string
	Lines   []reportLine
	Total   string
	HasSplit bool
}

func (s *Server) handleReports(w http.ResponseWriter, r *http.Request) {
	d := s.Store.Snapshot()
	now := time.Now()
	currentYear := now.Year()
	monthKey := invoice.FormatMonth(currentYear, now.Month())
	yearPrefix := fmt.Sprintf("%04d-", currentYear)

	var month, total, ytd statusSums
	for _, inv := range d.Invoices {
		total.add(inv.Status, inv.Amount)
		if inv.Month == monthKey {
			month.add(inv.Status, inv.Amount)
		}
		if strings.HasPrefix(inv.Month, yearPrefix) {
			ytd.add(inv.Status, inv.Amount)
		}
	}

	planned := plannedYearRevenue(d.Athletes, currentYear)

	v := s.chrome("reports", i18n.T("reports.title"), "")
	v["Tiles"] = []reportTile{
		buildTile(i18n.T("reports.month_revenue"), invoice.MonthDE(now.Month()), month),
		buildTile(i18n.T("reports.total_earned"), "", total),
		{
			Label: i18n.T("reports.yearly"),
			Sub:   i18n.T("reports.yearly.sub"),
			Total: invoice.FormatEUR(planned),
		},
		buildTile(i18n.T("reports.ytd"), "", ytd),
	}
	s.renderPage(w, "reports.html", v)
}

type statusSums struct {
	Pending store.Money
	Issued  store.Money
	Sent    store.Money
	Paid    store.Money
}

func (s *statusSums) add(st store.InvoiceStatus, amt store.Money) {
	switch st {
	case store.StatusPending:
		s.Pending += amt
	case store.StatusIssued:
		s.Issued += amt
	case store.StatusSent:
		s.Sent += amt
	case store.StatusPaid:
		s.Paid += amt
	}
}

func (s statusSums) total() store.Money {
	return s.Pending + s.Issued + s.Sent + s.Paid
}

func buildTile(label, sub string, sums statusSums) reportTile {
	return reportTile{
		Label: label,
		Sub:   sub,
		Lines: []reportLine{
			{i18n.T("invoices.status.pending"), invoice.FormatEUR(sums.Pending)},
			{i18n.T("invoices.status.issued"), invoice.FormatEUR(sums.Issued)},
			{i18n.T("invoices.status.sent"), invoice.FormatEUR(sums.Sent)},
			{i18n.T("invoices.status.paid"), invoice.FormatEUR(sums.Paid)},
		},
		Total:    invoice.FormatEUR(sums.total()),
		HasSplit: true,
	}
}

func plannedYearRevenue(athletes []store.Athlete, year int) store.Money {
	var sum store.Money
	for m := time.January; m <= time.December; m++ {
		for _, a := range athletes {
			if !a.Active(year, m) {
				continue
			}
			p := invoice.PeriodFor(a, year, m)
			sum += invoice.AmountFor(a, p)
		}
	}
	return sum
}

