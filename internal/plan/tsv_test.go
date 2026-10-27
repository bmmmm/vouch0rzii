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
		{ID: "alpha", Order: 1, Tier: TierMember, Plus: 1, Email: "alpha@example.org", Knows: []string{"bravo", "charlie"}},
		{ID: "bravo", Tier: TierGuest, Email: "bravo@example.org"},
		{ID: "charlie", Tier: TierAcquaintance, Plus: 2, Knows: []string{"alpha", "bravo"}, Group: true},
	}
	if !reflect.DeepEqual(people, want) {
		t.Fatalf("got %+v\nwant %+v", people, want)
	}
	// The old credit-card column and unknown columns are ignored, not errors.
	if len(warnings) != 2 || !strings.Contains(warnings[0], `"Kreditkarte Verfügbar" ignored`) || !strings.Contains(warnings[1], `"Farbe" ignored`) {
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
		{ID: "alpha", Order: 1, Tier: TierMember, Plus: 1, Email: "alpha@example.org", Knows: []string{"bravo"}, Note: "brings cake"},
		{ID: "bravo", Tier: TierGuest, Out: true},
		{ID: "mammut", Tier: TierAcquaintance, Plus: 10, Group: true},
	}
	tsv := FormatRoster(in)
	if want := "Order\tName\tStatus\t+N\tEmail\tKnows\tGroup\tOut\tNote\n1\talpha\tmember\t1\talpha@example.org\tbravo\tno\tno\tbrings cake\n"; !strings.HasPrefix(tsv, want) || !strings.Contains(tsv, "\tbravo\tguest\t\t\t\tno\tyes\t") {
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
	// A hand-typed line in its own order, as pasted by an organiser; the
	// yes/no words of the former credit-card field are reported, not guessed.
	people, warnings, err := ParseSignup("bananaaa ; hallo@babna.com ; mitglied ; no ; panda ; nein\n", []string{" Panda "})
	if err != nil {
		t.Fatal(err)
	}
	want := Person{ID: "bananaaa", Email: "hallo@babna.com", Tier: TierMember, Knows: []string{"panda"}}
	if len(people) != 1 || !reflect.DeepEqual(people[0], want) {
		t.Fatalf("got %+v, want %+v", people, want)
	}
	if len(warnings) != 3 || !strings.Contains(warnings[0], "bananaaa: fields are not in the form's order") || !strings.Contains(warnings[1], `bananaaa: "no" ignored, no field takes yes/no`) || !strings.Contains(warnings[2], `"nein" ignored`) {
		t.Fatalf("warnings %v", warnings)
	}
	// The form's six fields are read by position, a note is a note.
	people, warnings, err = ParseSignup("dodo; dodo@example.org; Gast; 1; alpha, bravo; vegan\n\n", nil)
	if err != nil || len(warnings) != 0 {
		t.Fatalf("form line: %v %v", err, warnings)
	}
	want = Person{ID: "dodo", Email: "dodo@example.org", Tier: TierGuest, Plus: 1, Knows: []string{"alpha", "bravo"}, Note: "vegan"}
	if !reflect.DeepEqual(people[0], want) {
		t.Fatalf("got %+v, want %+v", people[0], want)
	}
	// Six fields in the wrong order fall back to content with a warning.
	people, warnings, err = ParseSignup("emu; Bekanntschaft; emu@example.org; +2; ; brings cake along", nil)
	if err != nil {
		t.Fatal(err)
	}
	want = Person{ID: "emu", Email: "emu@example.org", Tier: TierAcquaintance, Plus: 2, Note: "brings cake along"}
	if !reflect.DeepEqual(people[0], want) || len(warnings) != 1 || !strings.Contains(warnings[0], "emu: fields are not in the form's order") {
		t.Fatalf("got %+v warnings %v", people[0], warnings)
	}
	// Six fields in the form's order with a typo are errors, not guesses:
	// a misspelled status, a +N that is no number, even with a tier word elsewhere.
	for _, line := range []string{"anna; anna@x.de; Bekantschaft; 1; mammut; vegan", "anna; anna@x.de; Gast; two; mammut; vegan", "anna; anna@x.de; Mitgleid; 1; ; Gast", "anna; ; Gast; two; mammut; vegan"} {
		if _, _, err = ParseSignup(line, nil); err == nil || !strings.Contains(err.Error(), "line 1: ") || strings.Contains(err.Error(), "\n") {
			t.Fatalf("%q: got %v", line, err)
		}
	}
	if _, _, err = ParseSignup("anna; anna@x.de; Gast; two; mammut; vegan", nil); !strings.Contains(err.Error(), `+N "two" is not a number`) {
		t.Fatalf("got %v", err)
	}
	// Six reordered fields without any status word are still read by content.
	people, warnings, err = ParseSignup("emu; 2; emu@example.org; alpha; ; cake", []string{"alpha"})
	if err != nil || !reflect.DeepEqual(people[0], Person{ID: "emu", Email: "emu@example.org", Plus: 2, Knows: []string{"alpha"}, Note: "cake"}) || len(warnings) != 3 || !strings.Contains(warnings[2], "counts as member") {
		t.Fatalf("got %+v %v %v", people, warnings, err)
	}
	// A seven-field line from the old form (card in position 5) is read by position without the card.
	people, warnings, err = ParseSignup("dodo; dodo@example.org; Gast; 1; nein; ; vegan", nil)
	if err != nil || !reflect.DeepEqual(people[0], Person{ID: "dodo", Email: "dodo@example.org", Tier: TierGuest, Plus: 1, Note: "vegan"}) || len(warnings) != 1 || !strings.Contains(warnings[0], `credit-card field "nein" dropped`) {
		t.Fatalf("got %+v %v %v", people, warnings, err)
	}
	// A second number is "already set", not a ja/nein word.
	_, warnings, _ = ParseSignup("fox; Gast; 2; 0", nil)
	if len(warnings) != 1 || !strings.Contains(warnings[0], `"0" ignored, that field is already set`) {
		t.Fatalf("warnings %v", warnings)
	}
	// Empty status and +N by position are fine; the pasted header line, English or German, is skipped.
	for _, header := range []string{SignupFields, "Name; E-Mail; Status; +N; Kennt; Notiz"} {
		people, warnings, err = ParseSignup(header+"\nxy; xy@example.org; ; ; ; ", nil)
		if err != nil || len(warnings) != 0 || len(people) != 1 || !reflect.DeepEqual(people[0], Person{ID: "xy", Email: "xy@example.org"}) {
			t.Fatalf("%s: got %+v %v %v", header, people, warnings, err)
		}
	}
	// By content: the first handle field is Knows, a trailing comma is fine, later words are the note.
	people, _, _ = ParseSignup("anna; Gast; panda, koala,; vegan", nil)
	if !reflect.DeepEqual(people[0].Knows, []string{"panda", "koala"}) || people[0].Note != "vegan" {
		t.Fatalf("got %+v", people[0])
	}
	// A single word is a connection only if it names a known handle, here one
	// from the same paste; any other single word is the note, with a warning.
	people, warnings, _ = ParseSignup("anna; Gast; BOB; vegan\nBob ; Gast; veggie", nil)
	if !reflect.DeepEqual(people[0].Knows, []string{"BOB"}) || people[0].Note != "vegan" || people[1].Knows != nil || people[1].Note != "veggie" || len(warnings) != 2 || !strings.Contains(warnings[0], `line 1: anna: "vegan" kept as note`) || !strings.Contains(warnings[1], `line 2: Bob: "veggie" kept as note, not handles of other people; Knows takes handles with a comma`) {
		t.Fatalf("got %+v warnings %v", people, warnings)
	}
	// Neither the pasted header nor the person's own handle is a connection,
	// and all single words kept as note share one warning.
	people, warnings, _ = ParseSignup(" Name ; E-Mail; Status\nanna; Gast; Name; ANNA", nil)
	if people[0].Knows != nil || people[0].Note != "Name ANNA" || len(warnings) != 1 || !strings.Contains(warnings[0], `line 2: anna: "Name", "ANNA" kept as note`) {
		t.Fatalf("got %+v warnings %v", people, warnings)
	}
	// The first list is the connections; every known single word and every
	// later list of known handles only adds to them, each handle once in any
	// case. A later list with an unknown handle is the note, with a warning
	// that does not ask for commas the line already has.
	people, warnings, _ = ParseSignup("anna; bob; panda, koala, Panda, BOB; carl; cake,salad; dora, CARL; BOB; Gast; 1", []string{"bob", "carl", "dora"})
	if !reflect.DeepEqual(people[0].Knows, []string{"bob", "panda", "koala", "carl", "dora"}) || people[0].Note != "cake,salad" || len(warnings) != 1 || !strings.HasSuffix(warnings[0], `line 1: anna: "cake,salad" kept as note, not handles of other people`) {
		t.Fatalf("got %+v warnings %v", people, warnings)
	}
	// A known handle with a space is a connection; an unknown one is the note.
	people, warnings, _ = ParseSignup("bob; Gast; Anna Lena; Lena Anna", []string{"anna lena"})
	if !reflect.DeepEqual(people[0].Knows, []string{"Anna Lena"}) || people[0].Note != "Lena Anna" || len(warnings) != 0 {
		t.Fatalf("got %+v warnings %v", people, warnings)
	}
	// A list with an entry that is no handle is the note, without a warning.
	people, warnings, _ = ParseSignup("anna; Gast; cake, salad with egg", nil)
	if people[0].Knows != nil || people[0].Note != "cake, salad with egg" || len(warnings) != 0 {
		t.Fatalf("got %+v warnings %v", people, warnings)
	}
	// The warning about words kept as note stands where the first of them was.
	_, warnings, _ = ParseSignup("anna; Gast; Gast; vegan; member; cake; 1", nil)
	if len(warnings) != 3 || !strings.Contains(warnings[0], `"Gast" ignored`) || !strings.Contains(warnings[1], `"vegan", "cake" kept as note`) || !strings.Contains(warnings[2], `"member" ignored`) {
		t.Fatalf("warnings %v", warnings)
	}
	// Read by position, a handle is listed once too.
	people, _, _ = ParseSignup("dodo; dodo@example.org; Gast; 1; alpha, ALPHA; vegan", nil)
	if !reflect.DeepEqual(people[0].Knows, []string{"alpha"}) {
		t.Fatalf("got %+v", people[0])
	}
	// Content mode without a status warns; "1" is a +N.
	people, warnings, _ = ParseSignup("fox; 1", nil)
	if people[0].Plus != 1 || len(warnings) != 1 || !strings.Contains(warnings[0], "counts as member") {
		t.Fatalf("got %+v warnings %v", people[0], warnings)
	}
	// A second e-mail is neither the address nor a connection.
	people, warnings, _ = ParseSignup("gnu; Gast; a@example.org; b@example.org", nil)
	if people[0].Email != "a@example.org" || people[0].Knows != nil || len(warnings) != 1 || !strings.Contains(warnings[0], `gnu: "b@example.org" ignored`) {
		t.Fatalf("got %+v warnings %v", people[0], warnings)
	}
	// Errors name the line; blank lines do not count as people.
	if _, _, err = ParseSignup("\n; nomail@example.org\n", nil); err == nil || !strings.Contains(err.Error(), "line 2: name missing") {
		t.Fatalf("got %v", err)
	}
}
