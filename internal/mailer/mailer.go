// Package mailer sends an invoice email with the PDF attached via SMTP.
//
// Implementation deliberately uses only the standard library so the single-
// binary distribution requirement (NFR-D-04) is not undermined by a fresh
// dependency. The MIME body is built by hand: a multipart/mixed envelope
// with a quoted-printable text/plain part and a base64-encoded
// application/pdf attachment.
//
// Transport selection: SMTP.Security picks the mode explicitly —
//   "ssl"      dials TLS directly (implicit TLS / SMTPS)
//   "starttls" dials plain and upgrades via STARTTLS (required)
//   "none"     plain TCP only, for local relays
//   "auto" or empty falls back to port-based routing (465 → ssl, else → starttls)
// for compatibility with v1 stores. AUTH PLAIN runs after the transport is
// established.
package mailer

import (
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/smtp"
	"net/textproto"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"coachly/internal/store"
)

// Timeouts: SMTP servers that go silent must not be allowed to hang the HTTP
// handler indefinitely (the standard `net/smtp` client sets no deadlines on
// its own). dialTimeout bounds connect; convTimeout is reset before every
// SMTP phase so the whole conversation stays under ~60s of total inactivity.
const (
	dialTimeout = 15 * time.Second
	convTimeout = 30 * time.Second
)

// Send delivers a multipart message to `to` with `bodyText` as the message
// body and the file at `attachmentPath` attached as application/pdf.
//
// Returns an error if cfg.Enabled is false or any required field is empty.
func Send(cfg store.SMTP, to, subject, bodyText, attachmentPath string) error {
	if !cfg.Enabled {
		return errors.New("smtp: nicht aktiviert")
	}
	if cfg.Host == "" || cfg.Port == 0 || cfg.From == "" {
		return errors.New("smtp: Host, Port und Absender müssen gesetzt sein")
	}
	if to == "" {
		return errors.New("smtp: keine Empfänger-Adresse")
	}

	msg, err := buildMessage(cfg.From, to, subject, bodyText, attachmentPath)
	if err != nil {
		return err
	}
	return send(cfg, to, msg)
}

// SendText delivers a plain-text-only message (no attachment). Used by the
// settings "send test" button so the user can verify SMTP without first
// having an issued invoice to ship.
func SendText(cfg store.SMTP, to, subject, bodyText string) error {
	if !cfg.Enabled {
		return errors.New("smtp: nicht aktiviert")
	}
	if cfg.Host == "" || cfg.Port == 0 || cfg.From == "" {
		return errors.New("smtp: Host, Port und Absender müssen gesetzt sein")
	}
	if to == "" {
		return errors.New("smtp: keine Empfänger-Adresse")
	}

	msg, err := buildPlainMessage(cfg.From, to, subject, bodyText)
	if err != nil {
		return err
	}
	return send(cfg, to, msg)
}

// deadlineConn lets the SMTP conversation refresh its deadline between
// commands so a slow but progressing server isn't killed mid-handshake.
type deadlineConn interface {
	net.Conn
	SetDeadline(time.Time) error
}

func send(cfg store.SMTP, to string, msg []byte) error {
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	tlsCfg := &tls.Config{ServerName: cfg.Host, MinVersion: tls.VersionTLS12}
	dialer := &net.Dialer{Timeout: dialTimeout}

	var (
		conn   deadlineConn
		client *smtp.Client
		err    error
	)
	mode := cfg.Security
	if mode == "" || mode == "auto" {
		if cfg.Port == 465 {
			mode = "ssl"
		} else {
			mode = "starttls"
		}
	}
	switch mode {
	case "ssl":
		tc, derr := tls.DialWithDialer(dialer, "tcp", addr, tlsCfg)
		if derr != nil {
			return fmt.Errorf("smtp: TLS-Verbindung fehlgeschlagen: %w", derr)
		}
		conn = tc
		_ = conn.SetDeadline(time.Now().Add(convTimeout))
		client, err = smtp.NewClient(conn, cfg.Host)
	case "starttls":
		c, derr := dialer.Dial("tcp", addr)
		if derr != nil {
			return fmt.Errorf("smtp: Verbindung fehlgeschlagen: %w", derr)
		}
		conn = c
		_ = conn.SetDeadline(time.Now().Add(convTimeout))
		client, err = smtp.NewClient(conn, cfg.Host)
		if err == nil {
			// STARTTLS is required: we never want to send credentials over
			// cleartext. If the server doesn't advertise it, abort with a
			// clear message instead of silently degrading.
			if ok, _ := client.Extension("STARTTLS"); !ok {
				client.Close()
				return fmt.Errorf("smtp: Server bietet kein STARTTLS auf Port %d an — bitte SSL/TLS oder einen anderen Port wählen", cfg.Port)
			}
			_ = conn.SetDeadline(time.Now().Add(convTimeout))
			if err := client.StartTLS(tlsCfg); err != nil {
				client.Close()
				return fmt.Errorf("smtp: STARTTLS fehlgeschlagen: %w", err)
			}
		}
	case "none":
		c, derr := dialer.Dial("tcp", addr)
		if derr != nil {
			return fmt.Errorf("smtp: Verbindung fehlgeschlagen: %w", derr)
		}
		conn = c
		_ = conn.SetDeadline(time.Now().Add(convTimeout))
		client, err = smtp.NewClient(conn, cfg.Host)
	default:
		return fmt.Errorf("smtp: unbekannter Sicherheitsmodus %q", mode)
	}
	if err != nil {
		if conn != nil {
			_ = conn.Close()
		}
		return fmt.Errorf("smtp: Verbindung fehlgeschlagen: %w", err)
	}
	defer client.Close()
	bump := func() { _ = conn.SetDeadline(time.Now().Add(convTimeout)) }

	if cfg.User != "" {
		bump()
		auth := smtp.PlainAuth("", cfg.User, cfg.Password, cfg.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("smtp: Authentifizierung fehlgeschlagen: %w", err)
		}
	}
	bump()
	if err := client.Mail(cfg.From); err != nil {
		return fmt.Errorf("smtp: MAIL FROM fehlgeschlagen: %w", err)
	}
	bump()
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp: RCPT TO fehlgeschlagen: %w", err)
	}
	bump()
	wc, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp: DATA fehlgeschlagen: %w", err)
	}
	if _, err := wc.Write(msg); err != nil {
		wc.Close()
		return fmt.Errorf("smtp: Schreiben fehlgeschlagen: %w", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("smtp: Abschluss fehlgeschlagen: %w", err)
	}
	bump()
	return client.Quit()
}

// buildMessage assembles the multipart/mixed RFC 5322 message.
func buildMessage(from, to, subject, body, attachmentPath string) ([]byte, error) {
	pdf, err := os.ReadFile(attachmentPath)
	if err != nil {
		return nil, fmt.Errorf("smtp: PDF lesen fehlgeschlagen: %w", err)
	}

	var buf strings.Builder
	mp := multipart.NewWriter(stringWriter{&buf})

	// Top-level headers.
	fmt.Fprintf(&buf, "From: %s\r\n", from)
	fmt.Fprintf(&buf, "To: %s\r\n", to)
	fmt.Fprintf(&buf, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", subject))
	fmt.Fprintf(&buf, "Date: %s\r\n", time.Now().UTC().Format(time.RFC1123Z))
	fmt.Fprintf(&buf, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&buf, "Content-Type: multipart/mixed; boundary=%q\r\n\r\n", mp.Boundary())

	// Text body part.
	textHdr := textproto.MIMEHeader{}
	textHdr.Set("Content-Type", "text/plain; charset=utf-8")
	textHdr.Set("Content-Transfer-Encoding", "quoted-printable")
	textPart, err := mp.CreatePart(textHdr)
	if err != nil {
		return nil, err
	}
	qp := quotedprintable.NewWriter(textPart)
	if _, err := io.WriteString(qp, body); err != nil {
		return nil, err
	}
	if err := qp.Close(); err != nil {
		return nil, err
	}

	// PDF attachment part.
	name := filepath.Base(attachmentPath)
	attHdr := textproto.MIMEHeader{}
	attHdr.Set("Content-Type", fmt.Sprintf("application/pdf; name=%q", name))
	attHdr.Set("Content-Transfer-Encoding", "base64")
	attHdr.Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", name))
	attPart, err := mp.CreatePart(attHdr)
	if err != nil {
		return nil, err
	}
	enc := base64.NewEncoder(base64.StdEncoding, &lineWrapWriter{w: attPart, width: 76})
	if _, err := enc.Write(pdf); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}

	if err := mp.Close(); err != nil {
		return nil, err
	}
	return []byte(buf.String()), nil
}

// buildPlainMessage assembles a simple text/plain RFC 5322 message (no
// attachment) used by the "send test" feature.
func buildPlainMessage(from, to, subject, body string) ([]byte, error) {
	var buf strings.Builder
	fmt.Fprintf(&buf, "From: %s\r\n", from)
	fmt.Fprintf(&buf, "To: %s\r\n", to)
	fmt.Fprintf(&buf, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", subject))
	fmt.Fprintf(&buf, "Date: %s\r\n", time.Now().UTC().Format(time.RFC1123Z))
	fmt.Fprintf(&buf, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&buf, "Content-Type: text/plain; charset=utf-8\r\n")
	fmt.Fprintf(&buf, "Content-Transfer-Encoding: quoted-printable\r\n\r\n")
	qp := quotedprintable.NewWriter(stringWriter{&buf})
	if _, err := io.WriteString(qp, body); err != nil {
		return nil, err
	}
	if err := qp.Close(); err != nil {
		return nil, err
	}
	return []byte(buf.String()), nil
}

// stringWriter adapts strings.Builder to io.Writer for multipart.NewWriter.
type stringWriter struct{ b *strings.Builder }

func (s stringWriter) Write(p []byte) (int, error) { return s.b.Write(p) }

// lineWrapWriter wraps base64 output at `width` columns so the resulting
// MIME body stays under the 998-character RFC 5322 line limit.
type lineWrapWriter struct {
	w     io.Writer
	width int
	col   int
}

func (l *lineWrapWriter) Write(p []byte) (int, error) {
	written := 0
	for len(p) > 0 {
		space := l.width - l.col
		if space <= 0 {
			if _, err := l.w.Write([]byte("\r\n")); err != nil {
				return written, err
			}
			l.col = 0
			space = l.width
		}
		chunk := p
		if len(chunk) > space {
			chunk = chunk[:space]
		}
		n, err := l.w.Write(chunk)
		written += n
		l.col += n
		p = p[n:]
		if err != nil {
			return written, err
		}
	}
	return written, nil
}
