package web

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"coachly/internal/i18n"
	"coachly/internal/invoice"
	"coachly/internal/store"
)

// singleLineInput is one row of the multi-month line-builder form.
type singleLineInput struct {
	Year     int
	Month    time.Month
	Key      string // "YYYY-MM" — convenience for templates / dedup
	YearStr  string
	MonthStr string
}

// singleFormVM is the view-model for invoices_new_single.html. Each Line
// gets a per-row hint computed up front so the template doesn't need to
// re-derive anything.
type singleFormVM struct {
	SelectedAthlete string
	Tipp            string
	Lines           []singleLineRow
	YearOptions     []int
	MonthOptions    []monthOption
	Athletes        []athleteOption
	SummaryBilled   []string // already-invoiced months for the selected athlete, e.g. ["Jan 2026", "Feb 2026"]
	Error           string
}

type singleLineRow struct {
	Idx          int
	Year         int
	MonthValue   string // "01".."12"
	ConflictWith string // empty if no conflict; "#142" otherwise
	Inactive     bool
}

func (s *Server) handleInvoiceSingleForm(w http.ResponseWriter, r *http.Request) {
	month := r.URL.Query().Get("month")
	if month == "" {
		month = time.Now().Format("2006-01")
	}
	y, m := parseMonthInput(month)
	vm := singleFormVM{
		Lines: []singleLineRow{{
			Idx: 0, Year: y, MonthValue: fmt.Sprintf("%02d", int(m)),
		}},
	}
	s.renderSingleInvoiceForm(w, vm)
}

func (s *Server) renderSingleInvoiceForm(w http.ResponseWriter, vm singleFormVM) {
	d := s.Store.Snapshot()

	// Athletes dropdown is unfiltered (multi-month invoices touch multiple
	// months, so a single "is active this month" filter would be wrong; the
	// per-row hint surfaces inactive lines instead).
	athleteOpts := make([]athleteOption, 0, len(d.Athletes))
	for _, a := range d.Athletes {
		athleteOpts = append(athleteOpts, athleteOption{
			ID:   a.ID,
			Name: strings.TrimSpace(a.FirstName + " " + a.LastName),
		})
	}
	sort.Slice(athleteOpts, func(i, j int) bool { return athleteOpts[i].Name < athleteOpts[j].Name })
	vm.Athletes = athleteOpts

	// Year range: ±2 around current, expanded to fit any picked row.
	currentYear := time.Now().Year()
	startYear := currentYear - 2
	endYear := currentYear + 2
	for _, ln := range vm.Lines {
		if ln.Year != 0 && ln.Year < startYear {
			startYear = ln.Year
		}
		if ln.Year != 0 && ln.Year > endYear {
			endYear = ln.Year
		}
	}
	years := make([]int, 0, endYear-startYear+1)
	for yr := startYear; yr <= endYear; yr++ {
		years = append(years, yr)
	}
	vm.YearOptions = years

	months := make([]monthOption, 0, 12)
	for i := time.Month(1); i <= 12; i++ {
		months = append(months, monthOption{Value: fmt.Sprintf("%02d", int(i)), Label: invoice.MonthDE(i)})
	}
	vm.MonthOptions = months

	// Compute per-row hints + summary for the selected athlete.
	if vm.SelectedAthlete != "" {
		var athlete store.Athlete
		for _, a := range d.Athletes {
			if a.ID == vm.SelectedAthlete {
				athlete = a
				break
			}
		}
		invoiced := invoice.InvoicedMonths(d, vm.SelectedAthlete)
		summary := summaryFromInvoiced(invoiced)
		vm.SummaryBilled = summary

		for i := range vm.Lines {
			ln := &vm.Lines[i]
			y, ok := atoiOK(strconv.Itoa(ln.Year))
			mInt, _ := atoiOK(ln.MonthValue)
			if !ok || mInt == 0 {
				continue
			}
			key := fmt.Sprintf("%04d-%02d", y, mInt)
			if existing, has := invoiced[key]; has {
				ln.ConflictWith = "#" + displayNumberFallback(existing)
			}
			if athlete.ID != "" && !athlete.Active(y, time.Month(mInt)) {
				ln.Inactive = true
			}
		}
	}

	v := s.chrome("invoices", i18n.T("invoices.single.title"), "")
	v["VM"] = vm
	v["SelectedAthlete"] = vm.SelectedAthlete
	v["Athletes"] = vm.Athletes
	v["YearOptions"] = vm.YearOptions
	v["MonthOptions"] = vm.MonthOptions
	v["Tipp"] = vm.Tipp
	v["Lines"] = vm.Lines
	v["SummaryBilled"] = vm.SummaryBilled
	v["Error"] = vm.Error
	s.renderPage(w, "invoices_new_single.html", v)
}

// parseSingleForm reads the form state of invoices_new_single.html into a
// view-model. Used by every POST handler that re-renders the form (add row,
// remove row, preview).
func parseSingleForm(r *http.Request) singleFormVM {
	_ = r.ParseForm()
	years := r.Form["line_year"]
	months := r.Form["line_month"]
	count := len(years)
	if len(months) < count {
		count = len(months)
	}
	vm := singleFormVM{
		SelectedAthlete: strings.TrimSpace(r.FormValue("athlete_id")),
		Tipp:            r.FormValue("tipp"),
	}
	for i := 0; i < count; i++ {
		y, _ := strconv.Atoi(strings.TrimSpace(years[i]))
		mn := strings.TrimSpace(months[i])
		if len(mn) == 1 {
			mn = "0" + mn
		}
		vm.Lines = append(vm.Lines, singleLineRow{
			Idx:        i,
			Year:       y,
			MonthValue: mn,
		})
	}
	if len(vm.Lines) == 0 {
		// Fall back to a single empty row so the form always shows at least one.
		now := time.Now()
		vm.Lines = []singleLineRow{{
			Year:       now.Year(),
			MonthValue: fmt.Sprintf("%02d", int(now.Month())),
		}}
	}
	return vm
}

func (s *Server) handleInvoiceSinglePreview(w http.ResponseWriter, r *http.Request) {
	vm := parseSingleForm(r)

	// Server-side row management + form re-render. Only an explicit
	// `action=preview` (the "Vorschau" button) advances to invoice creation;
	// every other submit (athlete dropdown change, "+ weiteren Monat",
	// "Entfernen") just re-renders the form with refreshed hints.
	switch r.FormValue("action") {
	case "":
		// Auto-submit from the athlete dropdown — re-render with hints.
		s.renderSingleInvoiceForm(w, vm)
		return
	case "add_row":
		now := time.Now()
		next := singleLineRow{
			Idx:        len(vm.Lines),
			Year:       now.Year(),
			MonthValue: fmt.Sprintf("%02d", int(now.Month())),
		}
		// Default to the month after the last filled row, if any.
		if len(vm.Lines) > 0 {
			last := vm.Lines[len(vm.Lines)-1]
			if last.Year != 0 && last.MonthValue != "" {
				mi, _ := strconv.Atoi(last.MonthValue)
				ny, nm := last.Year, mi+1
				if nm == 13 {
					nm = 1
					ny++
				}
				next.Year = ny
				next.MonthValue = fmt.Sprintf("%02d", nm)
			}
		}
		vm.Lines = append(vm.Lines, next)
		s.renderSingleInvoiceForm(w, vm)
		return
	case "remove_row":
		idx, _ := strconv.Atoi(r.FormValue("row_idx"))
		if idx >= 0 && idx < len(vm.Lines) {
			vm.Lines = append(vm.Lines[:idx], vm.Lines[idx+1:]...)
		}
		if len(vm.Lines) == 0 {
			now := time.Now()
			vm.Lines = []singleLineRow{{Year: now.Year(), MonthValue: fmt.Sprintf("%02d", int(now.Month()))}}
		}
		s.renderSingleInvoiceForm(w, vm)
		return
	}

	// Validate before calling the builder.
	if vm.SelectedAthlete == "" {
		vm.Error = i18n.T("invoices.single.no_athlete")
		s.renderSingleInvoiceForm(w, vm)
		return
	}
	if len(vm.Lines) == 0 {
		vm.Error = i18n.T("invoices.single.no_months")
		s.renderSingleInvoiceForm(w, vm)
		return
	}

	// Parse rows into LineSpecs; report any invalid row.
	specs := make([]invoice.LineSpec, 0, len(vm.Lines))
	seen := map[string]bool{}
	for _, ln := range vm.Lines {
		mi, _ := strconv.Atoi(ln.MonthValue)
		if ln.Year == 0 || mi < 1 || mi > 12 {
			vm.Error = i18n.T("invoices.single.no_months")
			s.renderSingleInvoiceForm(w, vm)
			return
		}
		k := fmt.Sprintf("%04d-%02d", ln.Year, mi)
		if seen[k] {
			vm.Error = i18n.T("invoices.single.duplicate_month")
			s.renderSingleInvoiceForm(w, vm)
			return
		}
		seen[k] = true
		specs = append(specs, invoice.LineSpec{Year: ln.Year, Month: time.Month(mi)})
	}

	// Single-month + same-month "recreate" mode reuses the existing number
	// via RecreateSingleSameNumber. Multi-month never reuses numbers (Q5).
	var draft store.Invoice
	var err error
	if r.FormValue("mode") == "recreate" && len(specs) == 1 {
		var oldPDF string
		draft, oldPDF, err = invoice.RecreateSingleSameNumber(
			s.Store, specs[0].Year, specs[0].Month, vm.SelectedAthlete, vm.Tipp,
		)
		if err == nil && oldPDF != "" {
			_ = os.Remove(oldPDF)
		}
	} else {
		draft, err = invoice.BuildSingleMulti(s.Store, specs, vm.SelectedAthlete, vm.Tipp)
	}
	if err != nil {
		if errors.Is(err, invoice.ErrInvoiceExists) {
			// Compute the list of conflicting months for the conflict page.
			conflicts := invoice.ConflictingMonths(s.Store.Snapshot(), vm.SelectedAthlete, keysFromSpecs(specs))
			s.renderSingleConflict(w, vm, conflicts)
			return
		}
		if strings.Contains(err.Error(), "not active") {
			vm.Error = i18n.T("invoices.single.row_inactive") + ": " + err.Error()
			s.renderSingleInvoiceForm(w, vm)
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
		if a.ID == vm.SelectedAthlete {
			ath = a
			break
		}
	}

	pv := []previewDraft{{
		Number:  draft.Number,
		Display: displayNumberFallback(draft),
		Athlete: ath.FirstName + " " + ath.LastName,
		Period:  formatInvoicePeriod(draft.Lines),
		Amount:  invoice.FormatEUR(draft.Total),
		Tipp:    draft.Tipp,
	}}

	v := s.chrome("invoices", i18n.T("invoices.preview"), i18n.T("invoices.preview_desc"))
	v["Drafts"] = pv
	v["BatchID"] = batchID
	s.renderPage(w, "invoices_preview.html", v)
}

func keysFromSpecs(specs []invoice.LineSpec) []string {
	out := make([]string, 0, len(specs))
	for _, sp := range specs {
		out = append(out, fmt.Sprintf("%04d-%02d", sp.Year, int(sp.Month)))
	}
	return out
}

type singleConflictRow struct {
	Month         string // "Mai 2026"
	InvoiceNumber int
	DisplayNumber string
	StatusLabel   string
	StatusClass   string
}

func (s *Server) renderSingleConflict(w http.ResponseWriter, vm singleFormVM, conflicts []invoice.Conflict) {
	d := s.Store.Snapshot()
	var name string
	for _, a := range d.Athletes {
		if a.ID == vm.SelectedAthlete {
			name = strings.TrimSpace(a.FirstName + " " + a.LastName)
			break
		}
	}

	rows := make([]singleConflictRow, 0, len(conflicts))
	allProtected := true
	anyProtected := false
	for _, c := range conflicts {
		cls, lbl := statusBadge(c.Status)
		yr, mo := 0, 0
		fmt.Sscanf(c.Month, "%d-%d", &yr, &mo)
		label := fmt.Sprintf("%s %d", invoice.MonthDE(time.Month(mo)), yr)
		rows = append(rows, singleConflictRow{
			Month:         label,
			InvoiceNumber: c.InvoiceNumber,
			DisplayNumber: c.DisplayNumber,
			StatusLabel:   lbl,
			StatusClass:   cls,
		})
		if invoice.IsEditable(c.Status) {
			allProtected = false
		} else {
			anyProtected = true
		}
	}

	isSingleMonth := len(vm.Lines) == 1
	canRecreate := isSingleMonth && !allProtected && !anyProtected && len(conflicts) == 1

	v := s.chrome("invoices", i18n.T("invoices.conflict.title"), "")
	v["AthleteID"] = vm.SelectedAthlete
	v["AthleteName"] = name
	v["Tipp"] = vm.Tipp
	v["Lines"] = vm.Lines
	v["Conflicts"] = rows
	v["Protected"] = anyProtected
	v["CanRecreate"] = canRecreate
	v["IsSingleMonth"] = isSingleMonth
	s.renderPage(w, "invoices_conflict_single.html", v)
}

// summaryFromInvoiced renders the set of already-billed months as a sorted
// list of "Mai 2026" labels. The set is taken from
// invoice.InvoicedMonths for the selected athlete.
func summaryFromInvoiced(invoiced map[string]store.Invoice) []string {
	keys := make([]string, 0, len(invoiced))
	for k := range invoiced {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		y, mo := 0, 0
		fmt.Sscanf(k, "%d-%d", &y, &mo)
		out = append(out, fmt.Sprintf("%s %d", invoice.MonthDE(time.Month(mo)), y))
	}
	return out
}

func atoiOK(s string) (int, bool) {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	return n, err == nil
}
