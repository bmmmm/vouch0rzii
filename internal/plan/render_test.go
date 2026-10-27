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

func TestMarkdownAndTextContainMatrixAndHandovers(t *testing.T) {
	p := mustCompute(t, renderEvent(), nil)
	md := p.Markdown()
	for _, want := range []string{"| Voucher | Fr 30.10. | Mo 02.11. |", "| 1 | a | c |", "| 2 | b | a+1 |", "- Voucher 1: a → c (c@example.org)", "- Voucher 2: b → a+1 (a@example.org)"} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q:\n%s", want, md)
		}
	}
	txt := p.Text()
	for _, want := range []string{"2 vouchers, 2 days, 4 slots", "a → c", "b → a+1 <a@example.org>", "strangers: 2"} {
		if !strings.Contains(txt, want) {
			t.Errorf("text missing %q:\n%s", want, txt)
		}
	}
	if csv := p.CSV(); !strings.HasPrefix(csv, "Voucher;Fr 30.10. 2026-10-30;Mo 02.11. 2026-11-02\n1;a;c\n2;b;a+1\n") {
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
	if ex.Warnings == nil || ex.Handovers == nil {
		t.Fatal("empty lists must serialise as [] not null")
	}
}
