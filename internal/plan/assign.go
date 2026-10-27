// SPDX-License-Identifier: GPL-3.0-or-later

package plan

// Edge costs between the holder of a voucher and the next person in the chain.
const (
	costKnown     = 0   // the two know each other (or group block)
	costUnknown   = 1   // strangers
	costForbidden = 100 // sponsor → own +k (or +j → +k) without the group flag
)

// edgeCost prices one handover. +k slots stand in for their sponsor: they share
// the sponsor's connections and contact address.
func edgeCost(e *Event, from, to *Slot) int {
	if from == nil || to == nil {
		return 0
	}
	if from.PersonID == to.PersonID {
		if p := e.Person(from.PersonID); p != nil && p.Group {
			return costKnown
		}
		return costForbidden
	}
	if e.Knows(from.PersonID, to.PersonID) {
		return costKnown
	}
	return costUnknown
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
// Without pins this is already the global optimum: the next day can be
// permuted freely, so its best cost does not depend on today's arrangement.
// Pins and chain ends break that, so a 2-opt pass follows: swap two unpinned
// slots within a day whenever that lowers the cost of the edges around them,
// until nothing improves.
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
			for j := range todo {
				cost[i][j] = edgeCost(e, prev, todo[j])
			}
			// columns beyond len(todo) are dummies: the chain ends, cost 0
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
					before := edgeCost(e, pa, sa) + edgeCost(e, pb, sb) + edgeCost(e, sa, na) + edgeCost(e, sb, nb)
					after := edgeCost(e, pa, sb) + edgeCost(e, pb, sa) + edgeCost(e, sb, na) + edgeCost(e, sa, nb)
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
