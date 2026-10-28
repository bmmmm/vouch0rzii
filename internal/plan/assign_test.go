// SPDX-License-Identifier: GPL-3.0-or-later

package plan

import (
	"math/rand/v2"
	"strings"
	"testing"
)

func totalCost(cost [][]int, match []int) int {
	sum := 0
	for i, j := range match {
		sum += cost[i][j]
	}
	return sum
}

func bruteForce(cost [][]int) int {
	n, m := len(cost), len(cost[0])
	perm := make([]int, n)
	used := make([]bool, m)
	best, found := 0, false
	var rec func(i, acc int)
	rec = func(i, acc int) {
		if i == n {
			if !found || acc < best {
				best, found = acc, true
			}
			return
		}
		for j := 0; j < m; j++ {
			if !used[j] {
				used[j] = true
				perm[i] = j
				rec(i+1, acc+cost[i][j])
				used[j] = false
			}
		}
	}
	rec(0, 0)
	return best
}

// The assignment hands the Hungarian fewer rows (slots) than columns
// (vouchers) and costs relative to idling, so negative ones.
func TestHungarianRectangularNegative(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 4))
	for n := 1; n <= 5; n++ {
		for m := n; m <= n+3; m++ {
			for range 40 {
				cost := make([][]int, n)
				for i := range cost {
					cost[i] = make([]int, m)
					for j := range cost[i] {
						cost[i][j] = rng.IntN(101) - 50
					}
				}
				match := hungarian(cost)
				seen := map[int]bool{}
				for _, j := range match {
					if seen[j] {
						t.Fatalf("column used twice: %v", match)
					}
					seen[j] = true
				}
				if got, want := totalCost(cost, match), bruteForce(cost); got != want {
					t.Fatalf("%d×%d cost=%v: hungarian %d, brute force %d", n, m, cost, got, want)
				}
			}
		}
	}
}

func TestHungarianKnownOptimum(t *testing.T) {
	cost := [][]int{{4, 1, 3}, {2, 0, 5}, {3, 2, 2}}
	m := hungarian(cost)
	if got := totalCost(cost, m); got != 5 {
		t.Fatalf("total %d want 5 (match %v)", got, m)
	}
	seen := map[int]bool{}
	for _, j := range m {
		if seen[j] {
			t.Fatalf("column used twice: %v", m)
		}
		seen[j] = true
	}
}

func TestHungarianMatchesBruteForce(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	for n := 1; n <= 6; n++ {
		for round := 0; round < 40; round++ {
			cost := make([][]int, n)
			for i := range cost {
				cost[i] = make([]int, n)
				for j := range cost[i] {
					cost[i][j] = []int{0, 1, 100}[rng.IntN(3)]
				}
			}
			if got, want := totalCost(cost, hungarian(cost)), bruteForce(cost); got != want {
				t.Fatalf("n=%d cost=%v: hungarian %d, brute force %d", n, cost, got, want)
			}
		}
	}
}

func ordered(ids ...string) []Person {
	var out []Person
	for i, id := range ids {
		out = append(out, Person{ID: id, Order: i + 1})
	}
	return out
}

func cell(p *Plan, day, voucher int) string {
	if s := p.Grid[day-1][voucher-1]; s != nil {
		return s.Label
	}
	return ""
}

func TestForbiddenEdgeAvoidedWhenPossible(t *testing.T) {
	ps := ordered("a", "b")
	ps[0].Plus, ps[1].Plus = 1, 1
	p := mustCompute(t, mkEvent(2, ps...), nil)
	if cell(p, 2, 1) != "b+1" || cell(p, 2, 2) != "a+1" {
		t.Fatalf("day 2: got %s/%s want b+1/a+1", cell(p, 2, 1), cell(p, 2, 2))
	}
	if p.Stats.Forbidden != 0 {
		t.Fatalf("forbidden handover used: %+v", p.Stats)
	}
}

func TestGroupKeepsItsVoucher(t *testing.T) {
	ps := ordered("a", "b")
	ps[0].Plus, ps[0].Group = 2, true
	p := mustCompute(t, mkEvent(1, ps...), nil)
	want := []string{"a", "a+1", "a+2", "b"}
	for d, l := range want {
		if cell(p, d+1, 1) != l {
			t.Fatalf("day %d: got %s want %s", d+1, cell(p, d+1, 1), l)
		}
	}
	if p.Stats.Forbidden != 0 || p.Stats.Kept != 2 || p.Stats.Handovers != 3 {
		t.Fatalf("stats %+v", p.Stats)
	}
}

func TestForbiddenEdgeUnavoidableWarns(t *testing.T) {
	ps := ordered("a")
	ps[0].Plus = 1
	p := mustCompute(t, mkEvent(1, ps...), nil)
	if p.Stats.Forbidden != 1 {
		t.Fatalf("stats %+v", p.Stats)
	}
	if !strings.Contains(strings.Join(p.Warnings, "\n"), "hands directly") {
		t.Fatalf("warnings %v", p.Warnings)
	}
}

// q is pinned on day 3 and q+1 is day 2's only slot: the assignment keeps q+1
// off q's voucher, whichever it is.
func TestAssignLooksAtTomorrowsPin(t *testing.T) {
	for v := 1; v <= 2; v++ {
		ps := ordered("a", "b", "q")
		ps[2].Plus = 1
		p := mustCompute(t, mkEvent(2, ps...), []Pin{{Label: "q", Day: 3, Voucher: v}})
		if p.Stats.Forbidden != 0 || cell(p, 2, 3-v) != "q+1" {
			t.Fatalf("q on voucher %d: %v, stats %+v", v, p.ToExport().Grid, p.Stats)
		}
	}
}

// a and b tie on day 1, but a+1 is day 2's only free slot on voucher 1 (x is
// pinned on voucher 2): a must not take voucher 1, or it hands straight to
// a+1. The look at tomorrow's slots breaks the tie.
func TestTieLooksAtTomorrowsSlots(t *testing.T) {
	ps := ordered("a", "b", "x")
	for i := range ps {
		ps[i].Plus = 1
	}
	p := mustCompute(t, mkEvent(2, ps...), []Pin{{Label: "x", Day: 2, Voucher: 2}, {Label: "x+1", Day: 3, Voucher: 1}})
	if p.Stats.Forbidden != 0 {
		t.Fatalf("%v, stats %+v", p.ToExport().Grid, p.Stats)
	}
}

// The tomorrow tier takes the cheapest edge to any of tomorrow's slots.
func TestTomorrowCostTakesTheCheapest(t *testing.T) {
	ps := ordered("a", "c")
	ps[0].Plus = 1
	e := mkEvent(1, ps...)
	if _, err := Normalize(e); err != nil {
		t.Fatal(err)
	}
	a, a1, c := &Slot{PersonID: "a"}, &Slot{PersonID: "a", K: 1}, &Slot{PersonID: "c"}
	if got := tomorrowCost(e, a, []*Slot{a1, c}); got != costPass*tomorrowScale {
		t.Fatalf("a before a+1 or c: %d", got)
	}
	if got := tomorrowCost(e, a, []*Slot{a1}); got != costForbidden*tomorrowScale {
		t.Fatalf("a before a+1 only: %d", got)
	}
}

// Each tier of the assignment outweighs all a day can add up below it: per
// slot four single units (three idle edges, day 1's order) and two tomorrow
// edges, for up to MaxVouchers slots.
func TestTieScales(t *testing.T) {
	if units := 4 * MaxVouchers; units >= tomorrowScale {
		t.Fatalf("%d single units reach tomorrowScale %d", units, tomorrowScale)
	}
	if below := MaxVouchers * (2*costForbidden*tomorrowScale + 4); below >= costScale {
		t.Fatalf("%d below reaches costScale %d", below, costScale)
	}
}

// p is pinned on day 3: x takes p's voucher on day 2 instead of leaving it
// idle before the pin, whichever voucher that is.
func TestNoIdleDayBeforeAPin(t *testing.T) {
	for v := 1; v <= 2; v++ {
		p := mustCompute(t, mkEvent(2, ordered("a", "b", "x", "p")...), []Pin{{Label: "p", Day: 3, Voucher: v}})
		if cell(p, 2, v) != "x" {
			t.Fatalf("p on voucher %d: %v", v, p.ToExport().Grid)
		}
	}
}

// A group goes on with its own voucher rather than one fresh from the
// organisers with a lower number (extras listed out of date order).
func TestGroupPrefersItsVoucherToAFreshOne(t *testing.T) {
	ps := ordered("b", "a")
	ps[0].Plus, ps[0].Group = 3, true
	ps[1].Plus, ps[1].Group = 1, true
	e := mkEvent(1, ps...)
	e.OneChain, e.End = true, "2026-11-11"
	e.Extra = []Extra{{Date: "2026-11-03", Count: 2}, {Date: "2026-11-01", Count: 1}} // vouchers 2, 3 from day 3, voucher 4 from day 2
	p := mustCompute(t, e, nil)
	for _, id := range []string{"a", "b"} {
		if _, vouchers := daysOf(p, id); len(vouchers) != 1 {
			t.Fatalf("%s on vouchers %v: %v", id, vouchers, p.ToExport().Grid)
		}
	}
}

// Day 1 follows the priority order, so a would take voucher 1 and hand it
// straight to a+1, pinned there on day 2; the look at the pin swaps a and b.
func TestDayOneLooksAtPins(t *testing.T) {
	ps := ordered("a", "b", "c")
	ps[0].Plus = 1
	p := mustCompute(t, mkEvent(2, ps...), []Pin{{Label: "a+1", Day: 2, Voucher: 1}})
	if cell(p, 1, 1) != "b" || cell(p, 1, 2) != "a" {
		t.Fatalf("day 1: got %s/%s want b/a", cell(p, 1, 1), cell(p, 1, 2))
	}
	if p.Stats.Forbidden != 0 {
		t.Fatalf("stats %+v", p.Stats)
	}
}
