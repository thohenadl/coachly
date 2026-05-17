package web

import (
	"net/http"

	"coachly/internal/auth"
	"coachly/internal/i18n"
)

type authVM struct {
	Title       string
	Heading     string
	Description string
	Action      string
	Submit      string
	Error       string
	Setup       bool
}

func (s *Server) handleLoginGet(w http.ResponseWriter, r *http.Request) {
	if s.Store.IsInitialized() {
		s.renderAuth(w, authVM{
			Title:       i18n.T("auth.unlock_title"),
			Heading:     i18n.T("auth.unlock_title"),
			Description: i18n.T("auth.unlock_desc"),
			Action:      "/login",
			Submit:      i18n.T("auth.unlock"),
		})
		return
	}
	s.renderAuth(w, authVM{
		Title:       i18n.T("auth.setup_title"),
		Heading:     i18n.T("auth.setup_title"),
		Description: i18n.T("auth.setup_desc"),
		Action:      "/login",
		Submit:      i18n.T("auth.create"),
		Setup:       true,
	})
}

func (s *Server) handleLoginPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	pw := []byte(r.FormValue("password"))

	if !s.Store.IsInitialized() {
		// Setup flow
		repeat := r.FormValue("password_repeat")
		if len(pw) < 8 {
			s.renderAuth(w, authVM{
				Title: i18n.T("auth.setup_title"), Heading: i18n.T("auth.setup_title"),
				Description: i18n.T("auth.setup_desc"), Action: "/login",
				Submit: i18n.T("auth.create"), Setup: true,
				Error: i18n.T("auth.password_too_short"),
			})
			return
		}
		if string(pw) != repeat {
			s.renderAuth(w, authVM{
				Title: i18n.T("auth.setup_title"), Heading: i18n.T("auth.setup_title"),
				Description: i18n.T("auth.setup_desc"), Action: "/login",
				Submit: i18n.T("auth.create"), Setup: true,
				Error: i18n.T("auth.password_mismatch"),
			})
			return
		}
		if err := s.Store.Create(pw); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		s.issueSession(w)
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}

	if err := s.Store.Unlock(pw); err != nil {
		msg := err.Error()
		if err == auth.ErrBadPassword {
			msg = i18n.T("auth.bad_password")
		}
		s.renderAuth(w, authVM{
			Title: i18n.T("auth.unlock_title"), Heading: i18n.T("auth.unlock_title"),
			Description: i18n.T("auth.unlock_desc"), Action: "/login",
			Submit: i18n.T("auth.unlock"),
			Error:  msg,
		})
		return
	}
	s.issueSession(w)
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	s.clearSession(w, r)
	s.Store.Lock()
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
