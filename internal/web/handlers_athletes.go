package web

import (
	"fmt"
	"net/http"
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
	A            store.Athlete
	FeeStr       string
	StartStr     string
	EndStr       string
	Action       string
	DeleteAction string
	ShowDelete   bool
	Error        string
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
	}
	v := s.chrome("athletes", i18n.T("athletes.add"), "")
	v["A"] = vm.A
	v["FeeStr"] = vm.FeeStr
	v["StartStr"] = vm.StartStr
	v["EndStr"] = vm.EndStr
	v["Action"] = vm.Action
	v["DeleteAction"] = vm.DeleteAction
	v["ShowDelete"] = vm.ShowDelete
	v["Error"] = vm.Error
	s.renderPage(w, "athlete_form.html", v)
}

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
	if err := s.Store.Mutate(func(d *store.Data) error {
		out := d.Athletes[:0]
		for _, a := range d.Athletes {
			if a.ID != id {
				out = append(out, a)
			}
		}
		d.Athletes = out
		return nil
	}); err != nil {
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
