package mailer

import (
	"bytes"
	"net/mail"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildMessage_HasExpectedHeadersAndParts(t *testing.T) {
	// Write a tiny "PDF" fixture (content doesn't need to be a real PDF for
	// this test — we just check it ends up base64-encoded in the attachment).
	dir := t.TempDir()
	pdfPath := filepath.Join(dir, "max_05_2026_2026042.pdf")
	if err := os.WriteFile(pdfPath, []byte("%PDF-1.4 fixture"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	msg, err := buildMessage("coach@example.com", "athlete@example.com",
		"Rechnung 2026042", "Hallo Athlet,\n\nanbei.\n", pdfPath)
	if err != nil {
		t.Fatalf("buildMessage: %v", err)
	}

	// Should be a parseable RFC 5322 message.
	m, err := mail.ReadMessage(bytes.NewReader(msg))
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	if m.Header.Get("From") != "coach@example.com" {
		t.Errorf("From = %q", m.Header.Get("From"))
	}
	if m.Header.Get("To") != "athlete@example.com" {
		t.Errorf("To = %q", m.Header.Get("To"))
	}
	if subj := m.Header.Get("Subject"); !strings.Contains(subj, "2026042") {
		t.Errorf("Subject = %q, want 2026042 in it", subj)
	}
	ct := m.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "multipart/mixed") {
		t.Errorf("Content-Type = %q, want multipart/mixed", ct)
	}

	body := string(msg)
	if !strings.Contains(body, "application/pdf") {
		t.Error("body missing application/pdf part header")
	}
	if !strings.Contains(body, `filename="max_05_2026_2026042.pdf"`) {
		t.Error("body missing PDF filename in Content-Disposition")
	}
	if !strings.Contains(body, "Content-Transfer-Encoding: base64") {
		t.Error("body missing base64 attachment encoding")
	}
	if !strings.Contains(body, "Content-Transfer-Encoding: quoted-printable") {
		t.Error("body missing quoted-printable for text part")
	}
}
