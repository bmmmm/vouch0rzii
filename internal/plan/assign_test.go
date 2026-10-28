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
	n := len(cost)
	perm := make([]int, n)
	used := make([]bool, n)
	best := -1
	var rec func(i, acc int)
	rec = func(i, acc int) {
		if i == n {
			if best < 0 || acc < best {
				best = acc
			}
			return
		}
		for j := 0; j < n; j++ {
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

// q is pinned on day 3 and q+1 is day 2's only slot, which the 2-opt pass
// cannot move: the assignment itself must keep q+1 off q's voucher, on
// either voucher.
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

// All idle units of one day's assignment (two edges per voucher) stay below
// one unit of edgeCost, so they only break ties.
func TestIdleTieOnlyBreaksTies(t *testing.T) {
	if idleTie <= 2*MaxVouchers {
		t.Fatalf("idleTie %d is not above 2 × %d vouchers", idleTie, MaxVouchers)
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

// Day 1 follows the priority order, so a holds voucher 1 and would hand it
// straight to a+1, pinned on day 2; the 2-opt pass must swap a and b.
func TestImproveLooksPastPins(t *testing.T) {
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
