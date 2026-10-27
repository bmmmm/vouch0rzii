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
		{ID: "bravo", Tier: TierGuest},
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
