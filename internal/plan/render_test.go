// SPDX-License-Identifier: GPL-3.0-or-later

package plan

import (
	"encoding/json"
	"reflect"
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
		"| Voucher | Fri 30/10 | Mon 02/11 |",
		"| 1 | a | c |",
		"| 2 | b | a+1 |",
		"**Fri 30/10, 23:00**",
		"- Voucher 1: a → c (c@example.org)\n",
		"- Voucher 2: b → a+1 (a@example.org)\n",
		"2 handovers, 0 of them stay within a group (✓).",
		"handover at 23:00. Version " + p.Fingerprint(),
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q:\n%s", want, md)
		}
	}
	if csv := p.CSV(); csv != "Voucher;Fri 30/10 2026-10-30;Mon 02/11 2026-11-02\n1;a;c\n2;b;a+1\n" {
		t.Errorf("csv:\n%s", csv)
	}
}

// Every other day: buying days skip a day, extras start on the first buying
// day from their date, the handover is on the evening the new voucher is
// expected, and the end counts buying days.
func TestEveryOtherDay(t *testing.T) {
	e := renderEvent()
	e.Start, e.End, e.Every = "2026-11-02", "2026-11-07", 2 // Monday to Saturday
	e.Extra = []Extra{{Date: "2026-11-05", Count: 1}}
	p := mustCompute(t, e, nil)
	if want := []string{"2026-11-02", "2026-11-04"}; !reflect.DeepEqual(p.Dates, want) {
		t.Errorf("dates %v, want %v", p.Dates, want)
	}
	if p.PhaseDays != 3 || p.From[2] != 3 {
		t.Errorf("phase days %d, extra from day %d; want 3 (02/11, 04/11, 06/11) and 3", p.PhaseDays, p.From[2])
	}
	if md := p.Markdown(); !strings.Contains(md, "**Tue 03/11, 23:00**") {
		t.Errorf("markdown handover heading:\n%s", md)
	}
	h := p.ToExport().Handovers[0]
	if h.Day != 1 || h.Date != "2026-11-03" || h.Label != "Tue 03/11" {
		t.Errorf("handover day %d date %s label %s, want 1 2026-11-03 Tue 03/11", h.Day, h.Date, h.Label)
	}
}

// The voucher process rests on the weekend: bought on Friday, the new voucher
// comes on Monday evening and the next holder buys on Tuesday. With weekends
// it comes on Saturday. With Monday, Wednesday and Friday it rests on the days
// between: every 1 hands over on the buying evening, every 2 on the evening of
// the next plan day.
func TestEveryOtherDayAcrossTheWeekend(t *testing.T) {
	for _, tc := range []struct {
		weekends            bool
		days                []string
		every               int
		start, handover, to string
	}{
		{false, nil, 2, "", "2026-11-02", "2026-11-03"},
		{true, nil, 2, "", "2026-10-31", "2026-11-01"},
		{false, []string{"mon", "wed", "fri"}, 1, "", "2026-10-30", "2026-11-02"}, // the voucher comes the same evening
		{false, []string{"mon", "wed", "fri"}, 2, "", "2026-11-02", "2026-11-04"},
		{false, []string{"mon", "wed", "fri"}, 2, "2026-11-04", "2026-11-06", "2026-11-09"}, // Wednesday: not Thursday
	} {
		e := renderEvent() // starts on Friday 30/10
		e.Every, e.Weekends, e.Days = tc.every, tc.weekends, tc.days
		if tc.start != "" {
			e.Start = tc.start
		}
		p := mustCompute(t, e, nil)
		h := p.ToExport().Handovers[0]
		if h.Date != tc.handover || p.Dates[1] != tc.to {
			t.Errorf("weekends %v days %v every %d: handover %s, next buys %s; want %s, %s", tc.weekends, tc.days, tc.every, h.Date, p.Dates[1], tc.handover, tc.to)
		}
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
	for _, want := range []string{`"handovers": []`, `"warnings": []`, `"public": []`, `"end": {
    "day": 0,`} {
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

// The calendar counts plan days on the page: the grid's dates, then on to End
// when the phase outlasts the grid.
func TestExportCalendarReachesEnd(t *testing.T) {
	e := renderEvent() // two plan days from Friday 30/10
	ex := mustCompute(t, e, nil).ToExport()
	if !reflect.DeepEqual(ex.Calendar, []string{"2026-10-30", "2026-11-02"}) {
		t.Errorf("without end: calendar %v, want the grid's two dates", ex.Calendar)
	}
	e = renderEvent()
	e.End, e.Days = "2026-11-12", []string{"mon", "wed", "fri"} // a Thursday: the phase ends on Wednesday's plan day
	ex = mustCompute(t, e, nil).ToExport()
	want := []string{"2026-10-30", "2026-11-02", "2026-11-04", "2026-11-06", "2026-11-09", "2026-11-11"}
	if !reflect.DeepEqual(ex.Calendar, want) || ex.End.Day != len(want) {
		t.Errorf("with end: calendar %v, end day %d; want %v, %d", ex.Calendar, ex.End.Day, want, len(want))
	}
}

// Extra and lost vouchers land on plan days of the week: on Monday, Wednesday
// and Friday from Friday 30/10, Wednesday 04/11 is day 3 and Friday 06/11
// day 4 (on the workweek 4 and 6).
func TestExtraAndLostOnWeekDays(t *testing.T) {
	e := mkEvent(1, members(6)...)
	e.Days = []string{"mon", "wed", "fri"}
	e.Extra = []Extra{{Date: "2026-11-04", Count: 1}}
	e.Lost = []Lost{{Voucher: 1, Date: "2026-11-06"}}
	p := mustCompute(t, e, nil)
	if !reflect.DeepEqual(p.From, []int{1, 3}) || !reflect.DeepEqual(p.Until, []int{4, 0}) {
		t.Errorf("voucher from %v until %v, want [1 3] [4 0]", p.From, p.Until)
	}
	if md := p.Markdown(); !strings.Contains(md, "Voucher 1 lost from Fri 06/11.") {
		t.Errorf("markdown misses the lost day:\n%s", md)
	}
}
