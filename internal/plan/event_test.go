// SPDX-License-Identifier: GPL-3.0-or-later

package plan

import (
	"reflect"
	"strings"
	"testing"
)

func TestNormalizeErrors(t *testing.T) {
	cases := []struct {
		name string
		e    *Event
		want string
	}{
		{"duplicate id", mkEvent(1, Person{ID: "Alpha"}, Person{ID: "alpha"}), "duplicate id"},
		{"plus over max", mkEvent(1, Person{ID: "a", Plus: 3}), "exceeds the limit of +2"},
		{"plus sign in id", mkEvent(1, Person{ID: "a+b"}), "must not contain"},
		{"no vouchers", mkEvent(0, Person{ID: "a"}), "vouchers must be between 1 and 200"},
		{"too many vouchers", mkEvent(MaxVouchers+1, Person{ID: "a"}), "vouchers must be between 1 and 200"},
		{"plus too big", mkEvent(1, Person{ID: "a", Plus: MaxPlus + 1, Group: true}), "plus must be between 0 and 200"},
		{"pipe in id", mkEvent(1, Person{ID: "a|b"}), "must not contain"},
		{"bad date", &Event{Name: "t", Vouchers: 1, Start: "30.10.2026", People: []Person{{ID: "a"}}}, "YYYY-MM-DD"},
		{"no people", mkEvent(1), "no people"},
		{"no name", &Event{Vouchers: 1, Start: "2026-10-30", People: []Person{{ID: "a"}}}, "name missing"},
	}
	for _, tc := range cases {
		_, err := Normalize(tc.e)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: got %v, want %q", tc.name, err, tc.want)
		}
	}
}

func TestNormalizeCollectsAllErrors(t *testing.T) {
	_, err := Normalize(mkEvent(0, Person{ID: "a", Plus: 5}, Person{ID: "a"}))
	if err == nil {
		t.Fatal("want error")
	}
	for _, want := range []string{"vouchers", "exceeds", "duplicate"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("missing %q in %q", want, err)
		}
	}
}

func TestNormalizeGroupLiftsPlusLimit(t *testing.T) {
	if _, err := Normalize(mkEvent(1, Person{ID: "a", Plus: 10, Group: true})); err != nil {
		t.Fatal(err)
	}
	e := mkEvent(1, Person{ID: "a", Plus: 4})
	e.PlusMax = 4
	if _, err := Normalize(e); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizeKnowsSymmetricAndCleaned(t *testing.T) {
	e := mkEvent(1,
		Person{ID: "a", Knows: []string{"B", "a", "ghost", "b"}},
		Person{ID: "b"},
	)
	warnings, err := Normalize(e)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(e.People[0].Knows, []string{"b"}) || !reflect.DeepEqual(e.People[1].Knows, []string{"a"}) {
		t.Fatalf("knows not symmetric/clean: %v / %v", e.People[0].Knows, e.People[1].Knows)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "ghost") {
		t.Fatalf("warnings %v", warnings)
	}
	if !e.Knows("a", "b") || !e.Knows("b", "a") || e.Knows("a", "ghost") {
		t.Fatal("Knows lookup wrong")
	}
}

func TestParseEventRejectsUnknownFields(t *testing.T) {
	if _, err := ParseEvent([]byte(`{"name":"t","vouchers":1,"start":"2026-10-30","peple":[]}`)); err == nil {
		t.Fatal("typo in field name must fail")
	}
	if _, err := ParseEvent([]byte(`{"name":"t","vouchers":1,"start":"2026-10-30","people":[]} {}`)); err == nil || !strings.Contains(err.Error(), "trailing data") {
		t.Fatalf("trailing data must fail, got %v", err)
	}
	e, err := ParseEvent([]byte(`{"name":"t","vouchers":1,"start":"2026-10-30","people":[{"id":"a","tier":"Gast"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if e.People[0].Tier != TierGuest {
		t.Fatalf("tier alias in JSON: %v", e.People[0].Tier)
	}
	b, _ := MarshalEvent(e)
	if !strings.Contains(string(b), `"tier": "guest"`) {
		t.Fatalf("tier should marshal as string: %s", b)
	}
}
