package web

import (
	"testing"
	"time"

	"coachly/internal/store"
)

func TestCategorizeAthleteCSV(t *testing.T) {
	existing := []store.Athlete{
		{
			ID: "id-keep", FirstName: "Anna", LastName: "Berger",
			Address:    store.Address{Street: "Hauptstr. 1", PostalCode: "6020", City: "Innsbruck", Country: "AT"},
			MonthlyFee: 35000,
			StartDate:  time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
			Email:      "anna@example.com",
		},
		{
			ID: "id-conflict", FirstName: "Ben", LastName: "Cramer",
			MonthlyFee: 40000,
			StartDate:  time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC),
			Email:      "ben@example.com",
		},
	}

	// Row 1: header
	// Row 2: id-keep with identical data → Unchanged
	// Row 3: id-conflict with a different fee → Conflict
	// Row 4: empty id → New (will get a UUID)
	// Row 5: id-new (not in store) → New
	// Row 6: bad date → Error
	csv := "" +
		"id,first_name,last_name,street,postal_code,city,country,monthly_fee,start_date,end_date,email,notes\n" +
		"id-keep,Anna,Berger,Hauptstr. 1,6020,Innsbruck,AT,350.00,2025-01-01,,anna@example.com,\n" +
		"id-conflict,Ben,Cramer,,,,,450.00,2025-02-01,,ben@example.com,\n" +
		",Carla,Diem,,,Wien,AT,300.00,2025-03-01,,carla@example.com,\n" +
		"id-new,Dora,Eder,,,Graz,AT,250.00,2025-04-01,,dora@example.com,\n" +
		"id-bad,Eli,Frank,,,,,200.00,not-a-date,,eli@example.com,\n"

	imp, err := categorizeAthleteCSV([]byte(csv), existing)
	if err != nil {
		t.Fatalf("categorize: %v", err)
	}
	if imp.Unchanged != 1 {
		t.Errorf("Unchanged = %d, want 1", imp.Unchanged)
	}
	if len(imp.Conflicts) != 1 {
		t.Fatalf("Conflicts = %d, want 1", len(imp.Conflicts))
	}
	if imp.Conflicts[0].Proposed.ID != "id-conflict" || imp.Conflicts[0].Proposed.MonthlyFee != 45000 {
		t.Errorf("conflict row = %+v", imp.Conflicts[0])
	}
	if imp.Conflicts[0].Current.MonthlyFee != 40000 {
		t.Errorf("current fee = %d, want 40000", imp.Conflicts[0].Current.MonthlyFee)
	}
	if len(imp.News) != 2 {
		t.Fatalf("News = %d, want 2", len(imp.News))
	}
	// Find the empty-id row and the id-new row.
	var sawEmpty, sawIDNew bool
	for _, n := range imp.News {
		if !n.HasID && n.Proposed.FirstName == "Carla" {
			sawEmpty = true
		}
		if n.HasID && n.Proposed.ID == "id-new" {
			sawIDNew = true
		}
	}
	if !sawEmpty || !sawIDNew {
		t.Errorf("News content unexpected: %+v", imp.News)
	}
	if len(imp.Errors) != 1 || imp.Errors[0].RowNum != 6 {
		t.Errorf("Errors = %+v, want one error on row 6", imp.Errors)
	}
}

func TestCategorizeAthleteCSV_SemicolonDelimiter(t *testing.T) {
	csv := "" +
		"id;first_name;last_name;street;postal_code;city;country;monthly_fee;start_date;end_date;email;notes\n" +
		";Carla;Diem;;;Wien;AT;300,00;2025-03-01;;carla@example.com;\n"
	imp, err := categorizeAthleteCSV([]byte(csv), nil)
	if err != nil {
		t.Fatalf("categorize: %v", err)
	}
	if len(imp.News) != 1 {
		t.Fatalf("News = %d, want 1", len(imp.News))
	}
	if imp.News[0].Proposed.MonthlyFee != 30000 {
		t.Errorf("fee = %d, want 30000 (comma decimal)", imp.News[0].Proposed.MonthlyFee)
	}
}

func TestCategorizeAthleteCSV_MissingColumn(t *testing.T) {
	csv := "id,first_name,last_name\nx,a,b\n"
	if _, err := categorizeAthleteCSV([]byte(csv), nil); err == nil {
		t.Fatal("expected error for missing required column")
	}
}
