// Package store loads / saves the encrypted JSON data file.
//
// Lifecycle: New() prepares a Store bound to a path. Open(password) decrypts
// (or initializes a fresh Data if no file exists). All mutations go through
// methods that take the store's lock and flush atomically via Save().
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"coachly/internal/auth"
)

type Store struct {
	path string

	mu   sync.Mutex
	data Data
	key  []byte // AES key for the current session
	salt []byte // salt used to derive key (reused on subsequent saves)
	open bool
}

func New(path string) *Store {
	return &Store{path: path}
}

func (s *Store) Path() string { return s.path }

func (s *Store) IsInitialized() bool {
	_, err := os.Stat(s.path)
	return err == nil
}

// Create writes a fresh, encrypted store with the given password.
// Used on first launch when no store file exists yet.
func (s *Store) Create(password []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	d := NewData()
	b, err := json.Marshal(d)
	if err != nil {
		return err
	}
	blob, err := auth.Seal(password, b)
	if err != nil {
		return err
	}
	if err := atomicWrite(s.path, blob); err != nil {
		return err
	}
	// Re-open to capture key+salt for the session.
	return s.unlock(password)
}

// Unlock decrypts the existing file with the given password.
func (s *Store) Unlock(password []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.unlock(password)
}

func (s *Store) unlock(password []byte) error {
	blob, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}
	pt, key, salt, err := auth.Open(password, blob)
	if err != nil {
		return err
	}
	var d Data
	if err := json.Unmarshal(pt, &d); err != nil {
		return fmt.Errorf("store: bad JSON: %w", err)
	}
	if err := migrate(&d); err != nil {
		return err
	}
	s.data = d
	s.key = key
	s.salt = salt
	s.open = true
	return nil
}

func (s *Store) Lock() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = Data{}
	for i := range s.key {
		s.key[i] = 0
	}
	s.key = nil
	s.salt = nil
	s.open = false
}

// Snapshot returns a deep-ish copy of current data for read-only use.
func (s *Store) Snapshot() Data {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data
}

// Mutate runs fn under the store lock and persists on success.
// fn should mutate the *Data in place.
func (s *Store) Mutate(fn func(*Data) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.open {
		return errors.New("store: locked")
	}
	if err := fn(&s.data); err != nil {
		return err
	}
	return s.flushLocked()
}

// ReplaceAll overwrites the entire in-memory Data with d and persists.
// Used by the plaintext-JSON import path. The current session key+salt are
// reused, so no password re-entry is required.
func (s *Store) ReplaceAll(d Data) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.open {
		return errors.New("store: locked")
	}
	if err := migrate(&d); err != nil {
		return err
	}
	s.data = d
	return s.flushLocked()
}

func (s *Store) flushLocked() error {
	b, err := json.Marshal(s.data)
	if err != nil {
		return err
	}
	blob, err := auth.SealWith(s.key, s.salt, b)
	if err != nil {
		return err
	}
	return atomicWrite(s.path, blob)
}

// ChangePassword re-derives a key from newPassword (with a fresh salt) and
// rewrites the file. Caller must hold no other store reference.
func (s *Store) ChangePassword(newPassword []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.open {
		return errors.New("store: locked")
	}
	b, err := json.Marshal(s.data)
	if err != nil {
		return err
	}
	blob, err := auth.Seal(newPassword, b)
	if err != nil {
		return err
	}
	if err := atomicWrite(s.path, blob); err != nil {
		return err
	}
	// Reload to refresh key/salt.
	return s.unlock(newPassword)
}

func atomicWrite(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// migrate upgrades older schema versions to the current one.
func migrate(d *Data) error {
	switch d.SchemaVersion {
	case 0:
		// Treat as v1 (legacy seed).
		d.SchemaVersion = 1
		fallthrough
	case 1:
		// v1 → v2: Invoice.DisplayNumber, Preferences.NumberingFormat,
		// SMTP.Security. Existing invoices keep their integer-as-string
		// identifier so historical printouts continue to round-trip.
		for i := range d.Invoices {
			if d.Invoices[i].DisplayNumber == "" {
				d.Invoices[i].DisplayNumber = strconv.Itoa(d.Invoices[i].Number)
			}
		}
		if d.Preferences.NumberingFormat == "" {
			d.Preferences.NumberingFormat = DefaultNumberingFormat
		}
		if d.SMTP.Security == "" {
			d.SMTP.Security = "auto"
		}
		d.SchemaVersion = 2
		fallthrough
	case 2:
		// v2 → v3: configurable email subject/body for invoice sends.
		// Seed with the German defaults so existing installs immediately
		// have a usable template.
		if d.EmailTemplate.Subject == "" {
			d.EmailTemplate.Subject = DefaultEmailSubject
		}
		if d.EmailTemplate.Body == "" {
			d.EmailTemplate.Body = DefaultEmailBody
		}
		d.SchemaVersion = 3
		fallthrough
	case 3:
		// v3 → v4: multi-month invoices. Each pre-v4 invoice carried scalar
		// Month/Description/Period/Amount/ProRata fields and was implicitly
		// one billable month. Fold those into a single-element Lines slice
		// and set Total = Amount. Clears the legacy fields so subsequent
		// saves omit them entirely.
		for i := range d.Invoices {
			inv := &d.Invoices[i]
			if len(inv.Lines) == 0 && inv.LegacyMonth != "" {
				inv.Lines = []InvoiceLine{{
					Month:       inv.LegacyMonth,
					PeriodFrom:  inv.LegacyPeriodFrom,
					PeriodTo:    inv.LegacyPeriodTo,
					Description: inv.LegacyDescription,
					Amount:      inv.LegacyAmount,
					ProRata:     inv.LegacyProRata,
				}}
				inv.Total = inv.LegacyAmount
			}
			inv.LegacyMonth = ""
			inv.LegacyDescription = ""
			inv.LegacyPeriodFrom = time.Time{}
			inv.LegacyPeriodTo = time.Time{}
			inv.LegacyAmount = 0
			inv.LegacyProRata = false
		}
		d.SchemaVersion = 4
		fallthrough
	case SchemaVersion:
		return nil
	default:
		return fmt.Errorf("store: unknown schema version %d", d.SchemaVersion)
	}
}
