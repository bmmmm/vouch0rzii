// SPDX-License-Identifier: GPL-3.0-or-later

package plan

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Roster columns. The header row is matched case-insensitively by the names
// below; the owner's long sheet headers ("Mitglied / (Häufiger) Gast /
// Bekanntschaft", "+N (max 2 …)") match by their first word. Columns nobody
// asked for (a credit-card column, colours) are ignored with a warning.
const (
	colOrder = iota
	colName
	colTier
	colPlus
	colEmail
	colGroup
	colOut
	colNote
)

var columnAliases = map[string]int{
	"reihenfolge": colOrder, "order": colOrder,
	"name": colName, "handle": colName,
	"status": colTier, "tier": colTier, "mitglied": colTier,
	"+n": colPlus, "plus": colPlus,
	"e-mail": colEmail, "email": colEmail, "mail": colEmail,
	"gruppe": colGroup, "group": colGroup,
	"ausgestiegen": colOut, "raus": colOut, "out": colOut, "left": colOut,
	"notiz": colNote, "note": colNote,
}

func columnOf(header string) (int, bool) {
	h := strings.ToLower(strings.TrimSpace(header))
	if c, ok := columnAliases[h]; ok {
		return c, true
	}
	if f := strings.Fields(h); len(f) > 1 {
		switch f[0] {
		case "mitglied", "+n":
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

// ParseRoster parses a pasted spreadsheet: one person per line, fields
// separated by tab, semicolon or comma (decided by the header line), header
// row required with at least a name column. Quotes are literal; a field must
// not contain the separator or a line break. Unknown columns are ignored with
// a warning, a missing status column makes everyone a member with a warning.
// Hard errors are joined.
func ParseRoster(text string) (people []Person, warnings []string, err error) {
	lines := strings.Split(strings.ReplaceAll(strings.TrimPrefix(text, "\uFEFF"), "\r\n", "\n"), "\n")
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	if len(lines) == 0 {
		return nil, nil, errors.New("roster is empty")
	}
	sep := ","
	switch {
	case strings.Contains(lines[0], "\t"):
		sep = "\t"
	case strings.Contains(lines[0], ";"):
		sep = ";"
	}
	records := make([][]string, len(lines))
	for i, l := range lines {
		records[i] = strings.Split(l, sep)
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
	yn := func(row int, what, f string) bool {
		v, perr := ParseBool(f)
		if perr != nil {
			errs = append(errs, fmt.Errorf("row %d: %s: %v", row, what, perr))
		}
		return v
	}
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
			case colTier:
				t, ok := ParseTier(f)
				if !ok {
					errs = append(errs, fmt.Errorf("row %d: unknown status %q (member, guest, acquaintance)", row, f))
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
			case colGroup:
				p.Group = yn(row, "group", f)
			case colOut:
				p.Out = yn(row, "out", f)
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

// RosterHeader is the header row FormatRoster writes; ParseRoster reads it back.
const RosterHeader = "Order\tName\tStatus\t+N\tEmail\tGroup\tOut\tNote"

// FormatRoster renders people as TSV with English labels, the shape the page
// shows for editing. ParseRoster reads it back with values trimmed and Plus
// or Order below 1 as 0; a tab or line break inside a field does not
// survive, and no people at all do not parse back.
func FormatRoster(people []Person) string {
	var b strings.Builder
	b.WriteString(RosterHeader)
	b.WriteByte('\n')
	yn := map[bool]string{true: "yes", false: "no"}
	num := func(v int) string {
		if v > 0 {
			return strconv.Itoa(v)
		}
		return ""
	}
	for _, p := range people {
		fmt.Fprintf(&b, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			num(p.Order), p.ID, p.Tier, num(p.Plus), p.Email, yn[p.Group], yn[p.Out], p.Note)
	}
	return b.String()
}

// SignupFields is the order in which the sign-up form writes a line.
const SignupFields = "Name; Email; Status; +N; Note"

// ParseSignup reads sign-up lines, one person per line, fields separated by
// ";". A line with the form's five fields is read by position (SignupFields);
// if that fails and the line is in the form's order (an address or nothing
// in position 2), its errors stand, except when only yes/no words are in the
// way. Any other line is read by content: the first field is the handle, "@"
// marks the e-mail, a tier word the status, a number the +N; a second one of
// these and yes/no words are ignored with a warning, anything else but bare
// commas is the note.
func ParseSignup(text string) (people []Person, warnings []string, err error) {
	lines := strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	var errs []error
	for n, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		row := n + 1
		fields := strings.Split(line, ";")
		for i := range fields {
			fields[i] = strings.TrimSpace(fields[i])
		}
		if fields[0] == "" {
			errs = append(errs, fmt.Errorf("line %d: name missing", row))
			continue
		}
		if isSignupHeader(fields) {
			continue // the header line from the hint, pasted along
		}
		if len(fields) == 5 {
			p, perrs, bad := signupByPosition(row, fields)
			if len(perrs) == 0 {
				people = append(people, p)
				continue
			}
			// An address in position 2 means the form's order: its errors
			// stand, unless the offending values are only yes/no words.
			// Anything else in position 2 is a reordered line.
			formOrder := fields[1] == "" || strings.Contains(fields[1], "@")
			leftovers := true
			for _, i := range bad {
				leftovers = leftovers && ParseBoolOK(fields[i])
			}
			if formOrder && !leftovers {
				errs = append(errs, perrs...)
				continue
			}
			warnings = append(warnings, fmt.Sprintf("line %d: %s: fields are not in the form's order, read by content", row, fields[0]))
		}
		p, more := signupByContent(row, fields)
		people = append(people, p)
		warnings = append(warnings, more...)
	}
	if len(errs) > 0 {
		return nil, warnings, errors.Join(errs...)
	}
	return people, warnings, nil
}

// isSignupHeader reports whether f is the header line from the hint.
func isSignupHeader(f []string) bool {
	return len(f) > 1 && strings.EqualFold(strings.TrimSpace(f[0]), "Name") && isEmailHeader(f[1])
}

// isEmailHeader matches the e-mail column of the hint, English or German.
func isEmailHeader(s string) bool {
	s = strings.TrimSpace(s)
	return strings.EqualFold(s, "Email") || strings.EqualFold(s, "E-Mail")
}

// ParseBoolOK reports whether ParseBool accepts s.
func ParseBoolOK(s string) bool {
	_, err := ParseBool(s)
	return err == nil
}

// signupByPosition reads the form's five fields; bad lists the indexes of the
// fields that did not parse.
func signupByPosition(row int, f []string) (p Person, errs []error, bad []int) {
	p = Person{ID: f[0], Email: f[1], Note: f[4]}
	if f[2] != "" {
		t, ok := ParseTier(f[2])
		if !ok {
			errs = append(errs, fmt.Errorf("line %d: unknown status %q", row, f[2]))
			bad = append(bad, 2)
		}
		p.Tier = t
	}
	if f[3] != "" {
		v, perr := strconv.Atoi(f[3]) // Atoi accepts "+2"
		if perr != nil {
			errs = append(errs, fmt.Errorf("line %d: +N %q is not a number", row, f[3]))
			bad = append(bad, 3)
		}
		p.Plus = v
	}
	return p, errs, bad
}

// signupByContent reads a line by content.
func signupByContent(row int, f []string) (p Person, warnings []string) {
	p = Person{ID: f[0]}
	var tier, plus bool
	for _, v := range f[1:] {
		t, isTier := ParseTier(v)
		n, nerr := strconv.Atoi(v)
		switch {
		case strings.Trim(v, ", ") == "": // nothing, or only commas
		case strings.Contains(v, "@") && p.Email == "":
			p.Email = v
		case isTier && !tier:
			p.Tier, tier = t, true
		case nerr == nil && !plus:
			p.Plus, plus = n, true
		case strings.Contains(v, "@") || isTier || nerr == nil:
			warnings = append(warnings, fmt.Sprintf("line %d: %s: %q ignored, that field is already set", row, p.ID, v))
		case ParseBoolOK(v):
			warnings = append(warnings, fmt.Sprintf("line %d: %s: %q ignored, no field takes yes/no", row, p.ID, v))
		default:
			p.Note = strings.TrimSpace(p.Note + " " + v)
		}
	}
	if !tier {
		warnings = append(warnings, fmt.Sprintf("line %d: %s: no status given, counts as member", row, p.ID))
	}
	return p, warnings
}
