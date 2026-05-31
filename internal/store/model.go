package store

import "time"

// SchemaVersion is bumped whenever the on-disk JSON shape changes.
// Migrations live in store.go (loadAndMigrate).
const SchemaVersion = 4

// Money is stored in euro-cents to avoid float pitfalls.
type Money int64

type Address struct {
	Street     string `json:"street"`
	PostalCode string `json:"postal_code"`
	City       string `json:"city"`
	Country    string `json:"country"`
}

type Coach struct {
	FirstName    string  `json:"first_name"`
	LastName     string  `json:"last_name"`
	Address      Address `json:"address"`
	Bank         string  `json:"bank"`
	AccountOwner string  `json:"account_owner"`
	IBAN         string  `json:"iban"`
	BIC          string  `json:"bic"`
	Steuernummer string  `json:"steuernummer"`
	UID          string  `json:"uid"`
	Email        string  `json:"email"`
	Phone        string  `json:"phone"`
}

type Finanzamt struct {
	Name    string  `json:"name"`
	Address Address `json:"address"`
}

type SMTP struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	From     string `json:"from"`
	Enabled  bool   `json:"enabled"`
	// Security selects the transport mode independently of the port:
	//   "auto"     — derive from port (465 → ssl, anything else → starttls)
	//   "ssl"      — implicit TLS from the first byte (SMTPS)
	//   "starttls" — connect plain, then upgrade via STARTTLS
	//   "none"     — plain TCP only, no encryption (local test relays)
	// Empty string is treated as "auto" so v1 stores keep their behaviour.
	Security string `json:"security,omitempty"`
}

// EmailTemplate is the subject and body that goes out with each invoice
// email. Tokens (e.g. {Vorname}, {Nummer}, {MM}, {JJJJ}, {CoachVorname})
// are expanded at send time — see mailer.RenderTemplate.
type EmailTemplate struct {
	Subject string `json:"subject"`
	Body    string `json:"body"`
}

// DefaultEmailSubject and DefaultEmailBody are used when EmailTemplate is
// empty (v2 stores migrated to v3, or a fresh install).
const (
	DefaultEmailSubject = "Deine Rechnung {Nummer}"
	DefaultEmailBody    = "Hi {Vorname},\n\nim Anhang findest du deine Rechnung für {MM}/{JJJJ} mit der Nummer {Nummer}.\n\nViele Grüße,\n{CoachVorname}"
)

type Athlete struct {
	ID         string     `json:"id"`
	FirstName  string     `json:"first_name"`
	LastName   string     `json:"last_name"`
	Address    Address    `json:"address"`
	MonthlyFee Money      `json:"monthly_fee_cents"`
	StartDate  time.Time  `json:"start_date"`
	EndDate    *time.Time `json:"end_date,omitempty"`
	Email      string     `json:"email,omitempty"`
	Notes      string     `json:"notes,omitempty"`
}

// Active reports whether the athlete is active at any point during the given month.
func (a Athlete) Active(year int, month time.Month) bool {
	monthStart := time.Date(year, month, 1, 0, 0, 0, 0, time.UTC)
	monthEnd := monthStart.AddDate(0, 1, -1)
	if a.StartDate.After(monthEnd) {
		return false
	}
	if a.EndDate != nil && a.EndDate.Before(monthStart) {
		return false
	}
	return true
}

type InvoiceStatus string

const (
	StatusPending InvoiceStatus = "pending_pdf"
	StatusIssued  InvoiceStatus = "issued"
	StatusSent    InvoiceStatus = "sent"
	StatusPaid    InvoiceStatus = "paid"
)

// InvoiceLine is one billable month on an invoice (FR-I-06, amended for
// multi-month invoices). Each line stands on its own: its own pro-rata
// period, its own gross amount. An invoice's Total is the sum of its
// Lines[].Amount.
type InvoiceLine struct {
	Month       string    `json:"month"` // "2026-05" — dedup key
	PeriodFrom  time.Time `json:"period_from"`
	PeriodTo    time.Time `json:"period_to"`
	Description string    `json:"description"`
	Amount      Money     `json:"amount_cents"` // gross, includes 20% USt
	ProRata     bool      `json:"pro_rata"`
}

type Invoice struct {
	Number int `json:"number"`
	// DisplayNumber is the human-readable invoice identifier rendered from
	// Preferences.NumberingFormat at creation time. It is *frozen* — changes
	// to the format never alter existing invoices (FR-I-03 spirit). Number
	// remains the internal monotonic id used for URL routing & lookups.
	DisplayNumber string        `json:"display_number,omitempty"`
	AthleteID     string        `json:"athlete_id"`
	Tipp          string        `json:"tipp"`
	Lines         []InvoiceLine `json:"lines,omitempty"`
	Total         Money         `json:"total_cents"` // gross, includes 20% USt; equals sum of Lines[].Amount
	Status        InvoiceStatus `json:"status"`
	IssuedAt      time.Time     `json:"issued_at"`
	PDFPath       string        `json:"pdf_path,omitempty"`

	// Legacy v3 scalar fields. Read by migrate() to seed Lines and Total on
	// stores written before SchemaVersion=4. Cleared after migration; omitempty
	// keeps them out of every subsequent on-disk write.
	LegacyMonth       string    `json:"month,omitempty"`
	LegacyDescription string    `json:"description,omitempty"`
	LegacyPeriodFrom  time.Time `json:"period_from,omitempty"`
	LegacyPeriodTo    time.Time `json:"period_to,omitempty"`
	LegacyAmount      Money     `json:"amount_cents,omitempty"`
	LegacyProRata     bool      `json:"pro_rata,omitempty"`
}

// PrimaryMonth returns the first line's month key (e.g. "2026-05"), used as
// the canonical "year/month this invoice belongs to" when a single value is
// needed (PDF filename, mailer template, etc.). Empty for invoices with no
// lines (shouldn't happen post-migration).
func (i Invoice) PrimaryMonth() string {
	if len(i.Lines) == 0 {
		return ""
	}
	return i.Lines[0].Month
}

// CoversMonth reports whether any of the invoice's lines is for monthKey
// ("2026-05" format). Used by dedup and "already invoiced" hints.
func (i Invoice) CoversMonth(monthKey string) bool {
	for _, l := range i.Lines {
		if l.Month == monthKey {
			return true
		}
	}
	return false
}

// CoversYear reports whether any of the invoice's lines is in the given year.
func (i Invoice) CoversYear(year int) bool {
	prefix := monthYearPrefix(year)
	for _, l := range i.Lines {
		if len(l.Month) >= 4 && l.Month[:5] == prefix {
			return true
		}
	}
	return false
}

func monthYearPrefix(year int) string {
	// Keep allocation off the heap for hot loops in invoices_list/dashboard.
	const digits = "0123456789"
	y := year
	var buf [5]byte
	for i := 3; i >= 0; i-- {
		buf[i] = digits[y%10]
		y /= 10
	}
	buf[4] = '-'
	return string(buf[:])
}

// MonthlyTipp holds the default tipp for a month plus optional per-athlete
// overrides (FR-T-04). PerAthlete maps Athlete.ID → override text.
// If an athlete is absent or maps to "", the default Text applies.
type MonthlyTipp struct {
	Month      string            `json:"month"` // "2026-05"
	Text       string            `json:"text"`
	PerAthlete map[string]string `json:"per_athlete,omitempty"`
}

// Resolve returns the effective tipp for a given athlete ID.
func (t MonthlyTipp) Resolve(athleteID string) string {
	if v, ok := t.PerAthlete[athleteID]; ok && v != "" {
		return v
	}
	return t.Text
}

type SavedFilter struct {
	Name      string `json:"name"`
	Predicate string `json:"predicate"` // JSON-encoded UI filter state
}

type Counter struct {
	NextInvoiceNumber int `json:"next_invoice_number"`
}

// Preferences holds user-configurable runtime settings that aren't part of
// coach/finanzamt/SMTP. InvoicesDir, when non-empty, overrides the default
// PDF output folder (FR-P-10). NumberingFormat is the template applied to
// invoice numbers; empty means "{YYYY}{NNN}" (legacy behaviour). See
// invoice.FormatDisplayNumber for the supported tokens.
type Preferences struct {
	InvoicesDir     string `json:"invoices_dir,omitempty"`
	NumberingFormat string `json:"numbering_format,omitempty"`
}

// DefaultNumberingFormat is used when Preferences.NumberingFormat is empty.
// It renders the same string a v1 store produced: year + zero-padded counter
// (e.g. "2026010").
const DefaultNumberingFormat = "{YYYY}{NNN}"

type Data struct {
	SchemaVersion int           `json:"schema_version"`
	Coach         Coach         `json:"coach"`
	Finanzamt     Finanzamt     `json:"finanzamt"`
	SMTP          SMTP          `json:"smtp"`
	EmailTemplate EmailTemplate `json:"email_template"`
	Athletes      []Athlete     `json:"athletes"`
	Invoices      []Invoice     `json:"invoices"`
	Tipps         []MonthlyTipp `json:"tipps"`
	Filters       []SavedFilter `json:"filters"`
	Counter       Counter       `json:"counter"`
	Preferences   Preferences   `json:"preferences"`
}

func NewData() Data {
	return Data{
		SchemaVersion: SchemaVersion,
		Finanzamt:     Finanzamt{Name: "Finanzamt Innsbruck"},
		EmailTemplate: EmailTemplate{Subject: DefaultEmailSubject, Body: DefaultEmailBody},
		Athletes:      []Athlete{},
		Invoices:      []Invoice{},
		Tipps:         []MonthlyTipp{},
		Filters:       []SavedFilter{},
		Counter:       Counter{NextInvoiceNumber: nextNumberSeed()},
	}
}

func nextNumberSeed() int {
	now := time.Now()
	return now.Year()*1000 + 1
}
