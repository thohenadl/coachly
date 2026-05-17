package web

import (
	"fmt"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"coachly/internal/i18n"
	"coachly/internal/invoice"
	"coachly/internal/pdf"
	"coachly/internal/store"
)

type invoiceRow struct {
	Number      int
	Athlete     string
	Period      string
	Amount      string
	StatusClass string
	StatusLabel string
	PDFPath     string
}

func (s *Server) handleInvoices(w http.ResponseWriter, r *http.Request) {
	d := s.Store.Snapshot()
	athleteByID := map[string]store.Athlete{}
	for _, a := range d.Athletes {
		athleteByID[a.ID] = a
	}
	sorted := append([]store.Invoice(nil), d.Invoices...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Number > sorted[j].Number })

	rows := make([]invoiceRow, 0, len(sorted))
	for _, inv := range sorted {
		a := athleteByID[inv.AthleteID]
		cls, lbl := statusBadge(inv.Status)
		rows = append(rows, invoiceRow{
			Number:      inv.Number,
			Athlete:     a.FirstName + " " + a.LastName,
			Period:      fmt.Sprintf("%s – %s", inv.PeriodFrom.Format("02.01.2006"), inv.PeriodTo.Format("02.01.2006")),
			Amount:      invoice.FormatEUR(inv.Amount),
			StatusClass: cls,
			StatusLabel: lbl,
			PDFPath:     inv.PDFPath,
		})
	}

	v := s.chrome("invoices", i18n.T("invoices.title"), "")
	v["Invoices"] = rows
	s.renderPage(w, "invoices.html", v)
}

type athleteOption struct {
	ID       string
	Name     string
	Override string
}

func (s *Server) handleInvoicesNewForm(w http.ResponseWriter, r *http.Request) {
	d := s.Store.Snapshot()
	month := r.URL.Query().Get("month")
	if month == "" {
		month = time.Now().Format("2006-01")
	}

	var defaultTipp string
	overrides := map[string]string{}
	for _, t := range d.Tipps {
		if t.Month == month {
			defaultTipp = t.Text
			overrides = t.PerAthlete
			break
		}
	}

	y, m := parseMonthInput(month)
	opts := make([]athleteOption, 0)
	for _, a := range d.Athletes {
		if !a.Active(y, m) {
			continue
		}
		opts = append(opts, athleteOption{
			ID:       a.ID,
			Name:     strings.TrimSpace(a.FirstName + " " + a.LastName),
			Override: overrides[a.ID],
		})
	}

	v := s.chrome("invoices", i18n.T("invoices.create"), "")
	v["Month"] = month
	v["TippDefault"] = defaultTipp
	v["Athletes"] = opts
	v["PlaceholderOverride"] = i18n.T("invoices.tipp_override")
	s.renderPage(w, "invoices_new.html", v)
}

// previewDraft is a flattened view of a draft invoice for the carousel.
type previewDraft struct {
	Number  int
	Athlete string
	Period  string
	Amount  string
	Tipp    string
}

func (s *Server) handleInvoicesPreview(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	month := r.FormValue("month")
	if month == "" {
		http.Error(w, "month required", 400)
		return
	}
	y, m := parseMonthInput(month)

	tipp := invoice.TippInput{
		Default:    r.FormValue("tipp_default"),
		PerAthlete: map[string]string{},
	}
	for k, vs := range r.Form {
		if strings.HasPrefix(k, "tipp_") && k != "tipp_default" && len(vs) > 0 && vs[0] != "" {
			tipp.PerAthlete[strings.TrimPrefix(k, "tipp_")] = vs[0]
		}
	}

	drafts, err := invoice.BuildBatch(s.Store, y, m, tipp)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if len(drafts) == 0 {
		// Nothing to do — bounce back with a message.
		http.Redirect(w, r, "/invoices", http.StatusSeeOther)
		return
	}

	batchID := uuid.NewString()
	s.mu.Lock()
	s.batches[batchID] = &Batch{Drafts: drafts, CreatedAt: time.Now()}
	s.mu.Unlock()

	d := s.Store.Snapshot()
	athleteByID := map[string]store.Athlete{}
	for _, a := range d.Athletes {
		athleteByID[a.ID] = a
	}

	pv := make([]previewDraft, 0, len(drafts))
	for _, inv := range drafts {
		a := athleteByID[inv.AthleteID]
		pv = append(pv, previewDraft{
			Number:  inv.Number,
			Athlete: a.FirstName + " " + a.LastName,
			Period:  fmt.Sprintf("%s – %s", inv.PeriodFrom.Format("02.01.2006"), inv.PeriodTo.Format("02.01.2006")),
			Amount:  invoice.FormatEUR(inv.Amount),
			Tipp:    inv.Tipp,
		})
	}

	v := s.chrome("invoices", i18n.T("invoices.preview"), i18n.T("invoices.preview_desc"))
	v["Drafts"] = pv
	v["BatchID"] = batchID
	s.renderPage(w, "invoices_preview.html", v)
}

func (s *Server) handleInvoicesConfirm(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	batchID := r.FormValue("batch_id")
	s.mu.Lock()
	batch, ok := s.batches[batchID]
	if ok {
		delete(s.batches, batchID)
	}
	s.mu.Unlock()
	if !ok {
		http.Error(w, "batch expired", 400)
		return
	}

	d := s.Store.Snapshot()
	athleteByID := map[string]store.Athlete{}
	for _, a := range d.Athletes {
		athleteByID[a.ID] = a
	}

	for _, inv := range batch.Drafts {
		a := athleteByID[inv.AthleteID]
		pdfPath, err := pdf.Render(s.InvoicesDir, d.Coach, d.Finanzamt, a, inv)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		if err := invoice.Finalize(s.Store, inv.Number, pdfPath); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
	}
	http.Redirect(w, r, "/invoices", http.StatusSeeOther)
}

func (s *Server) handleInvoicePDF(w http.ResponseWriter, r *http.Request) {
	numStr := chi.URLParam(r, "number")
	n, err := strconv.Atoi(numStr)
	if err != nil {
		http.Error(w, "bad number", 400)
		return
	}
	d := s.Store.Snapshot()
	for _, inv := range d.Invoices {
		if inv.Number == n && inv.PDFPath != "" {
			http.ServeFile(w, r, inv.PDFPath)
			return
		}
	}
	http.NotFound(w, r)
}

func parseMonthInput(s string) (int, time.Month) {
	if len(s) < 7 {
		now := time.Now()
		return now.Year(), now.Month()
	}
	y, _ := strconv.Atoi(s[:4])
	m, _ := strconv.Atoi(s[5:7])
	return y, time.Month(m)
}

// unused: keeping filepath import meaningful (we'll need it once we expose
// the data dir for "open data folder" buttons).
var _ = filepath.Separator
