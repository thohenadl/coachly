// Package i18n holds all user-facing strings in a single map (NFR-L-02).
// Currently only German; the layout is ready for additional locales later.
package i18n

var de = map[string]string{
	// App-wide
	"app.title":           "coachly",
	"app.tagline":         "Coaching-Verwaltung & Rechnungsstellung",
	"app.greeting":        "Willkommen zurück, Coach!",
	"app.subgreeting":     "Hier ist die heutige Übersicht deiner Athleten.",
	"app.search":          "Suchen…",
	"app.logout":          "Abmelden",
	"app.save":            "Speichern",
	"app.cancel":          "Abbrechen",
	"app.delete":          "Löschen",
	"app.edit":            "Bearbeiten",
	"app.add":             "Hinzufügen",
	"app.confirm":         "Bestätigen",
	"app.back":            "Zurück",
	"app.next":            "Weiter",

	// Navigation
	"nav.dashboard": "Dashboard",
	"nav.athletes":  "Athleten",
	"nav.invoices":  "Rechnungen",
	"nav.reports":   "Auswertungen",
	"nav.settings":  "Einstellungen",

	// Auth
	"auth.welcome":            "Willkommen bei coachly",
	"auth.setup_title":        "Master-Passwort festlegen",
	"auth.setup_desc":         "Dieses Passwort entschlüsselt deine Daten. Es kann nicht wiederhergestellt werden — bewahre es sicher auf!",
	"auth.unlock_title":       "Tresor entsperren",
	"auth.unlock_desc":        "Gib dein Master-Passwort ein, um auf deine Daten zuzugreifen.",
	"auth.password":           "Passwort",
	"auth.password_repeat":    "Passwort wiederholen",
	"auth.unlock":             "Entsperren",
	"auth.create":             "Tresor anlegen",
	"auth.bad_password":       "Falsches Passwort.",
	"auth.password_mismatch":  "Die Passwörter stimmen nicht überein.",
	"auth.password_too_short": "Mindestens 8 Zeichen.",

	// Dashboard
	"dash.kpi.month_revenue":  "Umsatz Monat",
	"dash.kpi.outstanding":    "Offen",
	"dash.kpi.paid":           "Bezahlt",
	"dash.kpi.athletes":       "Athleten",
	"dash.recent_invoices":    "Letzte Rechnungen",
	"dash.invoice_overview":   "Rechnungs­übersicht",
	"dash.new_invoice":        "Neue Rechnungen",
	"dash.view_all":           "Alle anzeigen",
	"dash.empty.title":        "Noch keine Daten",
	"dash.empty.desc":         "Lege deinen ersten Athleten an, um loszulegen.",
	"dash.empty.cta":          "Ersten Athleten anlegen",

	// Athletes
	"athletes.title":         "Athleten",
	"athletes.add":           "Athlet hinzufügen",
	"athletes.first_name":    "Vorname",
	"athletes.last_name":     "Nachname",
	"athletes.street":        "Straße",
	"athletes.postal_code":   "PLZ",
	"athletes.city":          "Ort",
	"athletes.country":       "Land",
	"athletes.monthly_fee":   "Monatliche Gebühr (€)",
	"athletes.start_date":    "Coaching-Start",
	"athletes.end_date":      "Coaching-Ende (optional)",
	"athletes.email":         "E-Mail",
	"athletes.notes":         "Notizen",
	"athletes.active":        "Aktiv",
	"athletes.inactive":      "Inaktiv",
	"athletes.filter.name":   "Name…",
	"athletes.filter.status": "Status",
	"athletes.empty":         "Noch keine Athleten angelegt.",

	// Invoices
	"invoices.title":           "Rechnungen",
	"invoices.create":          "Rechnungen erzeugen",
	"invoices.month":           "Abrechnungsmonat",
	"invoices.tipp_default":    "Coaches Tipp (Monats-Default)",
	"invoices.tipp_default_ph": "z. B. Vergiss nicht das Stabilisations-Programm…",
	"invoices.tipp_override":   "Tipp-Override (optional)",
	"invoices.preview":         "Vorschau",
	"invoices.preview_desc":    "Klick dich durch alle Rechnungen, bevor sie endgültig erzeugt werden.",
	"invoices.confirm_all":     "Alle bestätigen & PDF erzeugen",
	"invoices.number":          "Nummer",
	"invoices.athlete":         "Athlet",
	"invoices.period":          "Zeitraum",
	"invoices.amount":          "Betrag",
	"invoices.status":          "Status",
	"invoices.issued_at":       "Erstellt",
	"invoices.status.pending":  "Entwurf",
	"invoices.status.issued":   "Erstellt",
	"invoices.status.paid":     "Bezahlt",
	"invoices.empty":           "Noch keine Rechnungen.",
	"invoices.open_pdf":        "PDF öffnen",

	// Reports
	"reports.title":          "Auswertungen",
	"reports.month_revenue":  "Umsatz aktueller Monat",
	"reports.total_earned":   "Gesamtumsatz",
	"reports.yearly":         "Jahresumsatz",
	"reports.ytd":            "Year-to-Date",

	// Settings
	"settings.title":              "Einstellungen",
	"settings.tab.coach":          "Coach-Stammdaten",
	"settings.tab.finanzamt":      "Finanzamt",
	"settings.tab.smtp":           "SMTP (v2)",
	"settings.tab.password":       "Passwort",
	"settings.smtp.disabled_note": "SMTP-Versand ist in v1 deaktiviert. Du kannst die Felder bereits ausfüllen — sie werden verschlüsselt gespeichert.",
	"settings.password_current":   "Aktuelles Passwort",
	"settings.password_new":       "Neues Passwort",
	"settings.password_changed":   "Passwort geändert.",

	// Months
	"month.1":  "Januar",
	"month.2":  "Februar",
	"month.3":  "März",
	"month.4":  "April",
	"month.5":  "Mai",
	"month.6":  "Juni",
	"month.7":  "Juli",
	"month.8":  "August",
	"month.9":  "September",
	"month.10": "Oktober",
	"month.11": "November",
	"month.12": "Dezember",
}

// T returns the German string for `key`, or the key itself if not found
// (cheap "missing translation" indicator for development).
func T(key string) string {
	if v, ok := de[key]; ok {
		return v
	}
	return key
}
