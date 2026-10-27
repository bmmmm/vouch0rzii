// SPDX-License-Identifier: GPL-3.0-or-later

package plan

import (
	"encoding/csv"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Roster columns. The header row is matched by alias, case-insensitively, so a
// spreadsheet export in German or English can be pasted as is.
const (
	colOrder = iota
	colName
	colCard
	colTier
	colPlus
	colEmail
	colKnows
	colGroup
	colNote
	colIgnore
)

var columnAliases = map[string]int{
	"reihenfolge": colOrder, "order": colOrder, "nr": colOrder, "nr.": colOrder, "#": colOrder, "pos": colOrder, "position": colOrder,
	"name": colName, "handle": colName, "id": colName, "nick": colName, "nickname": colName,
	"kreditkarte": colCard, "card": colCard, "credit card": colCard, "creditcard": colCard, "cc": colCard, "kk": colCard,
	"status": colTier, "tier": colTier, "typ": colTier, "type": colTier, "rolle": colTier, "role": colTier,
	"+n": colPlus, "plus": colPlus, "+": colPlus, "n": colPlus, "begleitung": colPlus, "zusätzlich": colPlus, "extra": colPlus,
	"e-mail": colEmail, "email": colEmail, "mail": colEmail, "e-mail-adresse": colEmail,
	"kennt": colKnows, "knows": colKnows, "connections": colKnows, "connection": colKnows, "has connection to": colKnows, "kontakte": colKnows, "kontakt": colKnows,
	"gruppe": colGroup, "group": colGroup, "ausnahme": colGroup, "exception": colGroup,
	"notiz": colNote, "note": colNote, "bemerkung": colNote, "kommentar": colNote, "comment": colNote,
}

// columnOf resolves a header cell. Long German headers such as
// "Mitglied / (Häufiger) Gast / Bekanntschaft" or "Kreditkarte verfügbar" are
// recognised by their leading word.
func columnOf(header string) (int, bool) {
	h := strings.ToLower(strings.TrimSpace(header))
	if c, ok := columnAliases[h]; ok {
		return c, true
	}
	switch {
	case strings.HasPrefix(h, "mitglied"), strings.HasPrefix(h, "status"), strings.HasPrefix(h, "member"):
		return colTier, true
	case strings.HasPrefix(h, "kreditkarte"), strings.HasPrefix(h, "credit"):
		return colCard, true
	case strings.HasPrefix(h, "+n"), strings.HasPrefix(h, "plus"):
		return colPlus, true
	case strings.Contains(h, "mail"):
		return colEmail, true
	case strings.HasPrefix(h, "kennt"), strings.HasPrefix(h, "knows"), strings.Contains(h, "connection"):
		return colKnows, true
	case strings.HasPrefix(h, "gruppe"), strings.HasPrefix(h, "group"):
		return colGroup, true
	case strings.HasPrefix(h, "reihenfolge"), strings.HasPrefix(h, "order"):
		return colOrder, true
	case strings.HasPrefix(h, "name"):
		return colName, true
	case strings.HasPrefix(h, "notiz"), strings.HasPrefix(h, "note"), strings.HasPrefix(h, "bemerkung"):
		return colNote, true
	}
	return colIgnore, false
}

// ParseBool accepts ja/nein, yes/no, x, 1/0, true/false; empty and "-" are
// false.
func ParseBool(s string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "ja", "j", "yes", "y", "x", "1", "true", "wahr", "✓", "✔":
		return true, nil
	case "", "-", "nein", "n", "no", "0", "false", "falsch":
		return false, nil
	}
	return false, fmt.Errorf("not a yes/no value: %q", s)
}

// detectDelimiter picks tab, semicolon or comma from the header line.
func detectDelimiter(line string) rune {
	switch {
	case strings.Contains(line, "\t"):
		return '\t'
	case strings.Contains(line, ";"):
		return ';'
	}
	return ','
}

// ParseRoster parses a pasted spreadsheet (TSV, CSV with ';' or ','). The
// first non-empty line is the header and must name at least a name column.
// Unknown columns are ignored with a warning, a missing status column makes
// everyone a member with a warning. Hard errors are joined.
func ParseRoster(text string) (people []Person, warnings []string, err error) {
	text = strings.ReplaceAll(strings.TrimPrefix(text, "\uFEFF"), "\r\n", "\n")
	lines := strings.Split(text, "\n")
	first := -1
	for i, l := range lines {
		if strings.TrimSpace(l) != "" {
			first = i
			break
		}
	}
	if first < 0 {
		return nil, nil, errors.New("roster is empty")
	}
	r := csv.NewReader(strings.NewReader(strings.Join(lines[first:], "\n")))
	r.Comma = detectDelimiter(lines[first])
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	records, rerr := r.ReadAll()
	if rerr != nil {
		return nil, nil, fmt.Errorf("roster: %w", rerr)
	}

	cols := map[int]int{} // column index → field
	hasName, hasTier := false, false
	for i, h := range records[0] {
		if strings.TrimSpace(h) == "" {
			continue
		}
		c, ok := columnOf(h)
		if !ok {
			warnings = append(warnings, fmt.Sprintf("column %q ignored", strings.TrimSpace(h)))
			continue
		}
		if _, dup := cols[i]; dup {
			continue
		}
		for _, used := range cols {
			if used == c {
				return nil, nil, fmt.Errorf("roster: column %q appears twice", strings.TrimSpace(h))
			}
		}
		cols[i] = c
		hasName = hasName || c == colName
		hasTier = hasTier || c == colTier
	}
	if !hasName {
		return nil, nil, errors.New("roster: header row needs a Name column")
	}
	if !hasTier {
		warnings = append(warnings, "no status column: everyone counts as a member")
	}

	var errs []error
	for n, rec := range records[1:] {
		row := n + 2
		empty := true
		for _, f := range rec {
			if strings.TrimSpace(f) != "" {
				empty = false
				break
			}
		}
		if empty {
			continue
		}
		var p Person
		for i, f := range rec {
			c, ok := cols[i]
			if !ok {
				continue
			}
			f = strings.TrimSpace(f)
			switch c {
			case colOrder:
				if f == "" {
					continue
				}
				v, perr := strconv.Atoi(f)
				if perr != nil {
					errs = append(errs, fmt.Errorf("row %d: order %q is not a number", row, f))
				}
				p.Order = v
			case colName:
				p.ID = f
			case colCard:
				v, perr := ParseBool(f)
				if perr != nil {
					errs = append(errs, fmt.Errorf("row %d: card: %v", row, perr))
				}
				p.Card = v
			case colTier:
				if f == "" {
					continue
				}
				t, ok := ParseTier(f)
				if !ok {
					errs = append(errs, fmt.Errorf("row %d: unknown status %q (Mitglied, Gast, Bekanntschaft)", row, f))
				}
				p.Tier = t
			case colPlus:
				if f == "" {
					continue
				}
				v, perr := strconv.Atoi(strings.TrimPrefix(f, "+"))
				if perr != nil {
					errs = append(errs, fmt.Errorf("row %d: +N %q is not a number", row, f))
				}
				p.Plus = v
			case colEmail:
				p.Email = f
			case colKnows:
				p.Knows = splitList(f)
			case colGroup:
				v, perr := ParseBool(f)
				if perr != nil {
					errs = append(errs, fmt.Errorf("row %d: group: %v", row, perr))
				}
				p.Group = v
			case colNote:
				p.Note = f
			}
		}
		if p.ID == "" {
			errs = append(errs, fmt.Errorf("row %d: name missing", row))
			continue
		}
		people = append(people, p)
	}
	if len(errs) > 0 {
		return nil, warnings, errors.Join(errs...)
	}
	if len(people) == 0 {
		return nil, warnings, errors.New("roster: no rows below the header")
	}
	return people, warnings, nil
}

// splitList splits "a, b / c" style lists.
func splitList(s string) []string {
	f := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || r == '/' || r == ' ' || r == '|' })
	out := f[:0]
	for _, x := range f {
		if x = strings.TrimSpace(x); x != "" {
			out = append(out, x)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

var tierGerman = [...]string{"Mitglied", "Gast", "Bekanntschaft"}

// RosterHeader is the header row FormatRoster writes; ParseRoster reads it back.
const RosterHeader = "Reihenfolge\tName\tKreditkarte\tStatus\t+N\tE-Mail\tKennt\tGruppe\tNotiz"

// FormatRoster renders people as TSV with German labels, the shape the page
// shows for editing. ParseRoster(FormatRoster(p)) yields p again.
func FormatRoster(people []Person) string {
	var b strings.Builder
	b.WriteString(RosterHeader)
	b.WriteByte('\n')
	yn := func(v bool) string {
		if v {
			return "ja"
		}
		return "nein"
	}
	for _, p := range people {
		order := ""
		if p.Order > 0 {
			order = strconv.Itoa(p.Order)
		}
		plus := ""
		if p.Plus > 0 {
			plus = strconv.Itoa(p.Plus)
		}
		tier := tierGerman[0]
		if int(p.Tier) < len(tierGerman) && p.Tier >= 0 {
			tier = tierGerman[p.Tier]
		}
		fmt.Fprintf(&b, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			order, p.ID, yn(p.Card), tier, plus, p.Email, strings.Join(p.Knows, ", "), yn(p.Group), p.Note)
	}
	return b.String()
}
