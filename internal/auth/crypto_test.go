package auth

import (
	"bytes"
	"testing"
)

func TestSealOpenRoundTrip(t *testing.T) {
	pt := []byte(`{"hello":"welt","iban":"AT00 0000 0000 0000 0000"}`)
	pw := []byte("correct-horse-battery-staple")

	blob, err := Seal(pw, pt)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(blob, []byte("iban")) {
		t.Fatal("plaintext leaked into ciphertext blob")
	}

	got, _, _, err := Open(pw, blob)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, pt) {
		t.Fatalf("round-trip mismatch: got %s want %s", got, pt)
	}
}

func TestOpenBadPassword(t *testing.T) {
	blob, err := Seal([]byte("right"), []byte("data"))
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := Open([]byte("wrong"), blob); err != ErrBadPassword {
		t.Fatalf("expected ErrBadPassword, got %v", err)
	}
}
