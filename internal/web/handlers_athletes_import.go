package web

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/google/uuid"

	"coachly/internal/i18n"
	"coachly/internal/store"
)

// CSV column order. Header row is required and must match this exactly (in
// either order — we read header names, not positions).
var athleteCSVHeader = []string{
	"id", "first_name", "last_name", "street", "postal_code", "city", "country",
	"monthly_fee", "start_date", "end_date", "email", "notes",
}

// athleteImport holds the categorized result of a CSV upload, awaiting
// per-row decisions from the user.
type athleteImport struct {
	News      []athleteImportRow // ID empty or not in store → will be added
	Conflicts []athleteImportRow // ID exists; proposed differs from current
	Unchanged int                // ID exists; proposed equals current
	Errors    []athleteImportError
}

type athleteImportRow struct {
	RowNum    int           // 1-based, header counts as row 1
	Proposed  store.Athlete // ID populated for conflicts; empty for "new with empty id"
	Current   store.Athlete // only set for Conflicts
	HasID     bool          // false → we'll generate one at confirm time
}

type athleteImportError struct {
	RowNum int
	Reason string
}

func (s *Server) handleAthleteImportForm(w http.ResponseWriter, r *http.Request) {
	v := s.chrome("athletes", i18n.T("athletes.import.title"), "")
	v["Error"] = ""
	if k := r.URL.Query().Get("err"); k != "" {
		v["Error"] = i18n.T(k)
	}
	s.renderPage(w, "athletes_import.html", v)
}

func (s *Server) handleAthleteImportPreview(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Redirect(w, r, "/athletes/import?err=athletes.import.err_upload", http.StatusSeeOther)
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		http.Redirect(w, r, "/athletes/import?err=athletes.import.err_no_file", http.StatusSeeOther)
		return
	}
	defer file.Close()
	raw, err := io.ReadAll(file)
	if err != nil {
		http.Redirect(w, r, "/athletes/import?err=athletes.import.err_upload", http.StatusSeeOther)
		return
	}

	d := s.Store.Snapshot()
	imp, err := categorizeAthleteCSV(raw, d.Athletes)
	if err != nil {
		http.Redirect(w, r, "/athletes/import?err=athletes.import.err_parse", http.StatusSeeOther)
		return
	}

	token := uuid.NewString()
	s.mu.Lock()
	s.pendingAthleteImports[token] = imp
	s.mu.Unlock()

	v := s.chrome("athletes", i18n.T("athletes.import.preview_title"), "")
	v["Token"] = token
	v["News"] = imp.News
	v["Conflicts"] = imp.Conflicts
	v["Unchanged"] = imp.Unchanged
	v["Errors"] = imp.Errors
	s.renderPage(w, "athletes_import_preview.html", v)
}

func (s *Server) handleAthleteImportConfirm(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	token := r.FormValue("token")
	s.mu.Lock()
	imp, ok := s.pendingAthleteImports[token]
	if ok {
		delete(s.pendingAthleteImports, token)
	}
	s.mu.Unlock()
	if !ok {
		http.Redirect(w, r, "/athletes/import?err=athletes.import.err_expired", http.StatusSeeOther)
		return
	}

	// Collect per-conflict decisions.
	overwrite := map[string]bool{} // athlete ID → true if overwrite
	for _, c := range imp.Conflicts {
		if r.FormValue(fmt.Sprintf("decision_%d", c.RowNum)) == "overwrite" {
			overwrite[c.Proposed.ID] = true
		}
	}

	added := 0
	updated := 0
	if err := s.Store.Mutate(func(d *store.Data) error {
		// Apply new rows.
		for _, n := range imp.News {
			a := n.Proposed
			if !n.HasID || a.ID == "" {
				a.ID = uuid.NewString()
			}
			d.Athletes = append(d.Athletes, a)
			added++
		}
		// Apply conflict overwrites.
		for _, c := range imp.Conflicts {
			if !overwrite[c.Proposed.ID] {
				continue
			}
			for i := range d.Athletes {
				if d.Athletes[i].ID == c.Proposed.ID {
					d.Athletes[i] = c.Proposed
					updated++
					break
				}
			}
		}
		return nil
	}); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/athletes?imported=%d&updated=%d", added, updated), http.StatusSeeOther)
}

// categorizeAthleteCSV parses raw CSV bytes and buckets each data row into
// News / Conflicts / Unchanged / Errors against the existing athlete list.
// Pure function — no HTTP, no store. Tested directly.
func categorizeAthleteCSV(raw []byte, existing []store.Athlete) (*athleteImport, error) {
	// Sniff delimiter from header line: prefer comma, fall back to semicolon
	// (Excel-DE default). We sniff only the first line.
	delim := sniffDelimiter(raw)

	reader := csv.NewReader(bytes.NewReader(raw))
	reader.Comma = delim
	reader.FieldsPerRecord = -1 // tolerate variable lengths; we map by name
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("csv: %w", err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("csv: empty file")
	}
	headerIdx, err := indexHeader(records[0])
	if err != nil {
		return nil, err
	}

	byID := map[string]store.Athlete{}
	for _, a := range existing {
		byID[a.ID] = a
	}

	imp := &athleteImport{}
	for i, row := range records[1:] {
		rowNum := i + 2 // 1-based, +1 for header
		fields := map[string]string{}
		for name, idx := range headerIdx {
			if idx < len(row) {
				fields[name] = row[idx]
			}
		}
		// Skip fully-empty rows (trailing blank lines in spreadsheets).
		if allEmpty(fields) {
			continue
		}
		a, err := parseAthleteFields(fields)
		if err != nil {
			imp.Errors = append(imp.Errors, athleteImportError{RowNum: rowNum, Reason: err.Error()})
			continue
		}
		id := strings.TrimSpace(fields["id"])
		a.ID = id
		if id == "" {
			imp.News = append(imp.News, athleteImportRow{RowNum: rowNum, Proposed: a, HasID: false})
			continue
		}
		cur, exists := byID[id]
		if !exists {
			imp.News = append(imp.News, athleteImportRow{RowNum: rowNum, Proposed: a, HasID: true})
			continue
		}
		if athletesEqual(cur, a) {
			imp.Unchanged++
			continue
		}
		imp.Conflicts = append(imp.Conflicts, athleteImportRow{RowNum: rowNum, Proposed: a, Current: cur, HasID: true})
	}
	return imp, nil
}

func sniffDelimiter(raw []byte) rune {
	// Look at first line.
	nl := bytes.IndexByte(raw, '\n')
	head := raw
	if nl >= 0 {
		head = raw[:nl]
	}
	if bytes.Count(head, []byte{';'}) > bytes.Count(head, []byte{','}) {
		return ';'
	}
	return ','
}

func indexHeader(row []string) (map[string]int, error) {
	idx := map[string]int{}
	for i, name := range row {
		idx[strings.TrimSpace(strings.ToLower(name))] = i
	}
	// Require the essential columns.
	for _, required := range []string{"first_name", "last_name", "monthly_fee", "start_date"} {
		if _, ok := idx[required]; !ok {
			return nil, fmt.Errorf("csv: fehlende Spalte %q", required)
		}
	}
	return idx, nil
}

func allEmpty(f map[string]string) bool {
	for _, v := range f {
		if strings.TrimSpace(v) != "" {
			return false
		}
	}
	return true
}

func athletesEqual(a, b store.Athlete) bool {
	if a.ID != b.ID || a.FirstName != b.FirstName || a.LastName != b.LastName {
		return false
	}
	if a.Address != b.Address {
		return false
	}
	if a.MonthlyFee != b.MonthlyFee {
		return false
	}
	if !a.StartDate.Equal(b.StartDate) {
		return false
	}
	if (a.EndDate == nil) != (b.EndDate == nil) {
		return false
	}
	if a.EndDate != nil && !a.EndDate.Equal(*b.EndDate) {
		return false
	}
	if a.Email != b.Email || a.Notes != b.Notes {
		return false
	}
	return true
}
