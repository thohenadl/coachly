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
	if _, err := NewServer(s, dir, dir, nil); err != nil {
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
	srv, err := NewServer(s, dir, dir, nil)
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
				"SelectedYear": 2026, "SelectedMonth": "05",
				"YearOptions":  []int{2026},
				"MonthOptions": []monthOption{{Value: "05", Label: "Mai"}},
				"Athletes":     []athleteOption{},
				"Tipp":         "", "SelectedAthlete": "", "Error": "",
			},
			mustHave: []string{"Einzelne Rechnung"},
		},
		{
			name: "single conflict protected",
			file: "invoices_conflict_single.html",
			view: view{
				"Title": "x", "PageTitle": "x", "PageSubtitle": "", "ActiveNav": "invoices",
				"Nav": defaultNav(), "Initials": "C",
				"Month": "2026-05", "AthleteID": "a1", "AthleteName": "Alice",
				"Tipp": "", "Protected": true,
			},
			mustHave: []string{"versendet oder bezahlt"},
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
