package web

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"coachly/internal/auth"
	"coachly/internal/i18n"
	"coachly/internal/invoice"
	"coachly/internal/mailer"
	"coachly/internal/store"
)

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	tab := r.URL.Query().Get("tab")
	if tab == "" {
		tab = "coach"
	}
	saved := r.URL.Query().Get("saved") == "1"
	errKey := r.URL.Query().Get("err")
	d := s.Store.Snapshot()
	v := s.chrome("settings", i18n.T("settings.title"), "")
	v["ActiveTab"] = tab
	v["Saved"] = saved
	if errKey != "" {
		v["Error"] = i18n.T(errKey)
	}
	v["Coach"] = d.Coach
	v["Finanzamt"] = d.Finanzamt
	v["SMTP"] = d.SMTP
	v["EmailTemplate"] = d.EmailTemplate
	v["EmailDefaultSubject"] = store.DefaultEmailSubject
	v["EmailDefaultBody"] = store.DefaultEmailBody
	v["EmailTokens"] = []string{
		"{Vorname}", "{Nachname}",
		"{CoachVorname}", "{CoachNachname}",
		"{Nummer}", "{MM}", "{JJJJ}",
		"{Betrag}", "{Datum}",
	}
	v["Preferences"] = d.Preferences
	v["DefaultInvoicesDir"] = s.InvoicesDir
	v["EffectiveInvoicesDir"] = s.invoicesDirFor(d)

	format := d.Preferences.NumberingFormat
	if format == "" {
		format = store.DefaultNumberingFormat
	}
	v["NumberingFormat"] = format
	v["NumberingPreview"] = numberingPreview(format, d)
	s.renderPage(w, "settings.html", v)
}

// numberingPreview renders a sample invoice number using the configured
// format, the next counter value, and a placeholder athlete so the user
// sees roughly what the next invoice will be labelled. Pure read — uses
// d.Coach for {initials}.
func numberingPreview(format string, d store.Data) string {
	sample := store.Athlete{FirstName: "Max", LastName: "Mustermann"}
	now := time.Now()
	num := d.Counter.NextInvoiceNumber
	if num == 0 {
		num = now.Year()*1000 + 1
	}
	return invoice.FormatDisplayNumber(format, now.Year(), now.Month(), num, sample, d.Coach)
}

func (s *Server) handleSettingsCoach(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	format := strings.TrimSpace(r.FormValue("numbering_format"))
	if format == "" {
		format = store.DefaultNumberingFormat
	}
	if !isSafeNumberingFormat(format) {
		http.Redirect(w, r, "/settings?tab=coach&err=settings.numbering.err_invalid", http.StatusSeeOther)
		return
	}
	if err := s.Store.Mutate(func(d *store.Data) error {
		d.Coach.FirstName = r.FormValue("first_name")
		d.Coach.LastName = r.FormValue("last_name")
		d.Coach.Address.Street = r.FormValue("street")
		d.Coach.Address.PostalCode = r.FormValue("postal_code")
		d.Coach.Address.City = r.FormValue("city")
		d.Coach.Address.Country = r.FormValue("country")
		d.Coach.Bank = r.FormValue("bank")
		d.Coach.AccountOwner = r.FormValue("account_owner")
		d.Coach.IBAN = r.FormValue("iban")
		d.Coach.BIC = r.FormValue("bic")
		d.Coach.Steuernummer = r.FormValue("steuernummer")
		d.Coach.UID = r.FormValue("uid")
		d.Preferences.NumberingFormat = format
		return nil
	}); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/settings?tab=coach&saved=1", http.StatusSeeOther)
}

// isSafeNumberingFormat rejects formats that could produce dangerous PDF
// filenames or break parsing. We allow any printable runes except path
// separators and dot-dot sequences. Known tokens of the form {Word} are
// always permitted (FormatDisplayNumber decides if they're known).
func isSafeNumberingFormat(s string) bool {
	if len(s) == 0 || len(s) > 120 {
		return false
	}
	if strings.Contains(s, "..") {
		return false
	}
	for _, r := range s {
		if r == '/' || r == '\\' || r == 0 {
			return false
		}
	}
	return true
}

func (s *Server) handleSettingsFinanzamt(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if err := s.Store.Mutate(func(d *store.Data) error {
		d.Finanzamt.Name = r.FormValue("name")
		d.Finanzamt.Address.Street = r.FormValue("street")
		d.Finanzamt.Address.PostalCode = r.FormValue("postal_code")
		d.Finanzamt.Address.City = r.FormValue("city")
		return nil
	}); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/settings?tab=finanzamt&saved=1", http.StatusSeeOther)
}

func (s *Server) handleSettingsSMTP(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	port, _ := strconv.Atoi(r.FormValue("port"))
	security := normalizeSMTPSecurity(r.FormValue("security"))
	if err := s.Store.Mutate(func(d *store.Data) error {
		d.SMTP.Host = r.FormValue("host")
		d.SMTP.Port = port
		d.SMTP.User = r.FormValue("user")
		d.SMTP.Password = r.FormValue("password")
		d.SMTP.From = r.FormValue("from")
		d.SMTP.Security = security
		d.SMTP.Enabled = r.FormValue("enabled") == "1"
		return nil
	}); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/settings?tab=smtp&saved=1", http.StatusSeeOther)
}

// handleSettingsSMTPTest sends a small plain-text email using either the
// currently-saved SMTP config or the values posted in the form (so the user
// can verify settings without saving first). Replies with JSON
// {"ok": bool, "message": "..."} so the settings UI can render the result
// inline without a page reload. The test recipient defaults to the From
// address.
func (s *Server) handleSettingsSMTPTest(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		writeSMTPTestResult(w, false, err.Error())
		return
	}
	cfg := s.Store.Snapshot().SMTP
	if v := strings.TrimSpace(r.FormValue("host")); v != "" {
		cfg.Host = v
	}
	if v := strings.TrimSpace(r.FormValue("port")); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Port = p
		}
	}
	if v := r.FormValue("user"); v != "" {
		cfg.User = v
	}
	if v := r.FormValue("password"); v != "" {
		cfg.Password = v
	}
	if v := strings.TrimSpace(r.FormValue("from")); v != "" {
		cfg.From = v
	}
	if v := strings.TrimSpace(r.FormValue("security")); v != "" {
		cfg.Security = normalizeSMTPSecurity(v)
	}
	cfg.Enabled = true

	to := strings.TrimSpace(r.FormValue("to"))
	if to == "" {
		to = cfg.From
	}
	if to == "" {
		writeSMTPTestResult(w, false, i18n.T("settings.smtp.test.err_no_recipient"))
		return
	}

	subject := i18n.T("settings.smtp.test.subject")
	body := i18n.T("settings.smtp.test.body")
	if err := mailer.SendText(cfg, to, subject, body); err != nil {
		writeSMTPTestResult(w, false, err.Error())
		return
	}
	writeSMTPTestResult(w, true, i18n.T("settings.smtp.test.ok")+" "+to)
}

func writeSMTPTestResult(w http.ResponseWriter, ok bool, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if !ok {
		w.WriteHeader(http.StatusOK) // keep 200 so the JSON is always read by fetch
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"ok": ok, "message": msg})
}

func (s *Server) handleSettingsStorage(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	dir := strings.TrimSpace(r.FormValue("invoices_dir"))
	if dir != "" {
		if !filepath.IsAbs(dir) {
			http.Redirect(w, r, "/settings?tab=storage&err=settings.storage.err_not_absolute", http.StatusSeeOther)
			return
		}
		dir = filepath.Clean(dir)
		if err := os.MkdirAll(dir, 0o700); err != nil {
			http.Redirect(w, r, "/settings?tab=storage&err=settings.storage.err_create", http.StatusSeeOther)
			return
		}
		probe, err := os.CreateTemp(dir, ".coachly-write-test-*")
		if err != nil {
			http.Redirect(w, r, "/settings?tab=storage&err=settings.storage.err_not_writable", http.StatusSeeOther)
			return
		}
		probe.Close()
		os.Remove(probe.Name())
	}
	if err := s.Store.Mutate(func(d *store.Data) error {
		d.Preferences.InvoicesDir = dir
		return nil
	}); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/settings?tab=storage&saved=1", http.StatusSeeOther)
}

// handleSettingsStoragePick opens a native OS folder dialog and returns the
// chosen absolute path as JSON: {"path": "/abs/path"}. On cancel it returns
// 204 No Content. On platforms without a picker it returns 501.
func (s *Server) handleSettingsStoragePick(w http.ResponseWriter, r *http.Request) {
	path, err := pickFolder(i18n.T("settings.storage.pick_prompt"))
	if err != nil {
		if errors.Is(err, errPickerCanceled) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.Error(w, err.Error(), http.StatusNotImplemented)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]string{"path": path})
}

func (s *Server) handleSettingsPassword(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	current := []byte(r.FormValue("current"))
	newPw := r.FormValue("new")
	newRepeat := r.FormValue("new_repeat")
	if newPw != newRepeat {
		http.Error(w, i18n.T("auth.password_mismatch"), 400)
		return
	}
	if len(newPw) < 8 {
		http.Error(w, i18n.T("auth.password_too_short"), 400)
		return
	}
	// Verify current password by attempting unlock with it.
	if err := s.Store.Unlock(current); err != nil {
		if err == auth.ErrBadPassword {
			http.Error(w, i18n.T("auth.bad_password"), 400)
			return
		}
		http.Error(w, err.Error(), 500)
		return
	}
	if err := s.Store.ChangePassword([]byte(newPw)); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/settings?tab=password&saved=1", http.StatusSeeOther)
}

// normalizeSMTPSecurity clamps user input to the known set so we never store
// an unknown mode. Anything we don't recognise becomes "auto" (the
// port-based fallback), which is the safest default for existing setups.
func normalizeSMTPSecurity(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "ssl", "starttls", "none":
		return strings.ToLower(strings.TrimSpace(v))
	default:
		return "auto"
	}
}
