package web

import (
	"bytes"
	"html/template"
	"net/http"
	"strings"

	"coachly/internal/i18n"
	"coachly/internal/invoice"
	"coachly/internal/store"
	"coachly/internal/web/templates"
)

var templateFuncs = template.FuncMap{
	"t":         i18n.T,
	"formatEUR": invoice.FormatEUR,
}

type navItem struct {
	Key   string
	Label string
	Href  string
}

func defaultNav() []navItem {
	return []navItem{
		{"dashboard", i18n.T("nav.dashboard"), "/dashboard"},
		{"athletes", i18n.T("nav.athletes"), "/athletes"},
		{"invoices", i18n.T("nav.invoices"), "/invoices"},
		{"reports", i18n.T("nav.reports"), "/reports"},
		{"settings", i18n.T("nav.settings"), "/settings"},
	}
}

// view is the data map passed to every chrome-wrapped template.
// Handlers add their own keys before calling render.
type view map[string]any

func (s *Server) chrome(active, title, subtitle string) view {
	d := s.Store.Snapshot()
	return view{
		"Title":        title,
		"PageTitle":    title,
		"PageSubtitle": subtitle,
		"ActiveNav":    active,
		"Nav":          defaultNav(),
		"Initials":     buildInitials(d.Coach.FirstName, d.Coach.LastName),
	}
}

func buildInitials(fn, ln string) string {
	var b strings.Builder
	if fn != "" {
		b.WriteRune([]rune(fn)[0])
	}
	if ln != "" {
		b.WriteRune([]rune(ln)[0])
	}
	if b.Len() == 0 {
		return "C"
	}
	return strings.ToUpper(b.String())
}

// renderPage parses layout.html + the requested content template fresh per
// request — necessary because each content file defines its own "content"
// block and Go's html/template forbids duplicate define names in one set.
func (s *Server) renderPage(w http.ResponseWriter, contentFile string, v view) {
	t, err := template.New("layout").Funcs(templateFuncs).ParseFS(
		templates.FS, "layout.html", contentFile,
	)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "layout", v); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}

func (s *Server) renderAuth(w http.ResponseWriter, v authVM) {
	t, err := template.New("auth").Funcs(templateFuncs).ParseFS(
		templates.FS, "auth.html",
	)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, "auth", v); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}

func statusBadge(st store.InvoiceStatus) (string, string) {
	switch st {
	case store.StatusPaid:
		return "bg-teal-100 text-teal-700", i18n.T("invoices.status.paid")
	case store.StatusIssued:
		return "bg-amber-100 text-amber-700", i18n.T("invoices.status.issued")
	default:
		return "bg-slate-100 text-slate-500", i18n.T("invoices.status.pending")
	}
}
