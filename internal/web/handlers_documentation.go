package web

import (
	"net/http"

	"coachly/internal/i18n"
)

func (s *Server) handleDocumentation(w http.ResponseWriter, r *http.Request) {
	v := s.chrome("documentation", i18n.T("docs.title"), i18n.T("docs.subtitle"))
	s.renderPage(w, "documentation.html", v)
}
