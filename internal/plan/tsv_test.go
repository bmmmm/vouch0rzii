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
		{ID: "alpha", Order: 1, Card: true, Tier: TierMember, Plus: 1, Email: "alpha@example.org", Knows: []string{"bravo", "charlie"}},
		{ID: "bravo", Tier: TierGuest, Email: "bravo@example.org"},
		{ID: "charlie", Card: true, Tier: TierAcquaintance, Plus: 2, Knows: []string{"alpha", "bravo"}, Group: true},
	}
	if !reflect.DeepEqual(people, want) {
		t.Fatalf("got %+v\nwant %+v", people, want)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], `"Farbe" ignored`) {
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

func TestKnowsSplitsOnCommaOnly(t *testing.T) {
	people, _, err := ParseRoster("Name\tKennt\nMax Mustermann\tErika M, Bob/Alice\nErika M\t\nBob/Alice\t\n")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(people[0].Knows, []string{"Erika M", "Bob/Alice"}) {
		t.Fatalf("knows %v", people[0].Knows)
	}
}

func TestParseRosterErrors(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", "empty"},
		{"Foo\tBar\nx\ty\n", "needs a Name column"},
		{"Name\tKreditkarte\nalpha\tvielleicht\n", "row 2: card"},
		{"Name\tStatus\nalpha\tKönig\n", "row 2: unknown status"},
		{"Name\t+N\nalpha\tzwei\n", "row 2: +N"},
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
		{ID: "alpha", Order: 1, Card: true, Tier: TierMember, Plus: 1, Email: "alpha@example.org", Knows: []string{"bravo"}, Note: "bringt Kuchen"},
		{ID: "bravo", Tier: TierGuest, Out: true},
		{ID: "mammut", Tier: TierAcquaintance, Plus: 10, Group: true},
	}
	out, warnings, err := ParseRoster(FormatRoster(in))
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
	in := []Person{{ID: "alpha", Note: `komme "spät"`}}
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
	// A hand-typed line in its own order, as pasted by an organiser.
	people, warnings, err := ParseSignup("bananaaa ; hallo@babna.com ; mitglied ; no ; panda ; nein\n")
	if err != nil {
		t.Fatal(err)
	}
	want := Person{ID: "bananaaa", Email: "hallo@babna.com", Tier: TierMember, Knows: []string{"panda"}}
	if len(people) != 1 || !reflect.DeepEqual(people[0], want) {
		t.Fatalf("got %+v, want %+v", people, want)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], `bananaaa: "nein" ignored`) {
		t.Fatalf("warnings %v", warnings)
	}
	// The form's seven fields are read by position, a note is a note.
	people, warnings, err = ParseSignup("dodo; dodo@example.org; Gast; 1; ja; alpha, bravo; Veganerin\n\n")
	if err != nil || len(warnings) != 0 {
		t.Fatalf("form line: %v %v", err, warnings)
	}
	want = Person{ID: "dodo", Email: "dodo@example.org", Tier: TierGuest, Plus: 1, Card: true, Knows: []string{"alpha", "bravo"}, Note: "Veganerin"}
	if !reflect.DeepEqual(people[0], want) {
		t.Fatalf("got %+v, want %+v", people[0], want)
	}
	// Seven fields in the wrong order fall back to content with a warning.
	people, warnings, err = ParseSignup("emu; Bekanntschaft; emu@example.org; ja; +2; ; bringt Kuchen mit")
	if err != nil {
		t.Fatal(err)
	}
	want = Person{ID: "emu", Email: "emu@example.org", Tier: TierAcquaintance, Plus: 2, Card: true, Note: "bringt Kuchen mit"}
	if !reflect.DeepEqual(people[0], want) || len(warnings) != 1 || !strings.Contains(warnings[0], "emu: fields are not in the form's order") {
		t.Fatalf("got %+v warnings %v", people[0], warnings)
	}
	// Seven fields in the form's order with a typo are errors, not guesses.
	if _, _, err = ParseSignup("anna; anna@x.de; Bekantschaft; 1; ja; mammut; Veganerin"); err == nil || !strings.Contains(err.Error(), `line 1: unknown status "Bekantschaft"`) {
		t.Fatalf("got %v", err)
	}
	// Empty status and +N by position are fine; the pasted header line is skipped.
	people, warnings, err = ParseSignup("Name; E-Mail; Status; +N; Kreditkarte; Kennt; Notiz\nxy; xy@example.org; ; ; nein; ; ")
	if err != nil || len(warnings) != 0 || len(people) != 1 || !reflect.DeepEqual(people[0], Person{ID: "xy", Email: "xy@example.org"}) {
		t.Fatalf("got %+v %v %v", people, warnings, err)
	}
	// By content: the first handle field is Kennt, a trailing comma is fine, later words are the note.
	people, _, _ = ParseSignup("anna; Gast; panda, koala,; Veganerin")
	if !reflect.DeepEqual(people[0].Knows, []string{"panda", "koala"}) || people[0].Note != "Veganerin" {
		t.Fatalf("got %+v", people[0])
	}
	// Content mode without a status warns; "1" is a +N, not a card.
	people, warnings, _ = ParseSignup("fox; 1")
	if people[0].Plus != 1 || people[0].Card || len(warnings) != 1 || !strings.Contains(warnings[0], "counts as member") {
		t.Fatalf("got %+v warnings %v", people[0], warnings)
	}
	// A second e-mail is neither the address nor a connection.
	people, warnings, _ = ParseSignup("gnu; Gast; a@example.org; b@example.org")
	if people[0].Email != "a@example.org" || people[0].Knows != nil || len(warnings) != 1 || !strings.Contains(warnings[0], `gnu: "b@example.org" ignored`) {
		t.Fatalf("got %+v warnings %v", people[0], warnings)
	}
	// Errors name the line; blank lines do not count as people.
	if _, _, err = ParseSignup("\n; nomail@example.org\n"); err == nil || !strings.Contains(err.Error(), "line 2: name missing") {
		t.Fatalf("got %v", err)
	}
}
