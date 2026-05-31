package invoice

import (
	"testing"
	"time"

	"coachly/internal/store"
)

func TestBuildSingleMulti_ThreeContiguousMonths(t *testing.T) {
	s := newTestStore(t)
	specs := []LineSpec{
		{Year: 2026, Month: time.May},
		{Year: 2026, Month: time.June},
		{Year: 2026, Month: time.July},
	}
	counterBefore := s.Snapshot().Counter.NextInvoiceNumber

	draft, err := BuildSingleMulti(s, specs, "alice", "tipp")
	if err != nil {
		t.Fatalf("BuildSingleMulti: %v", err)
	}
	if len(draft.Lines) != 3 {
		t.Fatalf("Lines = %d, want 3", len(draft.Lines))
	}
	if draft.Total != 60000 {
		t.Errorf("Total = %d, want 60000 (3×20000)", draft.Total)
	}
	if draft.Lines[0].Month != "2026-05" || draft.Lines[2].Month != "2026-07" {
		t.Errorf("months = %s..%s", draft.Lines[0].Month, draft.Lines[2].Month)
	}
	if got := s.Snapshot().Counter.NextInvoiceNumber; got != counterBefore+1 {
		t.Errorf("counter = %d, want +1 (one number for whole invoice)", got)
	}
}

func TestBuildSingleMulti_RejectsDuplicate(t *testing.T) {
	s := newTestStore(t)
	specs := []LineSpec{
		{Year: 2026, Month: time.May},
		{Year: 2026, Month: time.May},
	}
	if _, err := BuildSingleMulti(s, specs, "alice", ""); err == nil {
		t.Fatal("expected error for duplicate months")
	}
	if got := len(s.Snapshot().Invoices); got != 0 {
		t.Errorf("invoices = %d, want 0 (no creation on validation failure)", got)
	}
}

func TestBuildSingleMulti_RejectsExistingMonth(t *testing.T) {
	s := newTestStore(t)
	if _, err := BuildBatchSingle(s, 2026, time.May, "alice", "v1"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	specs := []LineSpec{
		{Year: 2026, Month: time.May},
		{Year: 2026, Month: time.June},
	}
	if _, err := BuildSingleMulti(s, specs, "alice", ""); err != ErrInvoiceExists {
		t.Errorf("err = %v, want ErrInvoiceExists", err)
	}
}

func TestBuildSingleMulti_RejectsInactiveLine(t *testing.T) {
	s := newTestStore(t)
	// Alice starts 2026-01-01. April 2025 is before her start.
	specs := []LineSpec{
		{Year: 2025, Month: time.April},
		{Year: 2026, Month: time.May},
	}
	_, err := BuildSingleMulti(s, specs, "alice", "")
	if err == nil {
		t.Fatal("expected inactive-line error")
	}
}

func TestBuildBatch_SkipsAthletesCoveredByMultiMonth(t *testing.T) {
	s := newTestStore(t)
	// Multi-month invoice for alice covering May+June+July.
	specs := []LineSpec{
		{Year: 2026, Month: time.May},
		{Year: 2026, Month: time.June},
		{Year: 2026, Month: time.July},
	}
	if _, err := BuildSingleMulti(s, specs, "alice", ""); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Now run the monthly batch for June. Alice must be skipped — Bob
	// gets a new invoice.
	drafts, err := BuildBatch(s, 2026, time.June, TippInput{Default: "tipp"})
	if err != nil {
		t.Fatalf("BuildBatch: %v", err)
	}
	if len(drafts) != 1 || drafts[0].AthleteID != "bob" {
		t.Errorf("drafts = %+v, want only bob (alice covered by multi-month)", drafts)
	}
}

func TestConflictingMonths_ReportsCoveringInvoice(t *testing.T) {
	s := newTestStore(t)
	specs := []LineSpec{
		{Year: 2026, Month: time.May},
		{Year: 2026, Month: time.June},
	}
	if _, err := BuildSingleMulti(s, specs, "alice", ""); err != nil {
		t.Fatalf("seed: %v", err)
	}
	d := s.Snapshot()
	conflicts := ConflictingMonths(d, "alice", []string{"2026-05", "2026-07", "2026-06"})
	if len(conflicts) != 2 {
		t.Fatalf("conflicts = %d, want 2 (May+June covered, July not)", len(conflicts))
	}
	if conflicts[0].Month != "2026-05" || conflicts[1].Month != "2026-06" {
		t.Errorf("conflict months = %s, %s — want 2026-05, 2026-06", conflicts[0].Month, conflicts[1].Month)
	}
}

func TestRecreateSingleSameNumber_RefusesMultiMonth(t *testing.T) {
	s := newTestStore(t)
	specs := []LineSpec{
		{Year: 2026, Month: time.May},
		{Year: 2026, Month: time.June},
	}
	if _, err := BuildSingleMulti(s, specs, "alice", ""); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, _, err := RecreateSingleSameNumber(s, 2026, time.May, "alice", "v2"); err != ErrInvoiceExists {
		t.Errorf("err = %v, want ErrInvoiceExists (multi-month can't be recreated single-style)", err)
	}
}

func TestInvoiceCoversMonthAndYear(t *testing.T) {
	inv := store.Invoice{
		Lines: []store.InvoiceLine{
			{Month: "2026-05"},
			{Month: "2026-07"},
			{Month: "2027-01"},
		},
	}
	if !inv.CoversMonth("2026-05") || !inv.CoversMonth("2027-01") {
		t.Error("CoversMonth should be true for covered months")
	}
	if inv.CoversMonth("2026-06") {
		t.Error("CoversMonth should be false for gap month")
	}
	if !inv.CoversYear(2026) || !inv.CoversYear(2027) {
		t.Error("CoversYear should be true for 2026 and 2027")
	}
	if inv.CoversYear(2025) {
		t.Error("CoversYear should be false for 2025")
	}
}
