package web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"

	"coachly/internal/auth"
	"coachly/internal/i18n"
	"coachly/internal/store"
)

// handleDataExport streams the current store as pretty-printed JSON.
// This is the "decoded" form the user can put under version control.
// The SMTP password is redacted so the export can be shared or stored
// without leaking the mail credential; on re-import an empty password is
// treated as "keep the existing one" (see handleDataImportConfirm).
func (s *Server) handleDataExport(w http.ResponseWriter, r *http.Request) {
	d := s.Store.Snapshot()
	d.SMTP.Password = ""
	body, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	filename := fmt.Sprintf("coachly-export-%s.json", time.Now().Format("2006-01-02"))
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	_, _ = w.Write(body)
}

// handleDataImportPreview parses an uploaded JSON file and stashes it for
// confirmation. We do NOT touch the store here — the user must confirm.
func (s *Server) handleDataImportPreview(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(20 << 20); err != nil {
		http.Redirect(w, r, "/settings?tab=data&err=settings.data.err_upload", http.StatusSeeOther)
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Redirect(w, r, "/settings?tab=data&err=settings.data.err_no_file", http.StatusSeeOther)
		return
	}
	defer file.Close()
	raw, err := io.ReadAll(file)
	if err != nil {
		http.Redirect(w, r, "/settings?tab=data&err=settings.data.err_upload", http.StatusSeeOther)
		return
	}
	var d store.Data
	if err := json.Unmarshal(raw, &d); err != nil {
		http.Redirect(w, r, "/settings?tab=data&err=settings.data.err_json", http.StatusSeeOther)
		return
	}

	token := uuid.NewString()
	s.mu.Lock()
	s.pendingImports[token] = d
	s.mu.Unlock()

	v := s.chrome("settings", i18n.T("settings.data.import_confirm_title"), "")
	v["Token"] = token
	v["AthleteCount"] = len(d.Athletes)
	v["InvoiceCount"] = len(d.Invoices)
	v["TippCount"] = len(d.Tipps)
	v["NextNumber"] = d.Counter.NextInvoiceNumber
	v["SchemaVersion"] = d.SchemaVersion
	s.renderPage(w, "data_import_confirm.html", v)
}

// handleDataImportConfirm replaces the store with the previously-stashed
// pending data after the user clicks "Yes, replace everything".
func (s *Server) handleDataImportConfirm(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	token := r.FormValue("token")
	s.mu.Lock()
	d, ok := s.pendingImports[token]
	if ok {
		delete(s.pendingImports, token)
	}
	s.mu.Unlock()
	if !ok {
		http.Redirect(w, r, "/settings?tab=data&err=settings.data.err_expired", http.StatusSeeOther)
		return
	}
	if d.SMTP.Password == "" {
		d.SMTP.Password = s.Store.Snapshot().SMTP.Password
	}
	if err := s.Store.ReplaceAll(d); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/settings?tab=data&saved=1", http.StatusSeeOther)
}

// handleDataReset wipes the entire data store (athletes, invoices, tipps,
// filters, counter, coach, finanzamt, SMTP, email template, preferences) and
// replaces it with a fresh empty Data. The current password must be supplied
// and is verified by re-running Unlock; the existing AES key+salt are reused
// so no password re-entry is needed afterwards.
func (s *Server) handleDataReset(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if r.FormValue("confirm") != "RESET" {
		http.Redirect(w, r, "/settings?tab=data&err=settings.data.reset.err_confirm", http.StatusSeeOther)
		return
	}
	password := []byte(r.FormValue("password"))
	if len(password) == 0 {
		http.Redirect(w, r, "/settings?tab=data&err=settings.data.reset.err_password", http.StatusSeeOther)
		return
	}
	if err := s.Store.Unlock(password); err != nil {
		if err == auth.ErrBadPassword {
			http.Redirect(w, r, "/settings?tab=data&err=settings.data.reset.err_bad_password", http.StatusSeeOther)
			return
		}
		http.Error(w, err.Error(), 500)
		return
	}
	if err := s.Store.ReplaceAll(store.NewData()); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/settings?tab=data&saved=1", http.StatusSeeOther)
}
