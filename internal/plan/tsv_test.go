// SPDX-License-Identifier: GPL-3.0-or-later

package plan

import (
	"reflect"
	"strings"
	"testing"
)

const spreadsheetPaste = "Reihenfolge\tName\tKreditkarte Verfügbar\tMitglied / (Häufiger) Gast / Bekanntschaft\t+N (max 2)\tE-Mail\tKennt\tGruppe\tFarbe\n" +
	"1\talpha\tja\tMitglied\t+1\talpha@example.org\tbravo, charlie\tnein\tblau\n" +
	"\tbravo\tnein\tGast\t\tbravo@example.org\t\t\t\n" +
	"\n" +
	"\tcharlie\tx\tBekanntschaft\t2\t\talpha, bravo\tja\t\n"

func TestParseRosterSpreadsheetPaste(t *testing.T) {
	people, warnings, err := ParseRoster(spreadsheetPaste)
	if err != nil {
		t.Fatal(err)
	}
	want := []Person{
		{ID: "alpha", Order: 1, Tier: TierMember, Plus: 1, Email: "alpha@example.org"},
		{ID: "bravo", Tier: TierGuest, Email: "bravo@example.org"},
		{ID: "charlie", Tier: TierAcquaintance, Plus: 2, Group: true},
	}
	if !reflect.DeepEqual(people, want) {
		t.Fatalf("got %+v\nwant %+v", people, want)
	}
	// The old credit-card and Kennt columns and unknown columns are ignored, not errors.
	if len(warnings) != 3 || !strings.Contains(warnings[0], `"Kreditkarte Verfügbar" ignored`) || !strings.Contains(warnings[1], `"Kennt" ignored`) || !strings.Contains(warnings[2], `"Farbe" ignored`) {
		t.Fatalf("warnings %v", warnings)
	}
}

func TestParseRosterSemicolonBomAndMissingStatus(t *testing.T) {
	people, warnings, err := ParseRoster("\uFEFFName;E-Mail;+N\r\nalpha;a@example.org;1\r\nbravo;;\r\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(people) != 2 || people[0].Plus != 1 || people[0].Email != "a@example.org" || people[1].ID != "bravo" {
		t.Fatalf("got %+v", people)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "everyone counts as a member") {
		t.Fatalf("warnings %v", warnings)
	}
}

func TestParseRosterErrors(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", "empty"},
		{"Foo\tBar\nx\ty\n", "needs a Name column"},
		{"Name\tGruppe\nalpha\tvielleicht\n", "row 2: group"},
		{"Name\tStatus\nalpha\tking\n", "row 2: unknown status"},
		{"Name\t+N\nalpha\ttwo\n", "row 2: +N"},
		{"Name\tHandle\nalpha\talpha\n", "mean the same"},
		{"Name\n\n", "no rows"},
	}
	for _, tc := range cases {
		_, _, err := ParseRoster(tc.in)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%q: got %v want %q", tc.in, err, tc.want)
		}
	}
}

func TestRosterRoundTrip(t *testing.T) {
	in := []Person{
		{ID: "alpha", Order: 1, Tier: TierMember, Plus: 1, Email: "alpha@example.org", Note: "brings cake"},
		{ID: "bravo", Tier: TierGuest, Out: true},
		{ID: "mammut", Tier: TierAcquaintance, Plus: 10, Group: true},
	}
	tsv := FormatRoster(in)
	if want := "Order\tName\tStatus\t+N\tEmail\tGroup\tOut\tNote\n1\talpha\tmember\t1\talpha@example.org\tno\tno\tbrings cake\n"; !strings.HasPrefix(tsv, want) || !strings.Contains(tsv, "\tbravo\tguest\t\t\tno\tyes\t") {
		t.Fatalf("roster is written in English:\n%s", tsv)
	}
	out, warnings, err := ParseRoster(tsv)
	if err != nil || len(warnings) != 0 {
		t.Fatalf("err %v warnings %v", err, warnings)
	}
	if !reflect.DeepEqual(out, in) {
		t.Fatalf("round trip changed people:\n%+v\n%+v", out, in)
	}
}

func TestParseBool(t *testing.T) {
	for _, s := range []string{"ja", "Yes", "x", "1", "TRUE"} {
		if v, err := ParseBool(s); err != nil || !v {
			t.Errorf("%q should be true", s)
		}
	}
	for _, s := range []string{"", "nein", "No", "0", "-", "false"} {
		if v, err := ParseBool(s); err != nil || v {
			t.Errorf("%q should be false", s)
		}
	}
	if _, err := ParseBool("maybe"); err == nil {
		t.Error("maybe must fail")
	}
}

func TestRosterQuotesAreLiteralAndEmptyHeaderCellIgnored(t *testing.T) {
	in := []Person{{ID: "alpha", Note: `arriving "late"`}}
	out, _, err := ParseRoster(FormatRoster(in))
	if err != nil || len(out) != 1 || out[0].Note != in[0].Note {
		t.Fatalf("quotes must round-trip: %v %+v", err, out)
	}
	people, _, err := ParseRoster("\tName\tE-Mail\n1\talpha\ta@example.org\n")
	if err != nil || len(people) != 1 || people[0].ID != "alpha" || people[0].Email != "a@example.org" {
		t.Fatalf("empty first header cell must be ignored: %v %+v", err, people)
	}
}

func TestParseSignupByContentAndPosition(t *testing.T) {
	// Five fields in the form's order whose only problem is a yes/no word
	// are read by content; the yes/no words are reported, not guessed.
	people, warnings, err := ParseSignup("bananaaa ; hallo@babna.com ; mitglied ; no ; nein\n")
	if err != nil {
		t.Fatal(err)
	}
	want := Person{ID: "bananaaa", Email: "hallo@babna.com", Tier: TierMember}
	if len(people) != 1 || !reflect.DeepEqual(people[0], want) {
		t.Fatalf("got %+v, want %+v", people, want)
	}
	if len(warnings) != 3 || !strings.Contains(warnings[0], "bananaaa: fields are not in the form's order") || !strings.Contains(warnings[1], `bananaaa: "no" ignored, no field takes yes/no`) || !strings.Contains(warnings[2], `"nein" ignored`) {
		t.Fatalf("warnings %v", warnings)
	}
	// The form's five fields are read by position, a note is a note.
	people, warnings, err = ParseSignup("dodo; dodo@example.org; Gast; 1; vegan, no nuts\n\n")
	if err != nil || len(warnings) != 0 {
		t.Fatalf("form line: %v %v", err, warnings)
	}
	want = Person{ID: "dodo", Email: "dodo@example.org", Tier: TierGuest, Plus: 1, Note: "vegan, no nuts"}
	if !reflect.DeepEqual(people[0], want) {
		t.Fatalf("got %+v, want %+v", people[0], want)
	}
	// Five fields in the wrong order fall back to content with a warning.
	people, warnings, err = ParseSignup("emu; Bekanntschaft; emu@example.org; +2; brings cake along")
	if err != nil {
		t.Fatal(err)
	}
	want = Person{ID: "emu", Email: "emu@example.org", Tier: TierAcquaintance, Plus: 2, Note: "brings cake along"}
	if !reflect.DeepEqual(people[0], want) || len(warnings) != 1 || !strings.Contains(warnings[0], "emu: fields are not in the form's order") {
		t.Fatalf("got %+v warnings %v", people[0], warnings)
	}
	// Five fields in the form's order with a typo are errors, not guesses:
	// a misspelled status, a +N that is no number, even with a tier word elsewhere.
	for _, line := range []string{"anna; anna@x.de; Bekantschaft; 1; vegan", "anna; anna@x.de; Gast; two; vegan", "anna; anna@x.de; Mitgleid; 1; Gast", "anna; ; Gast; two; vegan"} {
		if _, _, err = ParseSignup(line); err == nil || !strings.Contains(err.Error(), "line 1: ") || strings.Contains(err.Error(), "\n") {
			t.Fatalf("%q: got %v", line, err)
		}
	}
	if _, _, err = ParseSignup("anna; anna@x.de; Gast; two; vegan"); !strings.Contains(err.Error(), `+N "two" is not a number`) {
		t.Fatalf("got %v", err)
	}
	// Five reordered fields without any status word are still read by content.
	people, warnings, err = ParseSignup("emu; 2; emu@example.org; alpha; cake")
	if err != nil || !reflect.DeepEqual(people[0], Person{ID: "emu", Email: "emu@example.org", Plus: 2, Note: "alpha cake"}) || len(warnings) != 2 || !strings.Contains(warnings[1], "counts as member") {
		t.Fatalf("got %+v %v %v", people, warnings, err)
	}
	// A second number is "already set", not a ja/nein word.
	_, warnings, _ = ParseSignup("fox; Gast; 2; 0")
	if len(warnings) != 1 || !strings.Contains(warnings[0], `"0" ignored, that field is already set`) {
		t.Fatalf("warnings %v", warnings)
	}
	// Empty status and +N by position are fine; the pasted header line, English or German, is skipped.
	for _, header := range []string{SignupFields, "Name; E-Mail; Status; +N; Notiz"} {
		people, warnings, err = ParseSignup(header + "\nxy; xy@example.org; ; ; ")
		if err != nil || len(warnings) != 0 || len(people) != 1 || !reflect.DeepEqual(people[0], Person{ID: "xy", Email: "xy@example.org"}) {
			t.Fatalf("%s: got %+v %v %v", header, people, warnings, err)
		}
	}
	// By content everything else is the note, in field order; bare commas are skipped.
	people, warnings, _ = ParseSignup("anna; Gast; panda, koala,; ,; vegan; ,")
	if people[0].Note != "panda, koala, vegan" || len(warnings) != 0 {
		t.Fatalf("got %+v warnings %v", people[0], warnings)
	}
	// Content mode without a status warns; "1" is a +N.
	people, warnings, _ = ParseSignup("fox; 1")
	if people[0].Plus != 1 || len(warnings) != 1 || !strings.Contains(warnings[0], "counts as member") {
		t.Fatalf("got %+v warnings %v", people[0], warnings)
	}
	// A second e-mail is not the address.
	people, warnings, _ = ParseSignup("gnu; Gast; a@example.org; b@example.org")
	if people[0].Email != "a@example.org" || people[0].Note != "" || len(warnings) != 1 || !strings.Contains(warnings[0], `gnu: "b@example.org" ignored`) {
		t.Fatalf("got %+v warnings %v", people[0], warnings)
	}
	// Errors name the line; blank lines do not count as people.
	if _, _, err = ParseSignup("\n; nomail@example.org\n"); err == nil || !strings.Contains(err.Error(), "line 2: name missing") {
		t.Fatalf("got %v", err)
	}
}
