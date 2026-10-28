// SPDX-License-Identifier: GPL-3.0-or-later

package plan

// Edge costs between the holder of a voucher and the next person in the chain.
const (
	costKeep      = 0   // the voucher stays with a group: one of its slots to the next
	costStart     = 1   // a voucher fresh from the organisers: dearer than keeping one
	costPass      = 2   // handed to someone else
	costForbidden = 200 // sponsor → own +k (or +j → +k) without the group flag
)

// edgeCost prices one handover. +k slots stand in for their sponsor: they share
// the sponsor's contact address. A chain that ends costs nothing.
func edgeCost(e *Event, from, to *Slot) int {
	switch {
	case to == nil:
		return 0
	case from == nil:
		return costStart
	}
	if from.PersonID != to.PersonID {
		return costPass
	}
	if p := e.Person(from.PersonID); p != nil && p.Group {
		return costKeep
	}
	return costForbidden
}

// idleTie scales edgeCost so that one more unit breaks ties: a handover
// across an idle day costs that unit, so between equal plans the one where
// a voucher does not idle wins (it otherwise ties with the look at
// tomorrow's pin). Larger than the idle edges compared at once: two per
// voucher and day (TestIdleTieOnlyBreaksTies).
const idleTie = 1024

// assignCost is edgeCost with the idle tie-break, for the assignment.
func assignCost(e *Event, from, to *Slot) int {
	c := edgeCost(e, from, to) * idleTie
	if from != nil && to != nil && to.Day > from.Day+1 {
		c++
	}
	return c
}

// prevHolder returns the last slot in chain v before day d (1-based), or nil.
func prevHolder(grid [][]*Slot, d, v int) *Slot {
	for i := d - 2; i >= 0; i-- {
		if s := grid[i][v-1]; s != nil {
			return s
		}
	}
	return nil
}

// nextHolder returns the first slot in chain v after day d, or nil.
func nextHolder(grid [][]*Slot, d, v int) *Slot {
	for i := d; i < len(grid); i++ {
		if s := grid[i][v-1]; s != nil {
			return s
		}
	}
	return nil
}

// assignChains gives every unpinned slot a voucher. Day by day, the free
// vouchers are matched to the day's unpinned slots with minimal handover cost
// against the previous holders (Hungarian); day 1 takes priority order.
//
// Without pins and idle days this is already the global optimum: the next
// day can be permuted freely, so its best cost does not depend on today's
// arrangement. Pins, idle days and chain ends break that, so a 2-opt pass
// follows: swap two unpinned slots within a day whenever that lowers the
// cost of the edges around them, until nothing improves.
func assignChains(e *Event, slots []*Slot, grid [][]*Slot, from []int) {
	for d := 1; d <= len(grid); d++ {
		var free []int
		for v := 1; v <= len(from); v++ {
			if from[v-1] <= d && grid[d-1][v-1] == nil {
				free = append(free, v)
			}
		}
		var todo []*Slot
		for _, s := range slots {
			if !s.Pinned && s.Day == d {
				todo = append(todo, s)
			}
		}
		if len(todo) == 0 {
			continue
		}
		n := len(free)
		cost := make([][]int, n)
		for i, v := range free {
			cost[i] = make([]int, n)
			prev := prevHolder(grid, d, v)
			var next *Slot // a pin tomorrow is the next holder for sure
			if d < len(grid) {
				next = grid[d][v-1]
			}
			for j := range todo {
				cost[i][j] = assignCost(e, prev, todo[j]) + assignCost(e, todo[j], next)
			}
			for j := len(todo); j < n; j++ { // dummies: nobody today, a pin tomorrow receives from prev
				cost[i][j] = assignCost(e, prev, next)
			}
		}
		var match []int
		if d == 1 {
			match = make([]int, n)
			for i := range match {
				match[i] = i
			}
		} else {
			match = hungarian(cost)
		}
		for i, v := range free {
			if j := match[i]; j < len(todo) {
				todo[j].Voucher = v
				grid[d-1][v-1] = todo[j]
			}
		}
	}
	improve(e, grid)
}

// improve is the 2-opt pass described on assignChains.
func improve(e *Event, grid [][]*Slot) {
	for round := 0; round < 100; round++ {
		changed := false
		for d := 1; d <= len(grid); d++ {
			row := grid[d-1]
			for a := 0; a < len(row); a++ {
				for b := a + 1; b < len(row); b++ {
					sa, sb := row[a], row[b]
					if sa == nil || sb == nil || sa.Pinned || sb.Pinned {
						continue
					}
					pa, pb := prevHolder(grid, d, a+1), prevHolder(grid, d, b+1)
					na, nb := nextHolder(grid, d, a+1), nextHolder(grid, d, b+1)
					before := assignCost(e, pa, sa) + assignCost(e, pb, sb) + assignCost(e, sa, na) + assignCost(e, sb, nb)
					after := assignCost(e, pa, sb) + assignCost(e, pb, sa) + assignCost(e, sb, na) + assignCost(e, sa, nb)
					if after < before {
						row[a], row[b] = sb, sa
						sa.Voucher, sb.Voucher = b+1, a+1
						changed = true
					}
				}
			}
		}
		if !changed {
			return
		}
	}
}
