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
	got := labels(buildSlots(e, nil))
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
	a, b := labels(buildSlots(e1, nil)), labels(buildSlots(e2, nil))
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("same seed, different draw: %v vs %v", a, b)
	}
	e2.Seed = "another"
	if c := labels(buildSlots(e2, nil)); reflect.DeepEqual(a, c) {
		t.Fatalf("different seed, same draw: %v", a)
	}
	// Adding a guest must not reshuffle the members.
	e3 := mkEvent(3, append(members(10), Person{ID: "g", Tier: TierGuest})...)
	_, _ = Normalize(e3)
	if d := labels(buildSlots(e3, nil)); !reflect.DeepEqual(d[:10], a) || d[10] != "g" {
		t.Fatalf("guest reshuffled members: %v", d)
	}
}

func TestOrderOverridesLot(t *testing.T) {
	ps := members(6)
	ps[5].Order = 1
	ps[0].Order = 2
	e := mkEvent(2, ps...)
	_, _ = Normalize(e)
	got := labels(buildSlots(e, nil))
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
	if md := p.Markdown(); !strings.Contains(md, "- Voucher 1: a → b, buys on Tue 03/11") {
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
	ps[3].Group = true
	e := mkEvent(3, ps...)
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

func TestMemberCountDoesNotReshuffleGuests(t *testing.T) {
	guests := func(n int) []Person {
		var out []Person
		for i := 0; i < n; i++ {
			out = append(out, Person{ID: fmt.Sprintf("g%02d", i), Tier: TierGuest})
		}
		return out
	}
	e1 := mkEvent(3, append(members(10), guests(6)...)...)
	e2 := mkEvent(3, append(members(11), guests(6)...)...)
	_, _ = Normalize(e1)
	_, _ = Normalize(e2)
	a, b := labels(buildSlots(e1, nil))[10:], labels(buildSlots(e2, nil))[11:]
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("an extra member reshuffled the guests: %v vs %v", a, b)
	}
}

func TestExtraVouchersStartLater(t *testing.T) {
	e := mkEvent(1, members(6)...)
	e.Extra = []Extra{{Date: "2026-11-03", Count: 1}} // Fri 30/10 = day 1, Mon 02/11 = day 2, Tue 03/11 = day 3
	p := mustCompute(t, e, nil)
	if p.Stats.Days != 4 || !reflect.DeepEqual(p.From, []int{1, 3}) {
		t.Fatalf("days %d from %v", p.Stats.Days, p.From)
	}
	if p.Grid[0][1] != nil || p.Grid[1][1] != nil || p.Grid[2][1] == nil || p.Grid[3][1] == nil {
		t.Fatalf("extra voucher must be used from day 3 on only: %v", p.ToExport().Grid)
	}
	ex := p.ToExport()
	if ex.Vouchers != 2 || !reflect.DeepEqual(ex.VoucherFrom, []int{1, 3}) {
		t.Fatalf("export %d %v", ex.Vouchers, ex.VoucherFrom)
	}
	for _, sh := range ex.Sheets {
		if sh.Voucher == 2 && sh.Day == 3 && sh.From != OrgaLabel {
			t.Fatalf("first holder of the extra chain must receive from %s: %+v", OrgaLabel, sh)
		}
	}
	p2 := mustCompute(t, mkEventExtra(), []Pin{{Label: "m00", Day: 1, Voucher: 2}})
	if !strings.Contains(strings.Join(p2.Warnings, "\n"), "not yet available") {
		t.Fatalf("pin on an unavailable cell must be dropped with a warning: %v", p2.Warnings)
	}
	far := mkEventExtra()
	far.Extra[0].Date = "2030-01-01"
	if _, err := Compute(far, nil); err == nil || !strings.Contains(err.Error(), "horizon") {
		t.Fatalf("extra beyond the horizon must fail, got %v", err)
	}
}

func mkEventExtra() *Event {
	e := mkEvent(1, members(6)...)
	e.Extra = []Extra{{Date: "2026-11-03", Count: 1}}
	return e
}

func TestOutPersonKeepsOnlyPinnedSlots(t *testing.T) {
	ps := ordered("a", "b", "c", "d")
	ps[2].Out, ps[2].Plus = true, 1
	p := mustCompute(t, mkEvent(2, ps...), nil)
	if got := labels(p.Slots); !reflect.DeepEqual(got, []string{"a", "b", "d"}) {
		t.Fatalf("unpinned slots of a person who left must vanish: %v", got)
	}
	p = mustCompute(t, mkEvent(2, ps...), []Pin{{Label: "c", Day: 1, Voucher: 1}})
	if got := labels(p.Slots); !reflect.DeepEqual(got, []string{"a", "b", "c", "d"}) || cell(p, 1, 1) != "c" {
		t.Fatalf("pinned slot of a person who left must stay: %v", got)
	}
	// Leaving does not reshuffle the others.
	ms := members(10)
	before := labels(mustCompute(t, mkEvent(3, ms...), nil).Slots)
	ms[3].Out = true
	after := labels(mustCompute(t, mkEvent(3, ms...), nil).Slots)
	var want []string
	for _, l := range before {
		if l != "m03" {
			want = append(want, l)
		}
	}
	if !reflect.DeepEqual(after, want) {
		t.Fatalf("others moved: %v vs %v", after, want)
	}
}

func TestHandoversSortedByDayThenVoucher(t *testing.T) {
	// b's group keeps voucher 2 for b+1 on day 2; voucher 1 idles on day 2
	// and continues with the pinned c on day 3. Both handovers happen on the
	// evening of day 1 and must be listed voucher 1 first.
	ps := ordered("a", "b", "c")
	ps[1].Plus, ps[1].Group = 1, true
	p := mustCompute(t, mkEvent(2, ps...), []Pin{{Label: "c", Day: 3, Voucher: 1}})
	if len(p.Handovers) != 2 || p.Handovers[0].Day != 1 || p.Handovers[0].Voucher != 1 || p.Handovers[0].To.Label != "c" || p.Handovers[1].Voucher != 2 {
		t.Fatalf("handovers %+v %+v", p.Handovers[0], p.Handovers[1])
	}
}

func TestOutPersonWithDroppedPinStaysOut(t *testing.T) {
	ps := ordered("a", "b", "c", "d")
	ps[2].Out = true
	p := mustCompute(t, mkEvent(2, ps...), []Pin{{Label: "c", Day: 1, Voucher: 3}}) // voucher 3 does not exist
	if got := labels(p.Slots); !reflect.DeepEqual(got, []string{"a", "b", "d"}) {
		t.Fatalf("a dropped pin must not bring back a person who left: %v", got)
	}
	e := mkEvent(1, ps...)
	e.Extra = []Extra{{Date: "2026-11-03", Count: 1}} // voucher 2 exists from day 3 on
	if got := labels(mustCompute(t, e, []Pin{{Label: "c", Day: 1, Voucher: 2}}).Slots); !reflect.DeepEqual(got, []string{"a", "b", "d"}) {
		t.Fatalf("a pin on a voucher that is not yet available must not bring back a person who left: %v", got)
	}
}

func TestEndOfVoucherPhase(t *testing.T) {
	// Fri 30/10 = day 1, Mon 02/11 = day 2, Tue 03/11 = day 3: two vouchers
	// hold six slots until the end; the seventh goes to the public sale.
	e := mkEvent(2, ordered("a", "b", "c", "d", "e", "f", "g")...)
	e.End = "2026-11-03"
	p := mustCompute(t, e, nil)
	if p.PhaseDays != 3 || p.Stats.Days != 3 || p.Stats.Slots != 7 || p.Stats.Free != 0 || len(p.Public()) != 1 || !reflect.DeepEqual(p.Public(), []string{"g"}) {
		t.Fatalf("phase %d stats %+v public %v", p.PhaseDays, p.Stats, p.Public())
	}
	ex := p.ToExport()
	if ex.End != (DayExport{Day: 3, Date: "2026-11-03", Label: "Tue 03/11"}) || !reflect.DeepEqual(ex.Public, []string{"g"}) || len(ex.Sheets) != 6 || len(ex.Pins) != 6 {
		t.Fatalf("export end %+v public %v sheets %d pins %d", ex.End, ex.Public, len(ex.Sheets), len(ex.Pins))
	}
	md := p.Markdown()
	for _, want := range []string{"Voucher phase until Tue 03/11 (3 days), 0 slots free.", "**Public sale:** 1 ticket(s) did not fit into the voucher phase: g"} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q:\n%s", want, md)
		}
	}
	// A pin after the end is dropped and the slot stays in the public sale.
	p = mustCompute(t, e, []Pin{{Label: "g", Day: 4, Voucher: 1}})
	if !strings.Contains(strings.Join(p.Warnings, "\n"), "after the voucher phase (3 days)") || len(p.Public()) != 1 || p.Stats.Days != 3 {
		t.Fatalf("pin after the end: warnings %v stats %+v", p.Warnings, p.Stats)
	}
	// A pin on the last day of the phase is honoured; someone else drops out.
	p = mustCompute(t, e, []Pin{{Label: "g", Day: 3, Voucher: 1}})
	if cell(p, 3, 1) != "g" || len(p.Public()) != 1 || !reflect.DeepEqual(p.Public(), []string{"f"}) || len(p.Warnings) != 0 {
		t.Fatalf("pin on the last day: public %v warnings %v", p.Public(), p.Warnings)
	}
	// Free slots count until the end, not only within the grid.
	e = mkEvent(2, ordered("a", "b", "c")...)
	e.End = "2026-11-03"
	if p = mustCompute(t, e, nil); p.Stats.Days != 2 || p.Stats.Free != 3 || len(p.Public()) != 0 {
		t.Fatalf("slack until the end: %+v", p.Stats)
	}
	if md := p.Markdown(); !strings.Contains(md, "3 slots free.") || strings.Contains(md, "Public sale") {
		t.Fatalf("markdown:\n%s", md)
	}
	// A one-day phase: end equals start.
	e.End = e.Start
	if p = mustCompute(t, e, nil); p.PhaseDays != 1 || len(p.Public()) != 1 || p.Stats.Free != 0 {
		t.Fatalf("one-day phase: %d %+v", p.PhaseDays, p.Stats)
	}
	// An extra voucher that arrives on the last day still counts one slot.
	e = mkEvent(1, ordered("a", "b", "c")...)
	e.End, e.Extra = "2026-11-03", []Extra{{Date: "2026-11-03", Count: 1}}
	if p = mustCompute(t, e, nil); p.Stats.Free != 1 || len(p.Public()) != 0 || len(p.Warnings) != 0 {
		t.Fatalf("extra on the last day: %+v %v", p.Stats, p.Warnings)
	}
	// Without an end the free slots are the empty cells of the grid.
	if p = mustCompute(t, mkEvent(2, ordered("a", "b", "c")...), nil); p.Stats.Free != 1 || p.ToExport().End != (DayExport{}) || len(p.ToExport().Public) != 0 {
		t.Fatalf("open-ended: %+v end %+v", p.Stats, p.ToExport().End)
	}
	if md := p.Markdown(); !strings.Contains(md, "Version "+p.Fingerprint()+"\n\n1 slots free.\n") || strings.Contains(md, "Voucher phase") {
		t.Fatalf("open-ended markdown:\n%s", md)
	}
	// An extra voucher that arrives two days after the end adds no capacity
	// (and never a negative one) and is reported as unused.
	e = mkEvent(1, ordered("a", "b", "c")...)
	e.End, e.Extra = "2026-11-03", []Extra{{Date: "2026-11-05", Count: 1}}
	if p = mustCompute(t, e, nil); p.Stats.Free != 0 || p.Stats.Days != 3 || len(p.Public()) != 0 || !strings.Contains(strings.Join(p.Warnings, "\n"), "voucher 2: available from day 5, after the voucher phase (3 days), unused") {
		t.Fatalf("late extra: %+v %v", p.Stats, p.Warnings)
	}
	// A pin after the end does not bring back a person who left.
	ps := ordered("a", "b", "c", "d")
	ps[2].Out = true
	e = mkEvent(2, ps...)
	e.End = "2026-11-03"
	if got := labels(mustCompute(t, e, []Pin{{Label: "c", Day: 9, Voucher: 1}}).Slots); !reflect.DeepEqual(got, []string{"a", "b", "d"}) {
		t.Fatalf("leaver revived by a pin after the end: %v", got)
	}
	if got := labels(mustCompute(t, e, []Pin{{Label: "c", Day: 3, Voucher: 1}}).Slots); !reflect.DeepEqual(got, []string{"a", "b", "c", "d"}) {
		t.Fatalf("leaver pinned on the last day must stay: %v", got)
	}
	// Start and end on one weekend without weekend days: no plan day at all.
	e = mkEvent(1, ordered("a")...)
	e.Start, e.End = "2026-10-31", "2026-11-01"
	if _, err := Compute(e, nil); err == nil || !strings.Contains(err.Error(), "no plan day") {
		t.Fatalf("want no-plan-day error, got %v", err)
	}
}

func TestDrawCountsTheLot(t *testing.T) {
	e := mkEvent(2,
		Person{ID: "a", Order: 1}, Person{ID: "b"}, Person{ID: "c", Out: true}, Person{ID: "d"},
		Person{ID: "e", Tier: TierGuest}, Person{ID: "f", Tier: TierGuest, Plus: 1},
	)
	d := e.Draw()
	want := DrawExport{Seed: "t", Lineups: "12", Tiers: []DrawTier{
		{Tier: "member", Drawn: 3, Fixed: 1, Orders: "6"},
		{Tier: "guest", Drawn: 2, Orders: "2"},
		{Tier: "acquaintance", Orders: "1"},
	}}
	if !reflect.DeepEqual(d, want) {
		t.Fatalf("got %+v, want %+v", d, want)
	}
	e.Seed = "dice"
	for i := 0; i < 30; i++ {
		e.People = append(e.People, Person{ID: fmt.Sprintf("q%02d", i), Tier: TierAcquaintance})
	}
	// 30! has 33 digits: no float on the way, the page shows it exactly.
	if d = e.Draw(); d.Seed != "dice" || d.Tiers[2].Orders != "265252859812191058636308480000000" || d.Lineups != "3183034317746292703635701760000000" { // 30! · 3! · 2!
		t.Fatalf("got %+v", d)
	}
}
