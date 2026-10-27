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

func TestAssignPrefersKnownPairs(t *testing.T) {
	ps := ordered("a", "b", "c", "d")
	ps[2].Tier, ps[3].Tier = TierGuest, TierGuest
	ps[0].Knows = []string{"d"}
	ps[1].Knows = []string{"c"}
	p := mustCompute(t, mkEvent(2, ps...), nil)
	if cell(p, 2, 1) != "d" || cell(p, 2, 2) != "c" {
		t.Fatalf("day 2: got %s/%s want d/c", cell(p, 2, 1), cell(p, 2, 2))
	}
	if p.Stats.Known != 2 || p.Stats.Unknown != 0 {
		t.Fatalf("stats %+v", p.Stats)
	}
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

func TestGroupBlockIsKnownNotForbidden(t *testing.T) {
	ps := ordered("a", "b")
	ps[0].Plus, ps[0].Group = 2, true
	p := mustCompute(t, mkEvent(1, ps...), nil)
	want := []string{"a", "b", "a+1", "a+2"}
	for d, l := range want {
		if cell(p, d+1, 1) != l {
			t.Fatalf("day %d: got %s want %s", d+1, cell(p, d+1, 1), l)
		}
	}
	if p.Stats.Forbidden != 0 || p.Stats.Known != 1 || p.Stats.Unknown != 2 {
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

// With a pin on day 3, the greedy day-2 choice (a→c known) costs two known
// handovers on day 3; the 2-opt pass must swap c and d on day 2 (and then a and b on day 1).
func TestImproveLooksPastPins(t *testing.T) {
	ps := ordered("a", "b", "c", "d", "e", "f")
	ps[0].Knows = []string{"c"}
	ps[4].Knows = []string{"d"}
	ps[5].Knows = []string{"c"}
	p := mustCompute(t, mkEvent(2, ps...), []Pin{{Label: "e", Day: 3, Voucher: 1}})
	if cell(p, 2, 1) != "d" || cell(p, 2, 2) != "c" {
		t.Fatalf("day 2: got %s/%s want d/c", cell(p, 2, 1), cell(p, 2, 2))
	}
	if p.Stats.Known != 3 || p.Stats.Unknown != 1 {
		t.Fatalf("stats %+v", p.Stats)
	}
}

func TestPlusSlotsInheritSponsorConnections(t *testing.T) {
	ps := ordered("a", "b", "c")
	ps[0].Plus = 1
	ps[2].Tier = TierGuest
	ps[1].Knows = []string{"a"}
	// Day 1: a, b. Day 2: c, a+1. b knows a, so b → a+1 is known; a → c unknown.
	p := mustCompute(t, mkEvent(2, ps...), nil)
	if cell(p, 2, 2) != "a+1" || p.Stats.Known != 1 {
		t.Fatalf("got day 2 %s/%s, stats %+v", cell(p, 2, 1), cell(p, 2, 2), p.Stats)
	}
}
