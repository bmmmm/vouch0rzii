// SPDX-License-Identifier: GPL-3.0-or-later

package plan

import (
	"encoding/csv"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Roster columns. The header row is matched case-insensitively by the names
// below; the owner's long sheet headers ("Kreditkarte Verfügbar", "Mitglied /
// (Häufiger) Gast / Bekanntschaft", "+N (max 2 …)") match by their first word.
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
)

var columnAliases = map[string]int{
	"reihenfolge": colOrder, "order": colOrder,
	"name": colName, "handle": colName,
	"kreditkarte": colCard, "card": colCard,
	"status": colTier, "tier": colTier, "mitglied": colTier,
	"+n": colPlus, "plus": colPlus,
	"e-mail": colEmail, "email": colEmail, "mail": colEmail,
	"kennt": colKnows, "knows": colKnows,
	"gruppe": colGroup, "group": colGroup,
	"notiz": colNote, "note": colNote,
}

func columnOf(header string) (int, bool) {
	h := strings.ToLower(strings.TrimSpace(header))
	if c, ok := columnAliases[h]; ok {
		return c, true
	}
	if f := strings.Fields(h); len(f) > 1 {
		switch f[0] {
		case "kreditkarte", "mitglied", "+n":
			return columnAliases[f[0]], true
		}
	}
	return 0, false
}

// ParseBool accepts ja/nein, yes/no, x, 1/0, true/false; empty and "-" are
// false.
func ParseBool(s string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "ja", "j", "yes", "y", "x", "1", "true":
		return true, nil
	case "", "-", "nein", "n", "no", "0", "false":
		return false, nil
	}
	return false, fmt.Errorf("not a yes/no value: %q", s)
}

// ParseRoster parses a pasted spreadsheet: tab, semicolon or comma separated
// (decided by the header line), header row required with at least a name
// column. Unknown columns are ignored with a warning, a missing status column
// makes everyone a member with a warning. Hard errors are joined.
func ParseRoster(text string) (people []Person, warnings []string, err error) {
	text = strings.TrimSpace(strings.ReplaceAll(strings.TrimPrefix(text, "\uFEFF"), "\r\n", "\n"))
	if text == "" {
		return nil, nil, errors.New("roster is empty")
	}
	r := csv.NewReader(strings.NewReader(text))
	switch header, _, _ := strings.Cut(text, "\n"); {
	case strings.Contains(header, "\t"):
		r.Comma = '\t'
	case strings.Contains(header, ";"):
		r.Comma = ';'
	}
	r.FieldsPerRecord = -1
	records, rerr := r.ReadAll()
	if rerr != nil {
		return nil, nil, fmt.Errorf("roster: %w", rerr)
	}

	cols := map[int]int{} // column index → field
	used := map[int]string{}
	for i, h := range records[0] {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		c, ok := columnOf(h)
		if !ok {
			warnings = append(warnings, fmt.Sprintf("column %q ignored", h))
			continue
		}
		if first, dup := used[c]; dup {
			return nil, nil, fmt.Errorf("roster: columns %q and %q mean the same", first, h)
		}
		cols[i], used[c] = c, h
	}
	if _, ok := used[colName]; !ok {
		return nil, nil, errors.New("roster: header row needs a Name column")
	}
	if _, ok := used[colTier]; !ok {
		warnings = append(warnings, "no status column: everyone counts as a member")
	}

	var errs []error
	for n, rec := range records[1:] {
		row := n + 2
		var p Person
		blank := true
		for i, f := range rec {
			f = strings.TrimSpace(f)
			if f == "" {
				continue
			}
			blank = false
			c, ok := cols[i]
			if !ok {
				continue
			}
			switch c {
			case colOrder:
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
				t, ok := ParseTier(f)
				if !ok {
					errs = append(errs, fmt.Errorf("row %d: unknown status %q (Mitglied, Gast, Bekanntschaft)", row, f))
				}
				p.Tier = t
			case colPlus:
				v, perr := strconv.Atoi(strings.TrimPrefix(f, "+"))
				if perr != nil {
					errs = append(errs, fmt.Errorf("row %d: +N %q is not a number", row, f))
				}
				p.Plus = v
			case colEmail:
				p.Email = f
			case colKnows:
				for _, id := range strings.Split(f, ",") {
					if id = strings.TrimSpace(id); id != "" {
						p.Knows = append(p.Knows, id)
					}
				}
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
		if blank {
			continue
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

var tierGerman = [...]string{"Mitglied", "Gast", "Bekanntschaft"}

// RosterHeader is the header row FormatRoster writes; ParseRoster reads it back.
const RosterHeader = "Reihenfolge\tName\tKreditkarte\tStatus\t+N\tE-Mail\tKennt\tGruppe\tNotiz"

// FormatRoster renders people as TSV with German labels, the shape the page
// shows for editing. ParseRoster(FormatRoster(p)) yields p again as long as no
// field contains a tab or a line break.
func FormatRoster(people []Person) string {
	var b strings.Builder
	b.WriteString(RosterHeader)
	b.WriteByte('\n')
	yn := map[bool]string{true: "ja", false: "nein"}
	num := func(v int) string {
		if v > 0 {
			return strconv.Itoa(v)
		}
		return ""
	}
	for _, p := range people {
		tier := tierGerman[0]
		if p.Tier >= 0 && int(p.Tier) < len(tierGerman) {
			tier = tierGerman[p.Tier]
		}
		fmt.Fprintf(&b, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			num(p.Order), p.ID, yn[p.Card], tier, num(p.Plus), p.Email, strings.Join(p.Knows, ", "), yn[p.Group], p.Note)
	}
	return b.String()
}
