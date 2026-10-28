// SPDX-License-Identifier: GPL-3.0-or-later

package plan

import (
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
		{"extra bad date", &Event{Name: "t", Vouchers: 1, Start: "2026-10-30", Extra: []Extra{{Date: "5.11.", Count: 1}}, People: []Person{{ID: "a"}}}, "extra vouchers 1: date"},
		{"extra zero count", &Event{Name: "t", Vouchers: 1, Start: "2026-10-30", Extra: []Extra{{Date: "2026-11-05"}}, People: []Person{{ID: "a"}}}, "count must be between 1 and 200"},
		{"extra too many", &Event{Name: "t", Vouchers: 1, Start: "2026-10-30", Extra: []Extra{{Date: "2026-11-05", Count: MaxVouchers}}, People: []Person{{ID: "a"}}}, "in total, more than"},
		{"extra count overflow", &Event{Name: "t", Vouchers: 1, Start: "2026-10-30", Extra: []Extra{{Date: "2026-11-05", Count: 1 << 62}, {Date: "2026-11-05", Count: 1 << 62}}, People: []Person{{ID: "a"}}}, "count must be between 1 and 200"},
		{"every too big", &Event{Name: "t", Vouchers: 1, Start: "2026-10-30", Every: MaxEvery + 1, People: []Person{{ID: "a"}}}, "every must be between 0 and 14"},
		{"every negative", &Event{Name: "t", Vouchers: 1, Start: "2026-10-30", Every: -1, People: []Person{{ID: "a"}}}, "every must be between 0 and 14"},
		{"plusMax too big", &Event{Name: "t", Vouchers: 1, Start: "2026-10-30", PlusMax: MaxPlus + 1, People: []Person{{ID: "a"}}}, "plusMax must be between 0 and 200"},
		{"tab in id", mkEvent(1, Person{ID: "a\tb"}), "must not contain"},
		{"too many people", mkEvent(1, members(MaxPeople+1)...), "more than 2000"},
		{"bad date", &Event{Name: "t", Vouchers: 1, Start: "30.10.2026", People: []Person{{ID: "a"}}}, "YYYY-MM-DD"},
		{"bad end", &Event{Name: "t", Vouchers: 1, Start: "2026-10-30", End: "13.11.", People: []Person{{ID: "a"}}}, "end must be YYYY-MM-DD"},
		{"end before start", &Event{Name: "t", Vouchers: 1, Start: "2026-10-30", End: "2026-10-29", People: []Person{{ID: "a"}}}, "end 2026-10-29 is before start"},
		{"bad frozen", &Event{Name: "t", Vouchers: 1, Start: "2026-10-30", Frozen: "heute", People: []Person{{ID: "a"}}}, "frozen must be YYYY-MM-DD"},
		{"lost voucher 0", &Event{Name: "t", Vouchers: 2, Start: "2026-10-30", Lost: []Lost{{Voucher: 0, Date: "2026-11-02"}}, People: []Person{{ID: "a"}}}, "lost voucher 0: no such voucher (1 to 2)"},
		{"lost voucher beyond extras", &Event{Name: "t", Vouchers: 2, Start: "2026-10-30", Extra: []Extra{{Date: "2026-11-02", Count: 1}}, Lost: []Lost{{Voucher: 4, Date: "2026-11-02"}}, People: []Person{{ID: "a"}}}, "lost voucher 4: no such voucher (1 to 3)"},
		{"lost voucher twice", &Event{Name: "t", Vouchers: 2, Start: "2026-10-30", Lost: []Lost{{Voucher: 2, Date: "2026-11-02"}, {Voucher: 2, Date: "2026-11-03"}}, People: []Person{{ID: "a"}}}, "lost voucher 2: listed twice"},
		{"lost bad date", &Event{Name: "t", Vouchers: 2, Start: "2026-10-30", Lost: []Lost{{Voucher: 1, Date: "heute"}}, People: []Person{{ID: "a"}}}, "lost voucher 1: date must be YYYY-MM-DD"},
		{"lost too many", &Event{Name: "t", Vouchers: 2, Start: "2026-10-30", Lost: make([]Lost, MaxVouchers+1), People: []Person{{ID: "a"}}}, "lost entries, more than"},
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
	// A bad start is reported once; the end is not compared against it.
	_, err = Normalize(&Event{Name: "t", Vouchers: 1, Start: "30.10.2026", End: "2026-11-13", People: []Person{{ID: "a"}}})
	if err == nil || !strings.Contains(err.Error(), "start must be") || strings.Contains(err.Error(), "before start") {
		t.Fatalf("got %v", err)
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
