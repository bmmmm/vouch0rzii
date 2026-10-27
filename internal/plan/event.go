// SPDX-License-Identifier: GPL-3.0-or-later

package plan

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
)

// LoadEvent reads and decodes an event file. Call Normalize before planning.
func LoadEvent(path string) (*Event, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	e, err := ParseEvent(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return e, nil
}

// ParseEvent decodes the JSON event format. Unknown fields are errors so that
// typos in hand-edited files do not pass silently.
func ParseEvent(b []byte) (*Event, error) {
	var e Event
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&e); err != nil {
		return nil, fmt.Errorf("event: %w", err)
	}
	return &e, nil
}

// MarshalEvent encodes an event as indented JSON, the canonical file format.
func MarshalEvent(e *Event) ([]byte, error) {
	b, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// Normalize validates an event in place: trims IDs, makes Knows symmetric and
// deduplicated, drops unknown or self references from Knows (warning). All
// hard problems are collected into one joined error so a form can show them at
// once.
func Normalize(e *Event) (warnings []string, err error) {
	var errs []error
	if strings.TrimSpace(e.Name) == "" {
		errs = append(errs, errors.New("event name missing"))
	}
	if e.Vouchers < 1 {
		errs = append(errs, fmt.Errorf("vouchers must be at least 1, got %d", e.Vouchers))
	}
	if e.PlusMax < 0 {
		errs = append(errs, fmt.Errorf("plusMax must not be negative, got %d", e.PlusMax))
	}
	if _, derr := ParseDate(strings.TrimSpace(e.Start)); derr != nil {
		errs = append(errs, fmt.Errorf("start must be YYYY-MM-DD, got %q", e.Start))
	} else {
		e.Start = strings.TrimSpace(e.Start)
	}
	if len(e.People) == 0 {
		errs = append(errs, errors.New("no people"))
	}

	plusMax := e.PlusMaxOrDefault()
	seen := map[string]int{} // lower-cased ID → index
	for i := range e.People {
		p := &e.People[i]
		p.ID = strings.TrimSpace(p.ID)
		p.Email = strings.TrimSpace(p.Email)
		switch {
		case p.ID == "":
			errs = append(errs, fmt.Errorf("person %d: empty id", i+1))
			continue
		case strings.ContainsAny(p.ID, "+\t\n;,"):
			errs = append(errs, fmt.Errorf("%s: id must not contain '+', ';', ',' or whitespace breaks", p.ID))
		}
		if j, dup := seen[strings.ToLower(p.ID)]; dup {
			errs = append(errs, fmt.Errorf("%s: duplicate id (rows %d and %d)", p.ID, j+1, i+1))
		} else {
			seen[strings.ToLower(p.ID)] = i
		}
		if p.Plus < 0 {
			errs = append(errs, fmt.Errorf("%s: plus must not be negative", p.ID))
		}
		if p.Plus > plusMax && !p.Group {
			errs = append(errs, fmt.Errorf("%s: +%d exceeds the limit of +%d; set group for an exception", p.ID, p.Plus, plusMax))
		}
		if p.Order < 0 {
			errs = append(errs, fmt.Errorf("%s: order must not be negative", p.ID))
		}
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	// Knows: resolve case-insensitively to canonical IDs, drop unknown and
	// self, then close symmetrically.
	known := make([]map[string]bool, len(e.People))
	for i := range known {
		known[i] = map[string]bool{}
	}
	for i := range e.People {
		p := &e.People[i]
		for _, raw := range p.Knows {
			id := strings.TrimSpace(raw)
			if id == "" {
				continue
			}
			j, ok := seen[strings.ToLower(id)]
			if !ok {
				warnings = append(warnings, fmt.Sprintf("%s: knows unknown handle %q, dropped", p.ID, id))
				continue
			}
			if j == i {
				continue
			}
			known[i][e.People[j].ID] = true
			known[j][p.ID] = true
		}
	}
	for i := range e.People {
		ids := make([]string, 0, len(known[i]))
		for id := range known[i] {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		e.People[i].Knows = ids
	}
	return warnings, nil
}

// Knows reports whether a and b know each other after Normalize.
func (e *Event) Knows(a, b string) bool {
	p := e.Person(a)
	if p == nil {
		return false
	}
	for _, id := range p.Knows {
		if id == b {
			return true
		}
	}
	return false
}
