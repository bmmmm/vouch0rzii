// SPDX-License-Identifier: GPL-3.0-or-later

package plan

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// mkEvent builds a valid event with the given people.
func mkEvent(vouchers int, people ...Person) *Event {
	return &Event{Name: "t", Vouchers: vouchers, Start: "2026-10-30", People: people}
}

func members(n int) []Person {
	var out []Person
	for i := 0; i < n; i++ {
		out = append(out, Person{ID: fmt.Sprintf("m%02d", i)})
	}
	return out
}

func labels(slots []*Slot) []string {
	out := make([]string, len(slots))
	for i, s := range slots {
		out[i] = s.Label
	}
	return out
}

func mustCompute(t *testing.T, e *Event, pins []Pin) *Plan {
	t.Helper()
	p, err := Compute(e, pins)
	if err != nil {
		t.Fatalf("Compute: %v", err)
	}
	return p
}

func TestBuildSlotsTierOrder(t *testing.T) {
	e := mkEvent(2,
		Person{ID: "a", Tier: TierAcquaintance},
		Person{ID: "b"},
		Person{ID: "c", Tier: TierGuest},
		Person{ID: "d", Plus: 2},
		Person{ID: "e", Plus: 3, Group: true},
	)
	if _, err := Normalize(e); err != nil {
		t.Fatal(err)
	}
	got := labels(buildSlots(e))
	if len(got) != 10 {
		t.Fatalf("want 10 slots, got %d: %v", len(got), got)
	}
	memberBlock := strings.Join(got[:3], " ")
	for _, id := range []string{"b", "d", "e"} {
		if !strings.Contains(" "+memberBlock+" ", " "+id+" ") {
			t.Fatalf("member %s not in first block %v", id, got[:3])
		}
	}
	if got[3] != "c" || got[4] != "a" {
		t.Fatalf("guest then acquaintance expected at 3,4: %v", got)
	}
	// +1 round in sponsor rank order, then +2 round, then the overflow.
	rank := map[string]int{}
	for i, l := range got[:3] {
		rank[l] = i
	}
	plus1, plus2 := got[5:7], got[7:9]
	wantFirst := "d"
	if rank["e"] < rank["d"] {
		wantFirst = "e"
	}
	if plus1[0] != wantFirst+"+1" || plus2[0] != wantFirst+"+2" {
		t.Fatalf("+k rounds not in sponsor rank order: %v", got)
	}
	if got[9] != "e+3" {
		t.Fatalf("overflow last, got %v", got)
	}
}

func TestDrawIsSeededAndStable(t *testing.T) {
	e1 := mkEvent(3, members(10)...)
	e2 := mkEvent(3, members(10)...)
	_, _ = Normalize(e1)
	_, _ = Normalize(e2)
	a, b := labels(buildSlots(e1)), labels(buildSlots(e2))
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("same seed, different draw: %v vs %v", a, b)
	}
	e2.Seed = "another"
	if c := labels(buildSlots(e2)); reflect.DeepEqual(a, c) {
		t.Fatalf("different seed, same draw: %v", a)
	}
	// Adding a guest must not reshuffle the members.
	e3 := mkEvent(3, append(members(10), Person{ID: "g", Tier: TierGuest})...)
	_, _ = Normalize(e3)
	if d := labels(buildSlots(e3)); !reflect.DeepEqual(d[:10], a) || d[10] != "g" {
		t.Fatalf("guest reshuffled members: %v", d)
	}
}

func TestOrderOverridesLot(t *testing.T) {
	ps := members(6)
	ps[5].Order = 1
	ps[0].Order = 2
	e := mkEvent(2, ps...)
	_, _ = Normalize(e)
	got := labels(buildSlots(e))
	if got[0] != "m05" || got[1] != "m00" {
		t.Fatalf("manual order ignored: %v", got)
	}
}

func TestDaysAreCeilOfSlotsOverVouchers(t *testing.T) {
	for _, tc := range []struct{ slots, vouchers, days int }{{47, 7, 7}, {49, 7, 7}, {50, 7, 8}, {1, 7, 1}, {7, 7, 1}} {
		p := mustCompute(t, mkEvent(tc.vouchers, members(tc.slots)...), nil)
		if p.Stats.Days != tc.days {
			t.Fatalf("%d slots / %d vouchers: got %d days want %d", tc.slots, tc.vouchers, p.Stats.Days, tc.days)
		}
		if p.Stats.Slots != tc.slots {
			t.Fatalf("slots: got %d want %d", p.Stats.Slots, tc.slots)
		}
	}
}

func TestPinsFixCellsAndRestFillsAround(t *testing.T) {
	ps := members(6)
	for i := range ps {
		ps[i].Order = i + 1
	}
	p := mustCompute(t, mkEvent(2, ps...), []Pin{{Label: "m05", Day: 1, Voucher: 2}})
	if s := p.Grid[0][1]; s == nil || s.Label != "m05" || !s.Pinned {
		t.Fatalf("pin not honoured: %+v", s)
	}
	want := [][]string{{"m00", "m05"}, {"m01", "m02"}, {"m03", "m04"}}
	for d, row := range want {
		for v, l := range row {
			if p.Grid[d][v] == nil || p.Grid[d][v].Label != l {
				t.Fatalf("day %d voucher %d: got %v want %s", d+1, v+1, p.Grid[d][v], l)
			}
		}
	}
	if p.Stats.Pinned != 1 || p.Stats.Days != 3 {
		t.Fatalf("stats %+v", p.Stats)
	}
}

func TestPinWarningsAndErrors(t *testing.T) {
	ps := members(4)
	p := mustCompute(t, mkEvent(2, ps...), []Pin{{Label: "m00", Day: 9, Voucher: 1}, {Label: "nobody", Day: 1, Voucher: 1}, {Label: "m01", Day: 1, Voucher: 5}})
	if p.Stats.Days != 9 {
		t.Fatalf("pin on day 9 should stretch the plan, got %d days", p.Stats.Days)
	}
	joined := strings.Join(p.Warnings, "\n")
	for _, want := range []string{"stretch the plan to 9 days", "no such slot", "outside the plan"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing warning %q in %q", want, joined)
		}
	}
	if _, err := Compute(mkEvent(2, members(4)...), []Pin{{Label: "m00", Day: 1, Voucher: 1}, {Label: "m01", Day: 1, Voucher: 1}}); err == nil || !strings.Contains(err.Error(), "same cell") {
		t.Fatalf("two pins on one cell must fail, got %v", err)
	}
	if _, err := Compute(mkEvent(2, members(4)...), []Pin{{Label: "m00", Day: 1, Voucher: 1}, {Label: "m00", Day: 2, Voucher: 1}}); err == nil || !strings.Contains(err.Error(), "pinned twice") {
		t.Fatalf("slot pinned twice must fail, got %v", err)
	}
}

func TestHandoversCrossIdleDays(t *testing.T) {
	// a holds voucher 1 on day 1, nobody on day 2, b (pinned) on day 3: the
	// handover a → b happens on the evening of day 1 and b buys on day 3.
	p := mustCompute(t, mkEvent(1, ordered("a", "b")...), []Pin{{Label: "b", Day: 3, Voucher: 1}})
	if p.Stats.Handovers != 1 || p.Handovers[0].Day != 1 || p.Handovers[0].To.Label != "b" {
		t.Fatalf("handovers %+v", p.Handovers)
	}
	if ex := p.ToExport(); ex.Sheets[1].From != "a" || ex.Handovers[0].Date != "2026-10-30" {
		t.Fatalf("export %+v", ex)
	}
	if md := p.Markdown(); !strings.Contains(md, "- Voucher 1: a → b, buys on Di 03.11.") {
		t.Fatalf("markdown:\n%s", md)
	}
	ps := ordered("a")
	ps[0].Plus = 1
	p = mustCompute(t, mkEvent(1, ps...), []Pin{{Label: "a+1", Day: 3, Voucher: 1}})
	if p.Stats.Forbidden != 1 {
		t.Fatalf("forbidden handover across an idle day not counted: %+v", p.Stats)
	}
}

func TestDayLimit(t *testing.T) {
	p := mustCompute(t, mkEvent(2, members(4)...), []Pin{{Label: "m00", Day: MaxDays + 1, Voucher: 1}})
	if p.Stats.Days != 2 || !strings.Contains(strings.Join(p.Warnings, "\n"), "outside the plan") {
		t.Fatalf("pin beyond MaxDays should be dropped: days %d, warnings %v", p.Stats.Days, p.Warnings)
	}
	if _, err := Compute(mkEvent(1, members(MaxDays+1)...), nil); err == nil || !strings.Contains(err.Error(), "more than 400") {
		t.Fatalf("too many days must fail, got %v", err)
	}
}

func TestFreezeReproducesPlanUnderAnotherSeed(t *testing.T) {
	ps := members(11)
	ps[3].Plus = 2
	ps[7].Plus = 1
	e := mkEvent(3, ps...)
	e.People[0].Knows = []string{"m07"}
	p1 := mustCompute(t, e, nil)
	pins := Freeze(p1)
	e2 := mkEvent(3, ps...)
	e2.Seed = "completely different"
	p2 := mustCompute(t, e2, pins)
	if !reflect.DeepEqual(p1.ToExport().Grid, p2.ToExport().Grid) {
		t.Fatalf("frozen plan differs:\n%v\n%v", p1.ToExport().Grid, p2.ToExport().Grid)
	}
	if p2.Stats.Pinned != p2.Stats.Slots {
		t.Fatalf("all slots should be pinned, got %d of %d", p2.Stats.Pinned, p2.Stats.Slots)
	}
}
