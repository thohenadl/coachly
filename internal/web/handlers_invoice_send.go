package web

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"coachly/internal/i18n"
	"coachly/internal/invoice"
	"coachly/internal/mailer"
	"coachly/internal/store"
)

func (s *Server) handleInvoiceSend(w http.ResponseWriter, r *http.Request) {
	numStr := chi.URLParam(r, "number")
	n, err := strconv.Atoi(numStr)
	if err != nil {
		http.Error(w, "bad number", 400)
		return
	}
	d := s.Store.Snapshot()
	if !d.SMTP.Enabled {
		http.Error(w, i18n.T("invoices.send.err_not_enabled"), http.StatusBadRequest)
		return
	}

	var (
		inv     store.Invoice
		athlete store.Athlete
		found   bool
	)
	for _, x := range d.Invoices {
		if x.Number == n {
			inv = x
			found = true
			break
		}
	}
	if !found {
		http.NotFound(w, r)
		return
	}
	if inv.Status != store.StatusIssued {
		http.Error(w, i18n.T("invoices.send.err_bad_status"), http.StatusBadRequest)
		return
	}
	if inv.PDFPath == "" {
		http.Error(w, i18n.T("invoices.send.err_no_pdf"), http.StatusBadRequest)
		return
	}
	for _, a := range d.Athletes {
		if a.ID == inv.AthleteID {
			athlete = a
			break
		}
	}
	if strings.TrimSpace(athlete.Email) == "" {
		http.Error(w, i18n.T("invoices.send.err_no_email"), http.StatusBadRequest)
		return
	}

	subjectTmpl := d.EmailTemplate.Subject
	if strings.TrimSpace(subjectTmpl) == "" {
		subjectTmpl = store.DefaultEmailSubject
	}
	bodyTmpl := d.EmailTemplate.Body
	if strings.TrimSpace(bodyTmpl) == "" {
		bodyTmpl = store.DefaultEmailBody
	}
	subject := mailer.RenderTemplate(subjectTmpl, inv, athlete, d.Coach)
	body := mailer.RenderTemplate(bodyTmpl, inv, athlete, d.Coach)

	if err := mailer.Send(d.SMTP, athlete.Email, subject, body, inv.PDFPath); err != nil {
		http.Error(w, i18n.T("invoices.send.err_smtp")+" "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := invoice.SetStatus(s.Store, inv.Number, store.StatusSent); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, "/invoices", http.StatusSeeOther)
}
