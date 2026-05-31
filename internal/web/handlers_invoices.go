package web

import (
	"fmt"
	"net/http"
	"os"
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
	Number      int    // canonical id used in URLs
	Display     string // human-readable identifier shown in the table
	Athlete     string
	Period      string
	Amount      string
	Status      string
	StatusClass string
	StatusLabel string
	PDFPath     string
	CanSend     bool
}

// invoiceFilter is the parsed URL state of /invoices, surfaced to the
// template so the form re-renders the user's selection.
type invoiceFilter struct {
	Status  string
	Athlete string
	Month   string
	Query   string
}

func (s *Server) handleInvoices(w http.ResponseWriter, r *http.Request) {
	d := s.Store.Snapshot()
	athleteByID := map[string]store.Athlete{}
	for _, a := range d.Athletes {
		athleteByID[a.ID] = a
	}
	sorted := append([]store.Invoice(nil), d.Invoices...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Number > sorted[j].Number })

	filter := invoiceFilter{
		Status:  strings.TrimSpace(r.URL.Query().Get("status")),
		Athlete: strings.TrimSpace(r.URL.Query().Get("athlete")),
		Month:   strings.TrimSpace(r.URL.Query().Get("month")),
		Query:   strings.TrimSpace(r.URL.Query().Get("q")),
	}
	qLower := strings.ToLower(filter.Query)

	monthsSet := map[string]struct{}{}
	rows := make([]invoiceRow, 0, len(sorted))
	for _, inv := range sorted {
		for _, l := range inv.Lines {
			monthsSet[l.Month] = struct{}{}
		}
		if !matchInvoiceFilter(inv, athleteByID[inv.AthleteID], filter, qLower) {
			continue
		}
		a := athleteByID[inv.AthleteID]
		cls, lbl := statusBadge(inv.Status)
		canSend := d.SMTP.Enabled &&
			inv.Status == store.StatusIssued &&
			inv.PDFPath != "" &&
			strings.TrimSpace(a.Email) != ""
		rows = append(rows, invoiceRow{
			Number:      inv.Number,
			Display:     displayNumberFallback(inv),
			Athlete:     a.FirstName + " " + a.LastName,
			Period:      formatInvoicePeriod(inv.Lines),
			Amount:      invoice.FormatEUR(inv.Total),
			Status:      string(inv.Status),
			StatusClass: cls,
			StatusLabel: lbl,
			PDFPath:     inv.PDFPath,
			CanSend:     canSend,
		})
	}

	// Athletes dropdown — sorted by name. Months dropdown — distinct values
	// from the data, newest first, since that's how invoices naturally flow.
	athleteOpts := make([]athleteOption, 0, len(d.Athletes))
	for _, a := range d.Athletes {
		athleteOpts = append(athleteOpts, athleteOption{
			ID:   a.ID,
			Name: strings.TrimSpace(a.FirstName + " " + a.LastName),
		})
	}
	sort.Slice(athleteOpts, func(i, j int) bool { return athleteOpts[i].Name < athleteOpts[j].Name })

	months := make([]string, 0, len(monthsSet))
	for m := range monthsSet {
		months = append(months, m)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(months)))

	v := s.chrome("invoices", i18n.T("invoices.title"), "")
	v["Invoices"] = rows
	v["Filter"] = filter
	v["AthleteOptions"] = athleteOpts
	v["MonthOptions"] = months
	v["HasFilter"] = filter.Status != "" || filter.Athlete != "" || filter.Month != "" || filter.Query != ""
	s.renderPage(w, "invoices.html", v)
}

// matchInvoiceFilter returns true when inv should appear under the given
// filter. The virtual status value "open" matches Issued+Sent (the set of
// invoices the coach still needs money for); empty values are wildcards.
func matchInvoiceFilter(inv store.Invoice, a store.Athlete, f invoiceFilter, qLower string) bool {
	if f.Status != "" {
		switch f.Status {
		case "open":
			if inv.Status != store.StatusIssued && inv.Status != store.StatusSent {
				return false
			}
		default:
			if string(inv.Status) != f.Status {
				return false
			}
		}
	}
	if f.Athlete != "" && inv.AthleteID != f.Athlete {
		return false
	}
	if f.Month != "" && !inv.CoversMonth(f.Month) {
		return false
	}
	if qLower != "" {
		hay := strings.ToLower(a.FirstName + " " + a.LastName + " " + inv.DisplayNumber)
		if !strings.Contains(hay, qLower) {
			return false
		}
	}
	return true
}

type athleteOption struct {
	ID       string
	Name     string
	Override string
}

type monthOption struct {
	Value string // "01" .. "12"
	Label string // "Januar" .. "Dezember"
}

func (s *Server) handleInvoicesNewForm(w http.ResponseWriter, r *http.Request) {
	month := r.URL.Query().Get("month")
	if month == "" {
		month = time.Now().Format("2006-01")
	}
	// Initial render: pull saved tipps for that month (if any).
	var defaultTipp string
	overrides := map[string]string{}
	for _, t := range s.Store.Snapshot().Tipps {
		if t.Month == month {
			defaultTipp = t.Text
			overrides = t.PerAthlete
			break
		}
	}
	s.renderNewInvoiceForm(w, month, defaultTipp, overrides, "")
}

// renderNewInvoiceForm renders invoices_new.html with the given form state.
// errMsg, when non-empty, is shown above the form.
func (s *Server) renderNewInvoiceForm(w http.ResponseWriter, month, defaultTipp string, overrides map[string]string, errMsg string) {
	d := s.Store.Snapshot()
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

	currentYear := time.Now().Year()
	startYear := currentYear - 2
	if y < startYear {
		startYear = y
	}
	endYear := currentYear + 2
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

	v := s.chrome("invoices", i18n.T("invoices.create"), "")
	v["Month"] = month
	v["SelectedYear"] = y
	v["SelectedMonth"] = fmt.Sprintf("%02d", int(m))
	v["YearOptions"] = years
	v["MonthOptions"] = months
	v["TippDefault"] = defaultTipp
	v["Athletes"] = opts
	v["PlaceholderOverride"] = i18n.T("invoices.tipp_override")
	v["Error"] = errMsg
	s.renderPage(w, "invoices_new.html", v)
}

// previewDraft is a flattened view of a draft invoice for the carousel.
type previewDraft struct {
	Number  int
	Display string
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
		yr := strings.TrimSpace(r.FormValue("year"))
		mn := strings.TrimSpace(r.FormValue("month_num"))
		if yr != "" && mn != "" {
			if len(mn) == 1 {
				mn = "0" + mn
			}
			month = yr + "-" + mn
		}
	}
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

	// Detect conflict: invoices already exist for this month. Unless the user
	// has already picked a mode (append / overwrite), surface the choice.
	// "Exists" here includes any invoice whose lines cover the target month —
	// both pure single-month invoices and multi-month bundles.
	mode := r.FormValue("mode")
	if mode == "" {
		monthKey := invoice.FormatMonth(y, m)
		exists := 0
		for _, inv := range s.Store.Snapshot().Invoices {
			if inv.CoversMonth(monthKey) {
				exists++
			}
		}
		if exists > 0 {
			s.renderInvoiceConflict(w, month, exists, tipp)
			return
		}
	}

	var drafts []store.Invoice
	var skippedProtected []int
	switch mode {
	case "overwrite":
		// Hard overwrite: delete editable invoices and re-create with fresh
		// numbers. Sent/Paid invoices stay untouched.
		removed, kept, err := invoice.DeleteForMonth(s.Store, invoice.FormatMonth(y, m), true)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		for _, p := range removed {
			_ = os.Remove(p)
		}
		skippedProtected = kept
		drafts, err = invoice.BuildBatch(s.Store, y, m, tipp)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
	case "overwrite_keep_numbers":
		var removed []string
		var err error
		drafts, removed, skippedProtected, err = invoice.RebuildBatchSameNumbers(s.Store, y, m, tipp)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		for _, p := range removed {
			_ = os.Remove(p)
		}
	default:
		var err error
		drafts, err = invoice.BuildBatch(s.Store, y, m, tipp)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
	}

	if len(drafts) == 0 {
		if mode == "overwrite" || mode == "overwrite_keep_numbers" {
			if len(skippedProtected) > 0 {
				s.renderNewInvoiceForm(w, month, tipp.Default, tipp.PerAthlete, i18n.T("invoices.err_all_protected"))
				return
			}
		}
		activeCount := 0
		for _, a := range s.Store.Snapshot().Athletes {
			if a.Active(y, m) {
				activeCount++
			}
		}
		errKey := "invoices.err_no_active"
		if activeCount > 0 {
			errKey = "invoices.err_all_done"
		}
		s.renderNewInvoiceForm(w, month, tipp.Default, tipp.PerAthlete, i18n.T(errKey))
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
			Display: displayNumberFallback(inv),
			Athlete: a.FirstName + " " + a.LastName,
			Period:  formatInvoicePeriod(inv.Lines),
			Amount:  invoice.FormatEUR(inv.Total),
			Tipp:    inv.Tipp,
		})
	}

	v := s.chrome("invoices", i18n.T("invoices.preview"), i18n.T("invoices.preview_desc"))
	v["Drafts"] = pv
	v["BatchID"] = batchID
	v["SkippedProtected"] = skippedProtected
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

	outDir := s.invoicesDirFor(d)
	for _, inv := range batch.Drafts {
		a := athleteByID[inv.AthleteID]
		pdfPath, err := pdf.Render(outDir, d.Coach, d.Finanzamt, a, inv)
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

// conflictTipp is a flattened per-athlete override for the hidden form fields.
type conflictTipp struct {
	AthleteID string
	Text      string
}

func (s *Server) renderInvoiceConflict(w http.ResponseWriter, month string, count int, tipp invoice.TippInput) {
	overrides := make([]conflictTipp, 0, len(tipp.PerAthlete))
	for id, text := range tipp.PerAthlete {
		overrides = append(overrides, conflictTipp{AthleteID: id, Text: text})
	}
	v := s.chrome("invoices", i18n.T("invoices.conflict.title"), "")
	v["Month"] = month
	v["Count"] = count
	v["TippDefault"] = tipp.Default
	v["TippOverrides"] = overrides
	s.renderPage(w, "invoices_conflict.html", v)
}

func (s *Server) handleInvoiceStatus(w http.ResponseWriter, r *http.Request) {
	numStr := chi.URLParam(r, "number")
	n, err := strconv.Atoi(numStr)
	if err != nil {
		http.Error(w, "bad number", 400)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	var next store.InvoiceStatus
	switch r.FormValue("status") {
	case "paid":
		next = store.StatusPaid
	case "sent":
		next = store.StatusSent
	case "issued":
		next = store.StatusIssued
	default:
		http.Error(w, "bad status", 400)
		return
	}
	if err := invoice.SetStatus(s.Store, n, next); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/invoices", http.StatusSeeOther)
}

func (s *Server) handleInvoiceDelete(w http.ResponseWriter, r *http.Request) {
	numStr := chi.URLParam(r, "number")
	n, err := strconv.Atoi(numStr)
	if err != nil {
		http.Error(w, "bad number", 400)
		return
	}
	var pdfPath string
	err = s.Store.Mutate(func(d *store.Data) error {
		for i, inv := range d.Invoices {
			if inv.Number != n {
				continue
			}
			if !invoice.IsEditable(inv.Status) {
				return errProtected
			}
			pdfPath = inv.PDFPath
			d.Invoices = append(d.Invoices[:i], d.Invoices[i+1:]...)
			return nil
		}
		return fmt.Errorf("invoice %d not found", n)
	})
	if err == errProtected {
		http.Error(w, i18n.T("invoices.err_protected"), http.StatusForbidden)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if pdfPath != "" {
		_ = os.Remove(pdfPath)
	}
	http.Redirect(w, r, "/invoices", http.StatusSeeOther)
}

// errProtected is a sentinel used inside Store.Mutate callbacks to indicate
// the caller tried to modify a Sent/Paid invoice. Surfaced as HTTP 403.
var errProtected = fmt.Errorf("invoice is protected (sent or paid)")

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

// displayNumberFallback returns inv.DisplayNumber, or a stringified integer
// id when an invoice hasn't been re-rendered yet. The migration backfills
// DisplayNumber for v1 stores, but we still guard against zero values so the
// UI never shows an empty cell.
func displayNumberFallback(inv store.Invoice) string {
	if inv.DisplayNumber != "" {
		return inv.DisplayNumber
	}
	return strconv.Itoa(inv.Number)
}
