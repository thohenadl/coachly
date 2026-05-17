package invoice

import (
	"path/filepath"
	"testing"
	"time"

	"coachly/internal/store"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s := store.New(filepath.Join(t.TempDir(), "store.enc"))
	if err := s.Create([]byte("test-password-1234")); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := s.Mutate(func(d *store.Data) error {
		d.Athletes = []store.Athlete{
			{ID: "alice", FirstName: "Alice", MonthlyFee: 20000, StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
			{ID: "bob", FirstName: "Bob", MonthlyFee: 30000, StartDate: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		}
		d.Counter.NextInvoiceNumber = 2026001
		return nil
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return s
}

func TestIsEditable(t *testing.T) {
	cases := map[store.InvoiceStatus]bool{
		store.StatusPending: true,
		store.StatusIssued:  true,
		store.StatusSent:    false,
		store.StatusPaid:    false,
	}
	for st, want := range cases {
		if got := IsEditable(st); got != want {
			t.Errorf("IsEditable(%q) = %v, want %v", st, got, want)
		}
	}
}

func TestDeleteForMonth_SkipProtected(t *testing.T) {
	s := newTestStore(t)
	// Build a batch, then flip Alice → sent.
	if _, err := BuildBatch(s, 2026, time.May, TippInput{Default: "tipp"}); err != nil {
		t.Fatalf("BuildBatch: %v", err)
	}
	var aliceNum, bobNum int
	for _, inv := range s.Snapshot().Invoices {
		if inv.AthleteID == "alice" {
			aliceNum = inv.Number
		}
		if inv.AthleteID == "bob" {
			bobNum = inv.Number
		}
	}
	if err := SetStatus(s, aliceNum, store.StatusSent); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}

	removed, kept, err := DeleteForMonth(s, FormatMonth(2026, time.May), true)
	if err != nil {
		t.Fatalf("DeleteForMonth: %v", err)
	}
	if len(removed) != 0 {
		t.Errorf("no PDFs to remove yet, got %v", removed)
	}
	if len(kept) != 1 || kept[0] != aliceNum {
		t.Errorf("kept = %v, want [%d]", kept, aliceNum)
	}
	// Verify: alice still in store, bob gone.
	survivors := s.Snapshot().Invoices
	if len(survivors) != 1 || survivors[0].Number != aliceNum {
		t.Fatalf("survivors = %+v, want only alice (#%d)", survivors, aliceNum)
	}
	_ = bobNum
}

func TestRebuildBatchSameNumbers_PreservesNumbersAndSkipsProtected(t *testing.T) {
	s := newTestStore(t)
	if _, err := BuildBatch(s, 2026, time.May, TippInput{Default: "v1"}); err != nil {
		t.Fatalf("BuildBatch: %v", err)
	}
	// Capture original numbers; protect alice.
	original := map[string]int{}
	for _, inv := range s.Snapshot().Invoices {
		original[inv.AthleteID] = inv.Number
	}
	if err := SetStatus(s, original["alice"], store.StatusSent); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}
	counterBefore := s.Snapshot().Counter.NextInvoiceNumber

	drafts, _, skipped, err := RebuildBatchSameNumbers(s, 2026, time.May, TippInput{Default: "v2"})
	if err != nil {
		t.Fatalf("RebuildBatchSameNumbers: %v", err)
	}
	if len(drafts) != 1 || drafts[0].AthleteID != "bob" {
		t.Fatalf("drafts = %+v, want only bob", drafts)
	}
	if drafts[0].Number != original["bob"] {
		t.Errorf("bob number = %d, want preserved %d", drafts[0].Number, original["bob"])
	}
	if drafts[0].Tipp != "v2" {
		t.Errorf("draft tipp = %q, want v2 (updated)", drafts[0].Tipp)
	}
	if len(skipped) != 1 || skipped[0] != original["alice"] {
		t.Errorf("skipped = %v, want [alice=%d]", skipped, original["alice"])
	}
	if got := s.Snapshot().Counter.NextInvoiceNumber; got != counterBefore {
		t.Errorf("counter advanced from %d to %d; should be unchanged when reusing", counterBefore, got)
	}
}

func TestBuildBatchSingle_ConsumesNumberAndRefusesDuplicate(t *testing.T) {
	s := newTestStore(t)
	counterBefore := s.Snapshot().Counter.NextInvoiceNumber

	draft, err := BuildBatchSingle(s, 2026, time.May, "alice", "hello")
	if err != nil {
		t.Fatalf("BuildBatchSingle: %v", err)
	}
	if draft.Number != counterBefore {
		t.Errorf("draft.Number = %d, want %d", draft.Number, counterBefore)
	}
	if got := s.Snapshot().Counter.NextInvoiceNumber; got != counterBefore+1 {
		t.Errorf("counter = %d, want %d", got, counterBefore+1)
	}
	if draft.Status != store.StatusPending {
		t.Errorf("status = %q, want %q", draft.Status, store.StatusPending)
	}

	// Second call for same athlete/month must refuse.
	if _, err := BuildBatchSingle(s, 2026, time.May, "alice", "again"); err != ErrInvoiceExists {
		t.Errorf("second call: err = %v, want ErrInvoiceExists", err)
	}
	// Counter must not have moved.
	if got := s.Snapshot().Counter.NextInvoiceNumber; got != counterBefore+1 {
		t.Errorf("counter after refused call = %d, want %d", got, counterBefore+1)
	}
}

func TestRecreateSingleSameNumber_PreservesNumber(t *testing.T) {
	s := newTestStore(t)
	if _, err := BuildBatchSingle(s, 2026, time.May, "alice", "v1"); err != nil {
		t.Fatalf("BuildBatchSingle: %v", err)
	}
	orig := s.Snapshot().Invoices[0]
	counterBefore := s.Snapshot().Counter.NextInvoiceNumber

	draft, _, err := RecreateSingleSameNumber(s, 2026, time.May, "alice", "v2")
	if err != nil {
		t.Fatalf("RecreateSingleSameNumber: %v", err)
	}
	if draft.Number != orig.Number {
		t.Errorf("draft.Number = %d, want %d", draft.Number, orig.Number)
	}
	if draft.Tipp != "v2" {
		t.Errorf("tipp = %q, want v2", draft.Tipp)
	}
	if got := s.Snapshot().Counter.NextInvoiceNumber; got != counterBefore {
		t.Errorf("counter advanced; should stay at %d, got %d", counterBefore, got)
	}

	// Protected: flip to sent, then try again — must refuse.
	if err := SetStatus(s, draft.Number, store.StatusSent); err != nil {
		t.Fatalf("SetStatus: %v", err)
	}
	if _, _, err := RecreateSingleSameNumber(s, 2026, time.May, "alice", "v3"); err != ErrInvoiceExists {
		t.Errorf("expected ErrInvoiceExists for protected invoice, got %v", err)
	}
}
