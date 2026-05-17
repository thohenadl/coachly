// Package web wires up the HTTP server, session middleware, and handlers.
package web

import (
	"html/template"
	"io/fs"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"coachly/internal/store"
	"coachly/internal/web/templates"
)

// AssetsFS is set by main.go to the embedded UI assets (CSS + vendored JS).
type Server struct {
	Store       *store.Store
	DataDir     string
	InvoicesDir string
	AssetsFS    fs.FS
	tmpl        *template.Template

	mu       sync.Mutex
	sessions map[string]time.Time
	// Pending invoice batches awaiting confirmation (keyed by batch_id).
	batches map[string]*Batch
	// Pending JSON imports awaiting "are you sure" confirmation (keyed by token).
	pendingImports map[string]store.Data
	// Pending bulk athlete CSV imports awaiting confirmation (keyed by token).
	pendingAthleteImports map[string]*athleteImport
}

// Batch holds preview-stage invoice drafts before confirmation.
type Batch struct {
	Drafts    []store.Invoice
	CreatedAt time.Time
}

func NewServer(s *store.Store, dataDir, invoicesDir string, assets fs.FS) (*Server, error) {
	tmpl, err := template.New("").
		Funcs(templateFuncs).
		ParseFS(templates.FS, "*.html")
	if err != nil {
		return nil, err
	}
	return &Server{
		Store:       s,
		DataDir:     dataDir,
		InvoicesDir: invoicesDir,
		AssetsFS:    assets,
		tmpl:        tmpl,
		sessions:              map[string]time.Time{},
		batches:               map[string]*Batch{},
		pendingImports:        map[string]store.Data{},
		pendingAthleteImports: map[string]*athleteImport{},
	}, nil
}

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()

	r.Handle("/assets/*", http.StripPrefix("/assets/", http.FileServer(http.FS(s.AssetsFS))))

	r.Get("/login", s.handleLoginGet)
	r.Post("/login", s.handleLoginPost)
	r.Get("/logout", s.handleLogout)

	r.Group(func(r chi.Router) {
		r.Use(s.requireAuth)
		r.Get("/", s.handleDashboard)
		r.Get("/dashboard", s.handleDashboard)

		r.Get("/athletes", s.handleAthletes)
		r.Get("/athletes/new", s.handleAthleteForm)
		r.Post("/athletes/new", s.handleAthleteCreate)
		r.Get("/athletes/import", s.handleAthleteImportForm)
		r.Post("/athletes/import/preview", s.handleAthleteImportPreview)
		r.Post("/athletes/import/confirm", s.handleAthleteImportConfirm)
		r.Get("/athletes/{id}/edit", s.handleAthleteForm)
		r.Post("/athletes/{id}/edit", s.handleAthleteUpdate)
		r.Post("/athletes/{id}/delete", s.handleAthleteDelete)

		r.Get("/invoices", s.handleInvoices)
		r.Get("/invoices/new", s.handleInvoicesNewForm)
		r.Post("/invoices/new", s.handleInvoicesPreview)
		r.Post("/invoices/confirm", s.handleInvoicesConfirm)
		r.Post("/invoices/{number}/status", s.handleInvoiceStatus)
		r.Get("/invoices/{number}/pdf", s.handleInvoicePDF)

		r.Get("/reports", s.handleReports)

		r.Get("/settings", s.handleSettings)
		r.Post("/settings/coach", s.handleSettingsCoach)
		r.Post("/settings/finanzamt", s.handleSettingsFinanzamt)
		r.Post("/settings/smtp", s.handleSettingsSMTP)
		r.Post("/settings/storage", s.handleSettingsStorage)
		r.Post("/settings/password", s.handleSettingsPassword)
		r.Get("/settings/data/export", s.handleDataExport)
		r.Post("/settings/data/import/preview", s.handleDataImportPreview)
		r.Post("/settings/data/import/confirm", s.handleDataImportConfirm)
	})

	return r
}

// --- session helpers ---

const sessionCookie = "coachly_session"
const sessionTTL = 8 * time.Hour

func (s *Server) issueSession(w http.ResponseWriter) {
	id := uuid.NewString()
	s.mu.Lock()
	s.sessions[id] = time.Now().Add(sessionTTL)
	s.mu.Unlock()
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    id,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Expires:  time.Now().Add(sessionTTL),
	})
}

func (s *Server) clearSession(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		s.mu.Lock()
		delete(s.sessions, c.Value)
		s.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1})
}

func (s *Server) hasSession(r *http.Request) bool {
	c, err := r.Cookie(sessionCookie)
	if err != nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	exp, ok := s.sessions[c.Value]
	if !ok || time.Now().After(exp) {
		delete(s.sessions, c.Value)
		return false
	}
	return true
}

// invoicesDirFor returns the effective output folder for invoice PDFs:
// the user-configured override if set, otherwise the platform default.
func (s *Server) invoicesDirFor(d store.Data) string {
	if dir := d.Preferences.InvoicesDir; dir != "" {
		return dir
	}
	return s.InvoicesDir
}

func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.hasSession(r) {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}
