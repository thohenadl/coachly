package store

import (
	"testing"
	"time"
)

func TestMigrateV3toV4_FoldsScalarsIntoLines(t *testing.T) {
	periodFrom := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	periodTo := time.Date(2026, 5, 31, 0, 0, 0, 0, time.UTC)
	d := &Data{
		SchemaVersion: 3,
		Invoices: []Invoice{
			{
				Number:            142,
				AthleteID:         "alice",
				LegacyMonth:       "2026-05",
				LegacyDescription: "Coaching Mai",
				LegacyPeriodFrom:  periodFrom,
				LegacyPeriodTo:    periodTo,
				LegacyAmount:      31000,
				LegacyProRata:     false,
				Status:            StatusIssued,
			},
		},
	}
	if err := migrate(d); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if d.SchemaVersion != SchemaVersion {
		t.Fatalf("SchemaVersion = %d, want %d", d.SchemaVersion, SchemaVersion)
	}
	inv := d.Invoices[0]
	if len(inv.Lines) != 1 {
		t.Fatalf("Lines = %d, want 1", len(inv.Lines))
	}
	if inv.Lines[0].Month != "2026-05" || inv.Lines[0].Amount != 31000 {
		t.Errorf("Line = %+v, want Month=2026-05 Amount=31000", inv.Lines[0])
	}
	if inv.Total != 31000 {
		t.Errorf("Total = %d, want 31000", inv.Total)
	}
	if inv.LegacyMonth != "" || inv.LegacyAmount != 0 {
		t.Errorf("legacy fields not cleared: %+v", inv)
	}
}

func TestMigrateV3toV4_IsIdempotentOnV4Data(t *testing.T) {
	d := &Data{
		SchemaVersion: 4,
		Invoices: []Invoice{
			{
				Number:    1,
				AthleteID: "alice",
				Lines:     []InvoiceLine{{Month: "2026-05", Amount: 20000}},
				Total:     20000,
				Status:    StatusIssued,
			},
		},
	}
	if err := migrate(d); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if len(d.Invoices[0].Lines) != 1 || d.Invoices[0].Total != 20000 {
		t.Errorf("re-migrated v4 data: %+v", d.Invoices[0])
	}
}
