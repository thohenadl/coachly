package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestReplaceAll_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := New(filepath.Join(dir, "store.enc"))
	pw := []byte("correct-horse-battery-staple")

	if err := s.Create(pw); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Seed something so we can detect the overwrite.
	if err := s.Mutate(func(d *Data) error {
		d.Coach.FirstName = "before"
		d.Athletes = append(d.Athletes, Athlete{ID: "a1", FirstName: "Old"})
		return nil
	}); err != nil {
		t.Fatalf("mutate: %v", err)
	}

	// Build a totally different Data to replace with.
	replacement := NewData()
	replacement.Coach.FirstName = "after"
	now := time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)
	replacement.Athletes = []Athlete{
		{ID: "b1", FirstName: "New", LastName: "Person", MonthlyFee: 35000, StartDate: now},
	}
	replacement.Counter.NextInvoiceNumber = 2027001

	if err := s.ReplaceAll(replacement); err != nil {
		t.Fatalf("ReplaceAll: %v", err)
	}

	// In-memory snapshot should reflect the replacement.
	got := s.Snapshot()
	if got.Coach.FirstName != "after" {
		t.Errorf("Coach.FirstName = %q, want %q", got.Coach.FirstName, "after")
	}
	if len(got.Athletes) != 1 || got.Athletes[0].ID != "b1" {
		t.Errorf("Athletes = %+v, want single b1", got.Athletes)
	}
	if got.Counter.NextInvoiceNumber != 2027001 {
		t.Errorf("Counter = %d, want 2027001", got.Counter.NextInvoiceNumber)
	}

	// On-disk: lock and re-open with the same password — data must persist.
	s.Lock()
	s2 := New(filepath.Join(dir, "store.enc"))
	if err := s2.Unlock(pw); err != nil {
		t.Fatalf("re-unlock: %v", err)
	}
	got2 := s2.Snapshot()
	if got2.Coach.FirstName != "after" || len(got2.Athletes) != 1 {
		t.Errorf("after reopen got = %+v", got2)
	}
}

func TestReplaceAll_RequiresOpenStore(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "store.enc"))
	if err := s.ReplaceAll(NewData()); err == nil {
		t.Fatal("expected error when calling ReplaceAll on locked store")
	}
}
