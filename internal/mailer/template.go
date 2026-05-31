package mailer

import (
	"fmt"
	"strings"
	"time"

	"coachly/internal/invoice"
	"coachly/internal/store"
)

// PlaceholderTokens lists the tokens supported in EmailTemplate.Subject and
// EmailTemplate.Body. Exposed so the settings UI can show the same list the
// renderer understands.
var PlaceholderTokens = []string{
	"{Vorname}", "{Nachname}",
	"{CoachVorname}", "{CoachNachname}",
	"{Nummer}", "{MM}", "{JJJJ}",
	"{Betrag}", "{Datum}",
}

// RenderTemplate expands the supported tokens in s using the given invoice,
// athlete, and coach. The month/year come from the invoice's first line
// (PrimaryMonth); if absent or malformed, MM/JJJJ fall back to inv.IssuedAt.
// {Betrag} is the gross total across all lines.
func RenderTemplate(s string, inv store.Invoice, athlete store.Athlete, coach store.Coach) string {
	month, year := monthAndYear(inv)
	number := inv.DisplayNumber
	if number == "" {
		number = fmt.Sprintf("%d", inv.Number)
	}
	r := strings.NewReplacer(
		"{Vorname}", athlete.FirstName,
		"{Nachname}", athlete.LastName,
		"{CoachVorname}", coach.FirstName,
		"{CoachNachname}", coach.LastName,
		"{Nummer}", number,
		"{MM}", fmt.Sprintf("%02d", month),
		"{JJJJ}", fmt.Sprintf("%04d", year),
		"{Betrag}", invoice.FormatEUR(inv.Total),
		"{Datum}", inv.IssuedAt.Format("02.01.2006"),
	)
	return r.Replace(s)
}

func monthAndYear(inv store.Invoice) (int, int) {
	primary := inv.PrimaryMonth()
	if len(primary) == 7 && primary[4] == '-' {
		var y, m int
		if _, err := fmt.Sscanf(primary, "%d-%d", &y, &m); err == nil {
			return m, y
		}
	}
	t := inv.IssuedAt
	if t.IsZero() {
		t = time.Now()
	}
	return int(t.Month()), t.Year()
}
