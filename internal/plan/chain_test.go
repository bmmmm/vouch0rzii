// SPDX-License-Identifier: GPL-3.0-or-later

package plan

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"slices"
	"testing"
)

// groupEvent: g (group, +4) first, then a, b, c; two vouchers. Plain
// placement would give g's +N days 3 and 4 on both vouchers.
func groupEvent() *Event {
	ps := ordered("g", "a", "b", "c")
	ps[0].Plus, ps[0].Group = 4, true
	return mkEvent(2, ps...)
}

// daysOf returns the day of every slot by label, and the vouchers person
// id holds.
func daysOf(p *Plan, id string) (days map[string]int, vouchers map[int]bool) {
	days, vouchers = map[string]int{}, map[int]bool{}
	for _, s := range p.Slots {
		days[s.Label] = s.Day
		if s.PersonID == id {
			vouchers[s.Voucher] = true
		}
	}
	return days, vouchers
}

func TestGroupRunsOnFewestVouchers(t *testing.T) {
	p := mustCompute(t, groupEvent(), nil)
	days, vouchers := daysOf(p, "g")
	want := map[string]int{"g": 1, "a": 1, "g+1": 2, "b": 2, "g+2": 3, "c": 3, "g+3": 4, "g+4": 4}
	if !reflect.DeepEqual(days, want) || len(vouchers) != 2 || p.Stats.Days != 4 || p.Stats.Kept != 3 {
		t.Fatalf("days %v, g on %d vouchers, stats %+v", days, len(vouchers), p.Stats)
	}
}

func TestGroupKeepsOneVoucher(t *testing.T) {
	e := groupEvent()
	e.OneChain = true
	p := mustCompute(t, e, nil)
	days, vouchers := daysOf(p, "g")
	want := map[string]int{"g": 1, "a": 1, "g+1": 2, "b": 2, "g+2": 3, "c": 3, "g+3": 4, "g+4": 5}
	if !reflect.DeepEqual(days, want) || len(vouchers) != 1 || p.Stats.Days != 5 || p.Stats.Kept != 4 {
		t.Fatalf("days %v, g on %d vouchers, stats %+v", days, len(vouchers), p.Stats)
	}
	// The end of the voucher phase caps the chain: two vouchers again,
	// nobody in the public sale.
	e = groupEvent()
	e.OneChain, e.End = true, "2026-11-04" // day 4
	p = mustCompute(t, e, nil)
	if _, vouchers = daysOf(p, "g"); len(vouchers) != 2 || p.Stats.Days != 4 || len(p.Public()) != 0 {
		t.Fatalf("g on %d vouchers, days %d, public %v", len(vouchers), p.Stats.Days, p.Public())
	}
}

// membersFirstEvent: groupEvent with g at +6, the guests x and y, and
// MembersFirst.
func membersFirstEvent() *Event {
	e := groupEvent()
	e.People[0].Plus = 6
	e.People = append(e.People, Person{ID: "x", Tier: TierGuest, Order: 1}, Person{ID: "y", Tier: TierGuest, Order: 2})
	e.MembersFirst = true
	return e
}

// With MembersFirst the members b and c buy on day 2 as without groups; g's
// chain starts on day 3, before the guest y, on as few vouchers as six days
// allow.
func TestGroupWaitsForTheMembers(t *testing.T) {
	p := keepsThePast(t, "members first", membersFirstEvent, nil)
	days, _ := daysOf(p, "g")
	want := map[string]int{"g": 1, "a": 1, "b": 2, "c": 2, "g+1": 3, "x": 3, "g+2": 4, "y": 4, "g+3": 5, "g+4": 5, "g+5": 6, "g+6": 6}
	if !reflect.DeepEqual(days, want) || p.Stats.Days != 6 || p.Stats.Kept != 4 {
		t.Fatalf("days %v, stats %+v", days, p.Stats)
	}
	// One voucher: g+1 to g+6 on days 3 to 8.
	p = keepsThePast(t, "members first, one voucher", func() *Event {
		e := membersFirstEvent()
		e.OneChain = true
		return e
	}, nil)
	days, _ = daysOf(p, "g")
	want = map[string]int{"g": 1, "a": 1, "b": 2, "c": 2, "g+1": 3, "x": 3, "g+2": 4, "y": 4, "g+3": 5, "g+4": 6, "g+5": 7, "g+6": 8}
	if !reflect.DeepEqual(days, want) || p.Stats.Days != 8 || p.Stats.Kept != 5 {
		t.Fatalf("one voucher: days %v, stats %+v", days, p.Stats)
	}
}

// A member pinned by hand to the plan's last day comes first too: g's chain
// would start after it, so its slots keep their turn.
func TestLateMemberHoldsTheGroup(t *testing.T) {
	p := keepsThePast(t, "late member", func() *Event {
		e := groupEvent()
		e.MembersFirst = true
		return e
	}, []Pin{{Label: "c", Day: 5, Voucher: 1}})
	days, _ := daysOf(p, "g")
	want := map[string]int{"g": 1, "a": 1, "b": 2, "g+1": 2, "g+2": 3, "g+3": 3, "g+4": 4, "c": 5}
	if !reflect.DeepEqual(days, want) {
		t.Fatalf("days %v", days)
	}
}

// Pinning the first days of a plan, as "Pin until today" does, keeps the
// rest of it.
func TestGroupChainsSurvivePinningThePast(t *testing.T) {
	for _, one := range []bool{false, true} {
		ps := ordered("a", "g", "b", "c", "d", "e")
		ps[1].Plus, ps[1].Group = 6, true
		ps[3].Plus = 1
		e := mkEvent(3, ps...)
		e.OneChain = one
		p1 := mustCompute(t, e, nil)
		var past []Pin
		for _, pin := range Freeze(p1) {
			if pin.Day <= 2 {
				past = append(past, pin)
			}
		}
		e = mkEvent(3, ps...)
		e.OneChain = one
		if p2 := mustCompute(t, e, past); !reflect.DeepEqual(p1.ToExport().Grid, p2.ToExport().Grid) {
			t.Fatalf("oneChain %v: pinning days 1-2 changed the plan:\n%v\n%v", one, p1.ToExport().Grid, p2.ToExport().Grid)
		}
	}
}

// A day full of pins takes no group slot; the chain goes on the next day
// and the plan keeps every slot.
func TestGroupChainSkipsFullDays(t *testing.T) {
	pins := []Pin{{Label: "a", Day: 2, Voucher: 1}, {Label: "b", Day: 2, Voucher: 2}}
	p := mustCompute(t, groupEvent(), pins)
	days, _ := daysOf(p, "g")
	if days["g"] != 1 || days["g+1"] != 3 || p.Stats.Slots != 8 || len(p.Public()) != 0 {
		t.Fatalf("days %v stats %+v", days, p.Stats)
	}
	for _, s := range p.Slots {
		if s.Voucher == 0 {
			t.Fatalf("%s has no voucher: %v", s.Label, p.ToExport().Grid)
		}
	}
}

// A group slot pinned by hand is part of the chain: the next one follows a
// day later, not on the same day.
func TestPinnedGroupSlotTakesItsDay(t *testing.T) {
	p := mustCompute(t, groupEvent(), []Pin{{Label: "g+1", Day: 2, Voucher: 1}})
	days, vouchers := daysOf(p, "g")
	if days["g+2"] != 3 || days["b"] != 2 || len(vouchers) != 2 || p.Stats.Kept != 3 {
		t.Fatalf("days %v, g on %d vouchers, stats %+v", days, len(vouchers), p.Stats)
	}
	// Two pinned on a day with one target: the free cell left that day
	// does not take a third.
	ps := ordered("g", "a", "b", "c", "d", "e", "f")
	ps[0].Plus, ps[0].Group = 4, true
	p = mustCompute(t, mkEvent(3, ps...), []Pin{{Label: "g+1", Day: 2, Voucher: 1}, {Label: "g+2", Day: 2, Voucher: 2}})
	if days, _ = daysOf(p, "g"); days["g+3"] != 3 || days["c"] != 2 {
		t.Fatalf("days %v", days)
	}
}

// keepsThePast computes mk() with the manual pins and checks that pinning
// every day 1..t, as "Pin until today" does on day t, keeps the plan for
// every t. Manual pins after t stay pins.
func keepsThePast(t *testing.T, name string, mk func() *Event, manual []Pin) *Plan {
	t.Helper()
	p := mustCompute(t, mk(), manual)
	for day := 1; day <= p.Stats.Days; day++ {
		var pins []Pin
		for _, pin := range Freeze(p) {
			if pin.Day <= day {
				pins = append(pins, pin)
			}
		}
		for _, pin := range manual {
			if pin.Day > day {
				pins = append(pins, pin)
			}
		}
		if again := mustCompute(t, mk(), pins); !reflect.DeepEqual(again.ToExport().Grid, p.ToExport().Grid) {
			t.Fatalf("%s: pinning days 1-%d changed the plan:\n%v\n%v", name, day, p.ToExport().Grid, again.ToExport().Grid)
		}
	}
	return p
}

// A later group starts its chain when its first slot is due, after the
// people drawn before it, and pinning the past keeps the rest.
func TestTwoGroupsKeepTheOrderAndThePast(t *testing.T) {
	mk := func() *Event {
		ps := ordered("a", "b", "c")
		ps[0].Plus, ps[0].Group = 2, true
		ps[2].Plus, ps[2].Group = 1, true
		return mkEvent(1, ps...)
	}
	p := keepsThePast(t, "two groups", mk, nil)
	want := [][]string{{"a"}, {"a+1"}, {"a+2"}, {"b"}, {"c"}, {"c+1"}}
	if got := p.ToExport().Grid; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

// A group slot that found no target does not start a second chain from its
// own day: where it lands would depend on which days are pinned.
func TestGroupChainsOnlyOnce(t *testing.T) {
	keepsThePast(t, "leftovers", func() *Event {
		ps := ordered("c", "a", "b")
		ps[1].Plus, ps[1].Group = 10, true
		ps[2].Plus, ps[2].Group = 13, true
		return mkEvent(3, ps...)
	}, nil)
}

// A group pinned by hand starts its chain on the day of its first pin, not
// on the day its first slot in priority order is pinned.
func TestGroupChainStartsAtTheFirstPin(t *testing.T) {
	keepsThePast(t, "first pin", func() *Event {
		ps := ordered("a", "b", "c", "g")
		ps[0].Plus = 2
		ps[3].Plus, ps[3].Group = 4, true
		return mkEvent(2, ps...)
	}, []Pin{{Label: "g", Day: 4, Voucher: 2}, {Label: "b", Day: 3, Voucher: 1}, {Label: "g+4", Day: 3, Voucher: 2}})
}

// Group slots cut by the end of the voucher phase stay in the public sale.
func TestGroupSlotsInThePublicSale(t *testing.T) {
	e := groupEvent()
	e.End = "2026-11-03" // day 3: six of eight slots fit
	p := mustCompute(t, e, nil)
	if !reflect.DeepEqual(p.Public(), []string{"g+3", "g+4"}) || p.Stats.Days != 3 {
		t.Fatalf("public %v, days %d", p.Public(), p.Stats.Days)
	}
}

// randomCase builds random roster number round with several groups, and up
// to three of its slots pinned by hand to cells anywhere in its plan.
func randomCase(t *testing.T, round uint64) (mk func() *Event, manual []Pin) {
	mk = func() *Event {
		r := rand.New(rand.NewPCG(round, 3))
		e := mkEvent(1 + r.IntN(4))
		e.Seed = fmt.Sprint(round)
		for i := 0; i < 3+r.IntN(10); i++ {
			p := Person{ID: fmt.Sprintf("p%02d", i), Tier: Tier(r.IntN(3)), Plus: r.IntN(3), Out: r.IntN(10) == 0}
			if r.IntN(3) == 0 {
				p.Group, p.Plus = true, r.IntN(15)
			}
			e.People = append(e.People, p)
		}
		if r.IntN(3) == 0 {
			e.Extra = []Extra{{Date: "2026-11-04", Count: 1 + r.IntN(2)}}
		}
		if r.IntN(3) == 0 {
			e.End = []string{"2026-11-05", "2026-11-10", "2026-11-20"}[r.IntN(3)]
		}
		e.Weekends = r.IntN(4) == 0
		e.OneChain = r.IntN(2) == 0
		e.MembersFirst = r.IntN(2) == 0
		return e
	}
	r := rand.New(rand.NewPCG(round, 5))
	p0 := mustCompute(t, mk(), nil)
	taken := map[[2]int]bool{}
	for i := r.IntN(4); i > 0 && len(p0.Slots) > 0; i-- {
		s, day, v := p0.Slots[r.IntN(len(p0.Slots))], 1+r.IntN(p0.Stats.Days), 1+r.IntN(len(p0.From))
		if s.Day == 0 || p0.From[v-1] > day || taken[[2]int{day, v}] || slices.ContainsFunc(manual, func(m Pin) bool { return m.Label == s.Label }) {
			continue
		}
		taken[[2]int{day, v}] = true
		manual = append(manual, Pin{Label: s.Label, Day: day, Voucher: v})
	}
	return mk, manual
}

// Random rosters with several groups, some with pins by hand: every placed
// slot sits on a voucher that exists that day, the same slots are placed as
// without chains, with MembersFirst every member's own slot on the same day,
// the default mode keeps the plan's length, and pinning the days up to any
// day keeps the plan, pins by hand after that day included.
func TestGroupChainsProperties(t *testing.T) {
	for round := range uint64(1500) {
		mk, manual := randomCase(t, round)
		p := keepsThePast(t, fmt.Sprintf("round %d", round), mk, manual)
		for _, s := range p.Slots {
			if s.Day > 0 && (s.Voucher == 0 || p.Grid[s.Day-1][s.Voucher-1] != s || p.From[s.Voucher-1] > s.Day) {
				t.Fatalf("round %d: %s on day %d voucher %d: %v", round, s.Label, s.Day, s.Voucher, p.ToExport().Grid)
			}
		}
		if len(manual) > 0 {
			continue // placeSlots below plans without pins
		}
		plain := mk()
		if _, err := Normalize(plain); err != nil {
			t.Fatal(err)
		}
		starts, _ := voucherStarts(plain)
		horizon := MaxDays
		if plain.End != "" {
			horizon = phaseDays(plain)
		}
		slots := buildSlots(plain, nil)
		grid, _, err := placeSlots(slots, nil, starts, horizon)
		if err != nil {
			t.Fatal(err)
		}
		for i, s := range slots {
			if (s.Day > 0) != (p.Slots[i].Day > 0) {
				t.Fatalf("round %d: %s placed %v with chains, %v without", round, s.Label, p.Slots[i].Day > 0, s.Day > 0)
			}
			if plain.MembersFirst && s.K == 0 && plain.Person(s.PersonID).Tier == TierMember && s.Day != p.Slots[i].Day {
				t.Fatalf("round %d: member %s on day %d, %d without chains", round, s.Label, p.Slots[i].Day, s.Day)
			}
		}
		if !p.Event.OneChain && p.Stats.Days != len(grid) {
			t.Fatalf("round %d: %d days with chains, %d without", round, p.Stats.Days, len(grid))
		}
	}
}
