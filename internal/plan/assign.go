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

// The assignment ranks plans in three tiers, each scale above everything the
// tiers below add up to in one day (TestTieScales): edgeCost first, then
// tomorrow (the cheapest handover a voucher free tomorrow could make to one
// of tomorrow's slots, so a tie today does not force a bad handover
// tomorrow), then single units: a handover across an idle day, and day 1
// out of priority order.
const (
	tomorrowScale = 1 << 10
	costScale     = 1 << 27
)

// assignCost is edgeCost with the idle tie-break, for the assignment.
func assignCost(e *Event, from, to *Slot) int {
	c := edgeCost(e, from, to) * costScale
	if from != nil && to != nil && to.Day > from.Day+1 {
		c++
	}
	return c
}

// tomorrowCost is the tomorrow tier for holder s of a voucher that is free
// tomorrow: its cheapest edge to one of tomorrow's unpinned slots.
func tomorrowCost(e *Event, s *Slot, tomorrow []*Slot) int {
	best := 0
	for i, t := range tomorrow {
		if c := edgeCost(e, s, t); i == 0 || c < best {
			best = c
		}
	}
	return best * tomorrowScale
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

// assignChains gives every unpinned slot a voucher, day by day: the day's
// unpinned slots are matched to the free vouchers (Hungarian) with minimal
// handover cost from each voucher's previous holder and into its next pin,
// against the cost of the voucher taking nobody today; ties go by tomorrow's
// slots (see costScale). A day depends only on the days before it, the
// pins and the slots' days, so pinning the past does not change the rest of
// the plan. Day 1 keeps priority order unless a pin says otherwise.
func assignChains(e *Event, slots []*Slot, grid [][]*Slot, from, until []int) {
	for d := 1; d <= len(grid); d++ {
		var free []int
		for v := 1; v <= len(from); v++ {
			if usable(from, until, v, d) && grid[d-1][v-1] == nil {
				free = append(free, v)
			}
		}
		var todo, tomorrow []*Slot // tomorrow's slots have their day already (placeSlots, chainGroups)
		for _, s := range slots {
			switch {
			case s.Pinned:
			case s.Day == d:
				todo = append(todo, s)
			case s.Day == d+1:
				tomorrow = append(tomorrow, s)
			}
		}
		if len(todo) == 0 {
			continue
		}
		prev, next := make([]*Slot, len(free)), make([]*Slot, len(free))
		idle, open := make([]int, len(free)), make([]bool, len(free))
		for i, v := range free {
			prev[i], next[i] = prevHolder(grid, d, v), nextHolder(grid, d, v) // only pins lie ahead yet
			open[i] = d < len(grid) && grid[d][v-1] == nil && usable(from, until, v, d+1)
			idle[i] = assignCost(e, prev[i], next[i])
		}
		cost := make([][]int, len(todo)) // slot j on voucher free[i], against that voucher idling
		for j, s := range todo {
			cost[j] = make([]int, len(free))
			for i := range free {
				cost[j][i] = assignCost(e, prev[i], s) + assignCost(e, s, next[i]) - idle[i]
				if open[i] {
					cost[j][i] += tomorrowCost(e, s, tomorrow)
				}
				if d == 1 && i != j {
					cost[j][i]++
				}
			}
		}
		for j, i := range hungarian(cost) {
			todo[j].Voucher = free[i]
			grid[d-1][free[i]-1] = todo[j]
		}
	}
}
