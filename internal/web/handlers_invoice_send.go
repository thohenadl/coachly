package web

import (
	"errors"
	"net/http"
	"net/url"
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

	if err := s.sendInvoiceMail(d, inv, athlete); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/invoices", http.StatusSeeOther)
}

// sendInvoiceMail renders the configured email template, sends the invoice
// PDF to the athlete and flips the invoice to "sent" (FR-E-04).
func (s *Server) sendInvoiceMail(d store.Data, inv store.Invoice, athlete store.Athlete) error {
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
		return errors.New(i18n.T("invoices.send.err_smtp") + " " + err.Error())
	}
	return invoice.SetStatus(s.Store, inv.Number, store.StatusSent)
}

// sendableForMonth returns the invoices covering month that can be mailed:
// status issued, PDF present, athlete has an email (FR-E-08).
func sendableForMonth(d store.Data, month string) []store.Invoice {
	emails := map[string]bool{}
	for _, a := range d.Athletes {
		emails[a.ID] = strings.TrimSpace(a.Email) != ""
	}
	var out []store.Invoice
	for _, inv := range d.Invoices {
		if inv.CoversMonth(month) && inv.Status == store.StatusIssued &&
			inv.PDFPath != "" && emails[inv.AthleteID] {
			out = append(out, inv)
		}
	}
	return out
}

// handleInvoicesSendMonth mails every sendable invoice of one month
// (FR-E-08). Individual failures don't abort the batch.
func (s *Server) handleInvoicesSendMonth(w http.ResponseWriter, r *http.Request) {
	month := strings.TrimSpace(r.FormValue("month"))
	if month == "" {
		http.Error(w, "missing month", http.StatusBadRequest)
		return
	}
	d := s.Store.Snapshot()
	if !d.SMTP.Enabled {
		http.Error(w, i18n.T("invoices.send.err_not_enabled"), http.StatusBadRequest)
		return
	}
	athletes := map[string]store.Athlete{}
	for _, a := range d.Athletes {
		athletes[a.ID] = a
	}
	sent := 0
	var failed []string
	for _, inv := range sendableForMonth(d, month) {
		a := athletes[inv.AthleteID]
		if err := s.sendInvoiceMail(d, inv, a); err != nil {
			failed = append(failed, strings.TrimSpace(a.FirstName+" "+a.LastName))
			continue
		}
		sent++
	}
	q := url.Values{}
	q.Set("month", month)
	q.Set("sent", strconv.Itoa(sent))
	if len(failed) > 0 {
		q.Set("failed", strings.Join(failed, ", "))
	}
	http.Redirect(w, r, "/invoices?"+q.Encode(), http.StatusSeeOther)
}
