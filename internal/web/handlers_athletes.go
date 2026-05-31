package web

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"coachly/internal/i18n"
	"coachly/internal/invoice"
	"coachly/internal/store"
)

type athleteRow struct {
	ID        string
	Name      string
	City      string
	StartDate string
	Fee       string
	Active    bool
}

func (s *Server) handleAthletes(w http.ResponseWriter, r *http.Request) {
	d := s.Store.Snapshot()
	q := strings.ToLower(r.URL.Query().Get("q"))
	statusFilter := r.URL.Query().Get("status")
	now := time.Now()

	rows := make([]athleteRow, 0, len(d.Athletes))
	for _, a := range d.Athletes {
		fullName := strings.TrimSpace(a.FirstName + " " + a.LastName)
		if q != "" && !strings.Contains(strings.ToLower(fullName), q) {
			continue
		}
		active := a.Active(now.Year(), now.Month())
		if statusFilter == "active" && !active {
			continue
		}
		if statusFilter == "inactive" && active {
			continue
		}
		rows = append(rows, athleteRow{
			ID:        a.ID,
			Name:      fullName,
			City:      a.Address.City,
			StartDate: a.StartDate.Format("02.01.2006"),
			Fee:       invoice.FormatEUR(a.MonthlyFee),
			Active:    active,
		})
	}

	v := s.chrome("athletes", i18n.T("athletes.title"), "")
	v["Athletes"] = rows
	v["Q"] = r.URL.Query().Get("q")
	v["StatusFilter"] = statusFilter
	s.renderPage(w, "athletes.html", v)
}

type athleteFormVM struct {
	A               store.Athlete
	FeeStr          string
	StartStr        string
	EndStr          string
	Action          string
	DeleteAction    string
	ShowDelete      bool
	HasInvoices     bool
	Invoices        []athleteInvoiceRow
	OpenCount       int
	OpenAmount      string
	InvoicesListURL string
	BillingGrid     []billingYearRow
	Error           string
}

type athleteInvoiceRow struct {
	Number      int
	Display     string
	Period      string
	Amount      string
	StatusClass string
	StatusLabel string
	PDFPath     string
}

// billingYearRow is one row of the chip grid on the athlete detail page:
// a year plus 12 chips (Jan…Dez), each describing the status of the invoice
// covering that month (or "none"/"inactive").
type billingYearRow struct {
	Year  int
	Chips []billingChip
}

type billingChip struct {
	Month       int    // 1..12
	Label       string // "Jan", "Feb", ...
	Status      string // "issued" | "sent" | "paid" | "pending_pdf" | "none" | "inactive"
	Class       string // tailwind classes for the chip background/border
	Title       string // tooltip — invoice number + status label
	AnchorTo    int    // invoice number; non-zero means linkable
	NumberLabel string // displayed when chip is linked, e.g. "#142"
}

func (s *Server) handleAthleteForm(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	vm := athleteFormVM{Action: "/athletes/new", StartStr: time.Now().Format("2006-01-02")}
	if id != "" {
		d := s.Store.Snapshot()
		for _, a := range d.Athletes {
			if a.ID == id {
				vm.A = a
				vm.FeeStr = fmt.Sprintf("%.2f", float64(a.MonthlyFee)/100)
				vm.StartStr = a.StartDate.Format("2006-01-02")
				if a.EndDate != nil {
					vm.EndStr = a.EndDate.Format("2006-01-02")
				}
				vm.Action = "/athletes/" + id + "/edit"
				vm.DeleteAction = "/athletes/" + id + "/delete"
				vm.ShowDelete = true
				break
			}
		}

		var open store.Money
		for _, inv := range d.Invoices {
			if inv.AthleteID != id {
				continue
			}
			cls, lbl := statusBadge(inv.Status)
			vm.Invoices = append(vm.Invoices, athleteInvoiceRow{
				Number:      inv.Number,
				Display:     displayNumberFallback(inv),
				Period:      formatInvoicePeriod(inv.Lines),
				Amount:      invoice.FormatEUR(inv.Total),
				StatusClass: cls,
				StatusLabel: lbl,
				PDFPath:     inv.PDFPath,
			})
			if inv.Status == store.StatusIssued || inv.Status == store.StatusSent {
				vm.OpenCount++
				open += inv.Total
			}
		}
		sort.Slice(vm.Invoices, func(i, j int) bool { return vm.Invoices[i].Number > vm.Invoices[j].Number })
		vm.HasInvoices = len(vm.Invoices) > 0
		vm.OpenAmount = invoice.FormatEUR(open)
		vm.InvoicesListURL = "/invoices?athlete=" + id
		vm.BillingGrid = buildBillingGrid(vm.A, d.Invoices)
	}
	v := s.chrome("athletes", i18n.T("athletes.add"), "")
	v["A"] = vm.A
	v["FeeStr"] = vm.FeeStr
	v["StartStr"] = vm.StartStr
	v["EndStr"] = vm.EndStr
	v["Action"] = vm.Action
	v["DeleteAction"] = vm.DeleteAction
	v["ShowDelete"] = vm.ShowDelete
	v["HasInvoices"] = vm.HasInvoices
	v["Invoices"] = vm.Invoices
	v["OpenCount"] = vm.OpenCount
	v["OpenAmount"] = vm.OpenAmount
	v["InvoicesListURL"] = vm.InvoicesListURL
	v["BillingGrid"] = vm.BillingGrid
	v["Error"] = vm.Error
	s.renderPage(w, "athlete_form.html", v)
}

// buildBillingGrid returns one row per year (current year + previous year)
// for the athlete's chip grid. Months outside the athlete's active range
// render as "inactive" chips; months covered by an invoice render with the
// invoice's status and link target (#invoice-N).
func buildBillingGrid(a store.Athlete, invoices []store.Invoice) []billingYearRow {
	if a.ID == "" {
		return nil
	}
	now := time.Now()
	currentYear := now.Year()
	startYear := currentYear - 1
	if !a.StartDate.IsZero() && a.StartDate.Year() > startYear {
		startYear = a.StartDate.Year()
	}
	endYear := currentYear
	if a.EndDate != nil && a.EndDate.Year() < endYear {
		endYear = a.EndDate.Year()
	}
	if startYear > endYear {
		startYear, endYear = currentYear, currentYear
	}

	// Index covering invoices by month for quick lookup.
	cover := map[string]store.Invoice{}
	for _, inv := range invoices {
		if inv.AthleteID != a.ID {
			continue
		}
		for _, l := range inv.Lines {
			cover[l.Month] = inv
		}
	}

	rows := make([]billingYearRow, 0, endYear-startYear+1)
	for y := endYear; y >= startYear; y-- {
		row := billingYearRow{Year: y, Chips: make([]billingChip, 0, 12)}
		for mo := time.January; mo <= time.December; mo++ {
			chip := billingChip{Month: int(mo), Label: shortMonth(mo)}
			if !a.Active(y, mo) {
				chip.Status = "inactive"
				chip.Class = "bg-slate-50 text-slate-300 border border-dashed border-slate-200"
				chip.Title = shortMonth(mo) + " " + intStr(y)
				row.Chips = append(row.Chips, chip)
				continue
			}
			key := fmt.Sprintf("%04d-%02d", y, int(mo))
			inv, has := cover[key]
			if !has {
				chip.Status = "none"
				chip.Class = "bg-white text-slate-400 border border-slate-200"
				chip.Title = shortMonth(mo) + " " + intStr(y) + " — nicht abgerechnet"
				row.Chips = append(row.Chips, chip)
				continue
			}
			chip.Status = string(inv.Status)
			chip.Class = chipClassForStatus(inv.Status)
			chip.AnchorTo = inv.Number
			chip.NumberLabel = "#" + displayNumberFallback(inv)
			chip.Title = shortMonth(mo) + " " + intStr(y) + " — " + chip.NumberLabel + " (" + statusLabel(inv.Status) + ")"
			row.Chips = append(row.Chips, chip)
		}
		rows = append(rows, row)
	}
	return rows
}

func chipClassForStatus(st store.InvoiceStatus) string {
	switch st {
	case store.StatusPaid:
		return "bg-teal-100 text-teal-700 border border-teal-200"
	case store.StatusSent:
		return "bg-indigo-100 text-indigo-700 border border-indigo-200"
	case store.StatusIssued:
		return "bg-amber-100 text-amber-700 border border-amber-200"
	default:
		return "bg-slate-100 text-slate-500 border border-slate-200"
	}
}

func statusLabel(st store.InvoiceStatus) string {
	_, lbl := statusBadge(st)
	return lbl
}

func intStr(n int) string { return strconv.Itoa(n) }

func (s *Server) handleAthleteCreate(w http.ResponseWriter, r *http.Request) {
	a, err := parseAthleteForm(r, "")
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	a.ID = uuid.NewString()
	if err := s.Store.Mutate(func(d *store.Data) error {
		d.Athletes = append(d.Athletes, a)
		return nil
	}); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/athletes", http.StatusSeeOther)
}

func (s *Server) handleAthleteUpdate(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	a, err := parseAthleteForm(r, id)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if err := s.Store.Mutate(func(d *store.Data) error {
		for i := range d.Athletes {
			if d.Athletes[i].ID == id {
				d.Athletes[i] = a
				return nil
			}
		}
		return fmt.Errorf("athlete %s not found", id)
	}); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/athletes", http.StatusSeeOther)
}

func (s *Server) handleAthleteDelete(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	errHasInvoices := fmt.Errorf("athlete has invoices")
	err := s.Store.Mutate(func(d *store.Data) error {
		for _, inv := range d.Invoices {
			if inv.AthleteID == id {
				return errHasInvoices
			}
		}
		out := d.Athletes[:0]
		for _, a := range d.Athletes {
			if a.ID != id {
				out = append(out, a)
			}
		}
		d.Athletes = out
		return nil
	})
	if err == errHasInvoices {
		http.Error(w, i18n.T("athletes.err_has_invoices"), http.StatusForbidden)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/athletes", http.StatusSeeOther)
}

func parseAthleteForm(r *http.Request, id string) (store.Athlete, error) {
	if err := r.ParseForm(); err != nil {
		return store.Athlete{}, err
	}
	fields := map[string]string{
		"first_name":   r.FormValue("first_name"),
		"last_name":    r.FormValue("last_name"),
		"street":       r.FormValue("street"),
		"postal_code":  r.FormValue("postal_code"),
		"city":         r.FormValue("city"),
		"country":      r.FormValue("country"),
		"monthly_fee":  r.FormValue("monthly_fee"),
		"start_date":   r.FormValue("start_date"),
		"end_date":     r.FormValue("end_date"),
		"email":        r.FormValue("email"),
		"notes":        r.FormValue("notes"),
	}
	a, err := parseAthleteFields(fields)
	if err != nil {
		return store.Athlete{}, err
	}
	a.ID = id
	return a, nil
}

// parseAthleteFields builds an Athlete from a flat string map. Used by both
// the HTML form handler and the CSV importer. ID is not populated — callers
// set it explicitly (form: existing ID; CSV: from the row, possibly empty).
func parseAthleteFields(f map[string]string) (store.Athlete, error) {
	email := strings.TrimSpace(f["email"])
	if email == "" {
		return store.Athlete{}, fmt.Errorf("E-Mail ist erforderlich")
	}
	feeStr := strings.Replace(strings.TrimSpace(f["monthly_fee"]), ",", ".", 1)
	feeF, err := strconv.ParseFloat(feeStr, 64)
	if err != nil {
		return store.Athlete{}, fmt.Errorf("ungültige Gebühr: %v", err)
	}
	start, err := time.Parse("2006-01-02", strings.TrimSpace(f["start_date"]))
	if err != nil {
		return store.Athlete{}, fmt.Errorf("ungültiges Startdatum: %v", err)
	}
	var endPtr *time.Time
	if v := strings.TrimSpace(f["end_date"]); v != "" {
		end, err := time.Parse("2006-01-02", v)
		if err != nil {
			return store.Athlete{}, fmt.Errorf("ungültiges Enddatum: %v", err)
		}
		endPtr = &end
	}
	return store.Athlete{
		FirstName: strings.TrimSpace(f["first_name"]),
		LastName:  strings.TrimSpace(f["last_name"]),
		Address: store.Address{
			Street:     strings.TrimSpace(f["street"]),
			PostalCode: strings.TrimSpace(f["postal_code"]),
			City:       strings.TrimSpace(f["city"]),
			Country:    strings.TrimSpace(f["country"]),
		},
		MonthlyFee: store.Money(feeF * 100),
		StartDate:  start,
		EndDate:    endPtr,
		Email:      email,
		Notes:      strings.TrimSpace(f["notes"]),
	}, nil
}
