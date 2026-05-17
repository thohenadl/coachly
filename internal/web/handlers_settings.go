package web

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"coachly/internal/auth"
	"coachly/internal/i18n"
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
	v["Preferences"] = d.Preferences
	v["DefaultInvoicesDir"] = s.InvoicesDir
	v["EffectiveInvoicesDir"] = s.invoicesDirFor(d)
	s.renderPage(w, "settings.html", v)
}

func (s *Server) handleSettingsCoach(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
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
		return nil
	}); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/settings?tab=coach&saved=1", http.StatusSeeOther)
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
	if err := s.Store.Mutate(func(d *store.Data) error {
		d.SMTP.Host = r.FormValue("host")
		d.SMTP.Port = port
		d.SMTP.User = r.FormValue("user")
		d.SMTP.Password = r.FormValue("password")
		d.SMTP.From = r.FormValue("from")
		return nil
	}); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/settings?tab=smtp&saved=1", http.StatusSeeOther)
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
