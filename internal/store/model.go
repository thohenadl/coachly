package store

import "time"

// SchemaVersion is bumped whenever the on-disk JSON shape changes.
// Migrations live in store.go (loadAndMigrate).
const SchemaVersion = 1

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
}

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
	StatusPaid    InvoiceStatus = "paid"
)

type Invoice struct {
	Number      int           `json:"number"`
	AthleteID   string        `json:"athlete_id"`
	Month       string        `json:"month"` // "2026-05"
	Tipp        string        `json:"tipp"`
	Description string        `json:"description"`
	PeriodFrom  time.Time     `json:"period_from"`
	PeriodTo    time.Time     `json:"period_to"`
	Amount      Money         `json:"amount_cents"` // gross, includes 20% USt
	ProRata     bool          `json:"pro_rata"`
	Status      InvoiceStatus `json:"status"`
	IssuedAt    time.Time     `json:"issued_at"`
	PDFPath     string        `json:"pdf_path,omitempty"`
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

type Data struct {
	SchemaVersion int           `json:"schema_version"`
	Coach         Coach         `json:"coach"`
	Finanzamt     Finanzamt     `json:"finanzamt"`
	SMTP          SMTP          `json:"smtp"`
	Athletes      []Athlete     `json:"athletes"`
	Invoices      []Invoice     `json:"invoices"`
	Tipps         []MonthlyTipp `json:"tipps"`
	Filters       []SavedFilter `json:"filters"`
	Counter       Counter       `json:"counter"`
}

func NewData() Data {
	return Data{
		SchemaVersion: SchemaVersion,
		Finanzamt:     Finanzamt{Name: "Finanzamt Innsbruck"},
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
