package web

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"

	"coachly/internal/i18n"
	"coachly/internal/invoice"
	"coachly/internal/store"
)

func (s *Server) handleInvoiceSingleForm(w http.ResponseWriter, r *http.Request) {
	month := r.URL.Query().Get("month")
	if month == "" {
		month = time.Now().Format("2006-01")
	}
	s.renderSingleInvoiceForm(w, month, "", "", "")
}

func (s *Server) renderSingleInvoiceForm(w http.ResponseWriter, month, selectedAthlete, tipp, errMsg string) {
	d := s.Store.Snapshot()
	y, m := parseMonthInput(month)

	opts := make([]athleteOption, 0)
	for _, a := range d.Athletes {
		if !a.Active(y, m) {
			continue
		}
		opts = append(opts, athleteOption{
			ID:   a.ID,
			Name: strings.TrimSpace(a.FirstName + " " + a.LastName),
		})
	}

	currentYear := time.Now().Year()
	startYear := currentYear - 5
	if y < startYear {
		startYear = y
	}
	endYear := currentYear + 1
	if y > endYear {
		endYear = y
	}
	years := make([]int, 0, endYear-startYear+1)
	for yr := startYear; yr <= endYear; yr++ {
		years = append(years, yr)
	}
	months := make([]monthOption, 0, 12)
	for i := time.Month(1); i <= 12; i++ {
		months = append(months, monthOption{Value: fmt.Sprintf("%02d", int(i)), Label: invoice.MonthDE(i)})
	}

	v := s.chrome("invoices", i18n.T("invoices.single.title"), "")
	v["SelectedYear"] = y
	v["SelectedMonth"] = fmt.Sprintf("%02d", int(m))
	v["YearOptions"] = years
	v["MonthOptions"] = months
	v["Athletes"] = opts
	v["SelectedAthlete"] = selectedAthlete
	v["Tipp"] = tipp
	v["Error"] = errMsg
	s.renderPage(w, "invoices_new_single.html", v)
}

func (s *Server) handleInvoiceSinglePreview(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	yr := strings.TrimSpace(r.FormValue("year"))
	mn := strings.TrimSpace(r.FormValue("month_num"))
	if len(mn) == 1 {
		mn = "0" + mn
	}
	month := r.FormValue("month")
	if month == "" && yr != "" && mn != "" {
		month = yr + "-" + mn
	}
	if month == "" {
		http.Error(w, "month required", 400)
		return
	}
	y, m := parseMonthInput(month)

	athleteID := strings.TrimSpace(r.FormValue("athlete_id"))
	tipp := r.FormValue("tipp")

	if athleteID == "" {
		s.renderSingleInvoiceForm(w, month, "", tipp, i18n.T("invoices.single.no_active"))
		return
	}

	// "recreate" flag (from the per-athlete conflict dialog) means: delete
	// the existing editable invoice and reuse its number.
	var draft store.Invoice
	var err error
	if r.FormValue("mode") == "recreate" {
		var oldPDF string
		draft, oldPDF, err = invoice.RecreateSingleSameNumber(s.Store, y, m, athleteID, tipp)
		if err == nil && oldPDF != "" {
			_ = os.Remove(oldPDF)
		}
	} else {
		draft, err = invoice.BuildBatchSingle(s.Store, y, m, athleteID, tipp)
	}
	if err != nil {
		if errors.Is(err, invoice.ErrInvoiceExists) {
			// Existing invoice — was it protected?
			protected := false
			d := s.Store.Snapshot()
			monthKey := invoice.FormatMonth(y, m)
			for _, inv := range d.Invoices {
				if inv.Month == monthKey && inv.AthleteID == athleteID && !invoice.IsEditable(inv.Status) {
					protected = true
					break
				}
			}
			s.renderSingleConflict(w, month, athleteID, tipp, protected)
			return
		}
		http.Error(w, err.Error(), 500)
		return
	}

	batchID := uuid.NewString()
	s.mu.Lock()
	s.batches[batchID] = &Batch{Drafts: []store.Invoice{draft}, CreatedAt: time.Now()}
	s.mu.Unlock()

	d := s.Store.Snapshot()
	var ath store.Athlete
	for _, a := range d.Athletes {
		if a.ID == athleteID {
			ath = a
			break
		}
	}

	pv := []previewDraft{{
		Number:  draft.Number,
		Athlete: ath.FirstName + " " + ath.LastName,
		Period:  fmt.Sprintf("%s – %s", draft.PeriodFrom.Format("02.01.2006"), draft.PeriodTo.Format("02.01.2006")),
		Amount:  invoice.FormatEUR(draft.Amount),
		Tipp:    draft.Tipp,
	}}

	v := s.chrome("invoices", i18n.T("invoices.preview"), i18n.T("invoices.preview_desc"))
	v["Drafts"] = pv
	v["BatchID"] = batchID
	s.renderPage(w, "invoices_preview.html", v)
}

func (s *Server) renderSingleConflict(w http.ResponseWriter, month, athleteID, tipp string, protected bool) {
	d := s.Store.Snapshot()
	var name string
	for _, a := range d.Athletes {
		if a.ID == athleteID {
			name = strings.TrimSpace(a.FirstName + " " + a.LastName)
			break
		}
	}
	v := s.chrome("invoices", i18n.T("invoices.conflict.title"), "")
	v["Month"] = month
	v["AthleteID"] = athleteID
	v["AthleteName"] = name
	v["Tipp"] = tipp
	v["Protected"] = protected
	s.renderPage(w, "invoices_conflict_single.html", v)
}
