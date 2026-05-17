// Package pdf renders a single-page invoice PDF per requirements FR-P-*.
package pdf

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/johnfercher/maroto/v2"
	"github.com/johnfercher/maroto/v2/pkg/components/col"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
	"github.com/johnfercher/maroto/v2/pkg/config"
	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/johnfercher/maroto/v2/pkg/consts/fontstyle"
	"github.com/johnfercher/maroto/v2/pkg/core"
	"github.com/johnfercher/maroto/v2/pkg/props"

	"coachly/internal/invoice"
	"coachly/internal/store"
)

// Render writes the invoice PDF to outDir and returns the full path.
// outDir is created if it doesn't exist.
func Render(outDir string, coach store.Coach, fa store.Finanzamt, athlete store.Athlete, inv store.Invoice) (string, error) {
	cfg := config.NewBuilder().
		WithLeftMargin(15).
		WithRightMargin(15).
		WithTopMargin(15).
		WithBottomMargin(15).
		WithAuthor("coachly", true).
		WithTitle(fmt.Sprintf("Rechnung %d", inv.Number), true).
		Build()
	m := maroto.New(cfg)

	addHeader(m, inv)
	addAddresses(m, coach, fa, athlete)
	addMeta(m, coach, inv)
	addTipp(m, inv.Tipp)
	addLineItems(m, inv)
	addPaymentText(m)
	addBankBlock(m, coach)
	addClosing(m, coach)

	doc, err := m.Generate()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(outDir, 0o700); err != nil {
		return "", err
	}
	month := monthFromString(inv.Month)
	year := yearFromString(inv.Month)
	name := invoice.PDFFilename(athlete.FirstName, year, month, inv.Number)
	full := filepath.Join(outDir, name)
	if err := doc.Save(full); err != nil {
		return "", err
	}
	return full, nil
}

func boldText(content string, size float64) core.Col {
	return text.NewCol(12, content, props.Text{Size: size, Style: fontstyle.Bold})
}

func plainText(content string, size float64) core.Col {
	return text.NewCol(12, content, props.Text{Size: size})
}

func addHeader(m core.Maroto, inv store.Invoice) {
	m.AddRow(14,
		text.NewCol(8, "Rechnung", props.Text{Size: 22, Style: fontstyle.Bold, Align: align.Left}),
		text.NewCol(4, fmt.Sprintf("Nr. %d", inv.Number), props.Text{Size: 12, Style: fontstyle.Bold, Align: align.Right, Top: 6}),
	)
	m.AddRow(2)
}

func addAddresses(m core.Maroto, coach store.Coach, fa store.Finanzamt, a store.Athlete) {
	// Two-column address block: sender left, recipient right
	senderLines := []string{
		fmt.Sprintf("%s %s", coach.FirstName, coach.LastName),
		coach.Address.Street,
		fmt.Sprintf("%s %s", coach.Address.PostalCode, coach.Address.City),
		coach.Address.Country,
	}
	recipientLines := []string{
		fmt.Sprintf("%s %s", a.FirstName, a.LastName),
		a.Address.Street,
		fmt.Sprintf("%s %s", a.Address.PostalCode, a.Address.City),
		a.Address.Country,
	}

	m.AddRow(5,
		text.NewCol(6, "Absender:", props.Text{Size: 9, Style: fontstyle.Bold}),
		text.NewCol(6, "Empfänger:", props.Text{Size: 9, Style: fontstyle.Bold}),
	)
	for i := 0; i < 4; i++ {
		m.AddRow(5,
			text.NewCol(6, senderLines[i], props.Text{Size: 10}),
			text.NewCol(6, recipientLines[i], props.Text{Size: 10}),
		)
	}
	m.AddRow(3)
	m.AddRow(5, text.NewCol(12, fmt.Sprintf("Finanzamt: %s", fa.Name), props.Text{Size: 9}))
	m.AddRow(4)
}

func addMeta(m core.Maroto, coach store.Coach, inv store.Invoice) {
	meta := [][2]string{
		{"Datum:", inv.IssuedAt.Format("02.01.2006")},
		{"Rechnungsnummer:", fmt.Sprintf("%d", inv.Number)},
		{"Steuernummer:", coach.Steuernummer},
		{"UID:", coach.UID},
	}
	for _, kv := range meta {
		m.AddRow(5,
			text.NewCol(4, kv[0], props.Text{Size: 9, Style: fontstyle.Bold}),
			text.NewCol(8, kv[1], props.Text{Size: 10}),
		)
	}
	m.AddRow(4)
}

func addTipp(m core.Maroto, tipp string) {
	if tipp == "" {
		return
	}
	m.AddRow(5, boldText("Coaches Tipp:", 9))
	m.AddRow(10, text.NewCol(12, tipp, props.Text{Size: 10, Style: fontstyle.Italic}))
	m.AddRow(4)
}

func addLineItems(m core.Maroto, inv store.Invoice) {
	m.AddRow(6,
		text.NewCol(4, "Zeitraum", props.Text{Size: 9, Style: fontstyle.Bold}),
		text.NewCol(5, "Beschreibung", props.Text{Size: 9, Style: fontstyle.Bold}),
		text.NewCol(3, "Kosten in Euro (inkl. 20 % USt)", props.Text{Size: 9, Style: fontstyle.Bold, Align: align.Right}),
	)
	period := fmt.Sprintf("%s – %s",
		inv.PeriodFrom.Format("02.01.2006"),
		inv.PeriodTo.Format("02.01.2006"),
	)
	m.AddRow(6,
		text.NewCol(4, period, props.Text{Size: 10}),
		text.NewCol(5, inv.Description, props.Text{Size: 10}),
		text.NewCol(3, invoice.FormatEUR(inv.Amount), props.Text{Size: 10, Align: align.Right}),
	)
	m.AddRow(2)
	m.AddRow(6,
		col.New(9),
		text.NewCol(3, "Total: "+invoice.FormatEUR(inv.Amount), props.Text{Size: 11, Style: fontstyle.Bold, Align: align.Right}),
	)
	m.AddRow(4)
}

func addPaymentText(m core.Maroto) {
	m.AddRow(10, plainText(
		"Bitte überweise den oben genannten Betrag binnen 10 Tagen auf das unten genannte Konto.",
		10,
	))
	m.AddRow(2)
}

func addBankBlock(m core.Maroto, c store.Coach) {
	rows := [][2]string{
		{"Bank:", c.Bank},
		{"Kontoinhaber:", c.AccountOwner},
		{"IBAN:", c.IBAN},
		{"BIC:", c.BIC},
	}
	for _, kv := range rows {
		m.AddRow(5,
			text.NewCol(3, kv[0], props.Text{Size: 9, Style: fontstyle.Bold}),
			text.NewCol(9, kv[1], props.Text{Size: 10}),
		)
	}
	m.AddRow(6)
}

func addClosing(m core.Maroto, c store.Coach) {
	m.AddRow(6, plainText("Vielen Dank für die Zusammenarbeit!", 10))
	m.AddRow(6, plainText("Mit sportlichen Grüßen,", 10))
	m.AddRow(6, plainText(fmt.Sprintf("%s %s", c.FirstName, c.LastName), 10))
}

// monthFromString parses "YYYY-MM" → month int.
func monthFromString(s string) int {
	if len(s) != 7 {
		return int(time.Now().Month())
	}
	return int(s[5]-'0')*10 + int(s[6]-'0')
}

func yearFromString(s string) int {
	if len(s) != 7 {
		return time.Now().Year()
	}
	y := 0
	for i := 0; i < 4; i++ {
		y = y*10 + int(s[i]-'0')
	}
	return y
}
