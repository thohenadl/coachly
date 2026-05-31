package web

import (
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"coachly/internal/store"
)

// TestNewServer_ParsesTemplates is a smoke test that catches template parse
// errors (duplicate defines, bad action syntax, missing i18n keys at parse
// time) at build/test time rather than first user request.
func TestNewServer_ParsesTemplates(t *testing.T) {
	dir := t.TempDir()
	s := store.New(filepath.Join(dir, "store.enc"))
	if err := s.Create([]byte("test-password-1234")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := NewServer(s, dir, dir, nil, nil); err != nil {
		t.Fatalf("NewServer: %v", err)
	}
}

// TestRenderNewTemplates exercises the templates added for the lifecycle
// change so that action-level errors (missing fields, bad t-keys) surface in
// tests rather than at first user click.
func TestRenderNewTemplates(t *testing.T) {
	dir := t.TempDir()
	s := store.New(filepath.Join(dir, "store.enc"))
	if err := s.Create([]byte("test-password-1234")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	srv, err := NewServer(s, dir, dir, nil, nil)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}

	cases := []struct {
		name     string
		file     string
		view     view
		mustHave []string
	}{
		{
			name: "conflict 3 options",
			file: "invoices_conflict.html",
			view: view{
				"Title": "x", "PageTitle": "x", "PageSubtitle": "", "ActiveNav": "invoices",
				"Nav": defaultNav(), "Initials": "C",
				"Month": "2026-05", "Count": 2, "TippDefault": "", "TippOverrides": nil,
			},
			mustHave: []string{"Mit selben Rechnungsnummern", "Alle überschreiben", "Nur fehlende"},
		},
		{
			name: "single form",
			file: "invoices_new_single.html",
			view: view{
				"Title": "x", "PageTitle": "x", "PageSubtitle": "", "ActiveNav": "invoices",
				"Nav": defaultNav(), "Initials": "C",
				"YearOptions":  []int{2026},
				"MonthOptions": []monthOption{{Value: "05", Label: "Mai"}},
				"Athletes":     []athleteOption{},
				"Lines": []singleLineRow{{Idx: 0, Year: 2026, MonthValue: "05"}},
				"Tipp":         "", "SelectedAthlete": "",
				"SummaryBilled": []string{},
				"Error":         "",
			},
			mustHave: []string{"Einzelne Rechnung", "Weiteren Monat"},
		},
		{
			name: "athlete form with chip grid",
			file: "athlete_form.html",
			view: view{
				"Title": "x", "PageTitle": "x", "PageSubtitle": "", "ActiveNav": "athletes",
				"Nav": defaultNav(), "Initials": "C",
				"A":               store.Athlete{ID: "alice", FirstName: "Alice", LastName: "Anders"},
				"FeeStr":          "200,00",
				"StartStr":        "2026-01-01",
				"EndStr":          "",
				"Action":          "/athletes/alice/edit",
				"DeleteAction":    "/athletes/alice/delete",
				"ShowDelete":      true,
				"HasInvoices":     true,
				"Invoices":        []athleteInvoiceRow{{Number: 142, Display: "142", Period: "Mai – Juli 2026", Amount: "600,00 €", StatusClass: "bg-amber-100 text-amber-700", StatusLabel: "Erstellt"}},
				"OpenCount":       1,
				"OpenAmount":      "600,00 €",
				"InvoicesListURL": "/invoices?athlete=alice",
				"BillingGrid": []billingYearRow{{
					Year: 2026,
					Chips: []billingChip{
						{Month: 5, Label: "Mai", Status: "issued", Class: "bg-amber-100", AnchorTo: 142, NumberLabel: "#142", Title: "Mai 2026"},
					},
				}},
				"Error": "",
			},
			mustHave: []string{"Abgerechnete Monate", "Mai – Juli 2026", "#invoice-142"},
		},
		{
			name: "single conflict protected",
			file: "invoices_conflict_single.html",
			view: view{
				"Title": "x", "PageTitle": "x", "PageSubtitle": "", "ActiveNav": "invoices",
				"Nav": defaultNav(), "Initials": "C",
				"AthleteID": "a1", "AthleteName": "Alice",
				"Tipp":          "",
				"Lines":         []singleLineRow{{Year: 2026, MonthValue: "05"}},
				"Conflicts":     []singleConflictRow{{Month: "Mai 2026", InvoiceNumber: 142, DisplayNumber: "142", StatusLabel: "Versendet", StatusClass: "bg-indigo-100 text-indigo-700"}},
				"Protected":     true,
				"CanRecreate":   false,
				"IsSingleMonth": true,
			},
			mustHave: []string{"versendet oder bezahlt", "Mai 2026"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rr := httptest.NewRecorder()
			srv.renderPage(rr, tc.file, tc.view)
			if rr.Code != 200 {
				t.Fatalf("status %d: %s", rr.Code, rr.Body.String())
			}
			body := rr.Body.String()
			for _, s := range tc.mustHave {
				if !strings.Contains(body, s) {
					t.Errorf("rendered body missing %q", s)
				}
			}
		})
	}
}
