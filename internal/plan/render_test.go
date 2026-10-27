// SPDX-License-Identifier: GPL-3.0-or-later

package plan

import (
	"encoding/json"
	"strings"
	"testing"
)

func renderEvent() *Event {
	ps := ordered("a", "b", "c")
	ps[0].Plus = 1
	ps[0].Email = "a@example.org"
	ps[2].Tier = TierGuest
	ps[2].Email = "c@example.org"
	return mkEvent(2, ps...)
}

func TestMarkdownAndCSV(t *testing.T) {
	p := mustCompute(t, renderEvent(), nil)
	md := p.Markdown()
	for _, want := range []string{
		"# t\n\n2 vouchers, 2 days, 4 slots, handover at 23:00.",
		"| Voucher | Fr 30.10. | Mo 02.11. |",
		"| 1 | a | c |",
		"| 2 | b | a+1 |",
		"**Fr 30.10., 23:00**",
		"- Voucher 1: a → c (c@example.org)\n",
		"- Voucher 2: b → a+1 (a@example.org)\n",
		"2 handovers, 0 between buddies, 2 new connections.",
		"handover at 23:00. Version " + p.Fingerprint(),
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q:\n%s", want, md)
		}
	}
	if csv := p.CSV(); csv != "Voucher;Fr 30.10. 2026-10-30;Mo 02.11. 2026-11-02\n1;a;c\n2;b;a+1\n" {
		t.Errorf("csv:\n%s", csv)
	}
}

func TestJSONExport(t *testing.T) {
	p := mustCompute(t, renderEvent(), nil)
	b, err := p.JSON()
	if err != nil {
		t.Fatal(err)
	}
	var ex Export
	if err := json.Unmarshal(b, &ex); err != nil {
		t.Fatal(err)
	}
	if len(ex.Days) != 2 || ex.Grid[1][0] != "c" || ex.Grid[1][1] != "a+1" || len(ex.Pins) != 4 {
		t.Fatalf("export %+v", ex)
	}
	var sheet *Sheet
	for i := range ex.Sheets {
		if ex.Sheets[i].Label == "a+1" {
			sheet = &ex.Sheets[i]
		}
	}
	if sheet == nil || sheet.Person != "a" || sheet.Email != "a@example.org" || sheet.From != "b" || sheet.Day != 2 || sheet.Voucher != 2 || sheet.To != "" {
		t.Fatalf("sheet %+v", sheet)
	}
	if ex.Sheets[0].From != OrgaLabel {
		t.Fatalf("first holder should receive from %s: %+v", OrgaLabel, ex.Sheets[0])
	}
	// A one-day plan has no handovers and no warnings; both must be [] in JSON.
	one, err := mustCompute(t, mkEvent(2, ordered("a", "b")...), nil).JSON()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"handovers": []`, `"warnings": []`} {
		if !strings.Contains(string(one), want) {
			t.Errorf("missing %s in\n%s", want, one)
		}
	}
	// Everyone left and nothing is pinned: no slots, still a complete export.
	none := mkEvent(2, ordered("a", "b")...)
	none.People[0].Out, none.People[1].Out = true, true
	empty, err := mustCompute(t, none, nil).JSON()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"sheets": []`, `"handovers": []`, `"days": 1`} {
		if !strings.Contains(string(empty), want) {
			t.Errorf("zero-slot export missing %s in\n%s", want, empty)
		}
	}
}
