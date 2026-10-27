// SPDX-License-Identifier: GPL-3.0-or-later

package plan

import (
	"fmt"
	"hash/fnv"
	"math/rand/v2"
	"sort"
)

// newRand derives a platform-independent generator from the seed string, so
// the CLI and the browser build draw the same lots.
func newRand(seed string) *rand.Rand {
	h1 := fnv.New64a()
	h1.Write([]byte(seed))
	h2 := fnv.New64a()
	h2.Write([]byte(seed))
	h2.Write([]byte{0, 's', 'a', 'l', 't'})
	return rand.New(rand.NewPCG(h1.Sum64(), h2.Sum64()))
}

// buildSlots returns all slots in priority order:
//
//  1. named people, tier by tier (member, guest, acquaintance); within a tier
//     those with a manual Order first (ascending), then the rest drawn by lot;
//  2. all +1, then all +2, … up to PlusMax, each round in the order of the
//     sponsors' ranks;
//  3. the overflow (+k beyond PlusMax, group exceptions only), by sponsor rank
//     and then k.
//
// The lot depends only on the seed and the set of IDs per tier, so adding a
// guest never reshuffles the members.
func buildSlots(e *Event) []*Slot {
	rng := newRand(e.SeedOrDefault())
	var named []*Person
	for tier := TierMember; tier <= TierAcquaintance; tier++ {
		var fixed, drawn []*Person
		for i := range e.People {
			p := &e.People[i]
			if p.Tier != tier {
				continue
			}
			if p.Order > 0 {
				fixed = append(fixed, p)
			} else {
				drawn = append(drawn, p)
			}
		}
		sort.SliceStable(fixed, func(i, j int) bool {
			if fixed[i].Order != fixed[j].Order {
				return fixed[i].Order < fixed[j].Order
			}
			return fixed[i].ID < fixed[j].ID
		})
		sort.Slice(drawn, func(i, j int) bool { return drawn[i].ID < drawn[j].ID })
		rng.Shuffle(len(drawn), func(i, j int) { drawn[i], drawn[j] = drawn[j], drawn[i] })
		named = append(named, fixed...)
		named = append(named, drawn...)
	}

	var slots []*Slot
	add := func(p *Person, k int) {
		slots = append(slots, &Slot{Label: SlotLabel(p.ID, k), PersonID: p.ID, K: k, rank: len(slots)})
	}
	for _, p := range named {
		add(p, 0)
	}
	plusMax := e.PlusMaxOrDefault()
	for k := 1; k <= plusMax; k++ {
		for _, p := range named {
			if p.Plus >= k {
				add(p, k)
			}
		}
	}
	for _, p := range named {
		for k := plusMax + 1; k <= p.Plus; k++ {
			add(p, k)
		}
	}
	return slots
}

// placeSlots fixes pinned slots to their cells and fills the remaining cells
// day by day with the unpinned slots in priority order. Vouchers of unpinned
// slots stay 0 until assignChains runs. Returns the grid and warnings about
// pins that could not be applied; conflicting pins are an error.
func placeSlots(e *Event, slots []*Slot, pins []Pin) (grid [][]*Slot, warnings []string, err error) {
	byLabel := make(map[string]*Slot, len(slots))
	for _, s := range slots {
		byLabel[s.Label] = s
	}
	v := e.Vouchers
	days := (len(slots) + v - 1) / v
	type cell struct{ d, v int }
	taken := map[cell]string{}
	for _, pin := range pins {
		s, ok := byLabel[pin.Label]
		switch {
		case !ok:
			warnings = append(warnings, fmt.Sprintf("pin %s: no such slot, dropped", pin.Label))
			continue
		case pin.Day < 1 || pin.Voucher < 1 || pin.Voucher > v:
			warnings = append(warnings, fmt.Sprintf("pin %s: cell day %d / voucher %d outside the plan, dropped", pin.Label, pin.Day, pin.Voucher))
			continue
		case s.Pinned:
			return nil, nil, fmt.Errorf("pin %s: slot pinned twice", pin.Label)
		}
		if other, dup := taken[cell{pin.Day, pin.Voucher}]; dup {
			return nil, nil, fmt.Errorf("pins %s and %s on the same cell (day %d, voucher %d)", other, pin.Label, pin.Day, pin.Voucher)
		}
		taken[cell{pin.Day, pin.Voucher}] = pin.Label
		s.Pinned, s.Day, s.Voucher = true, pin.Day, pin.Voucher
		if pin.Day > days {
			days = pin.Day
		}
	}
	if minDays := (len(slots) + v - 1) / v; days > minDays {
		warnings = append(warnings, fmt.Sprintf("pins stretch the plan to %d days, %d would do", days, minDays))
	}

	grid = make([][]*Slot, days)
	for d := range grid {
		grid[d] = make([]*Slot, v)
	}
	for _, s := range slots {
		if s.Pinned {
			grid[s.Day-1][s.Voucher-1] = s
		}
	}
	next := 0
	for d := 1; d <= days; d++ {
		free := 0
		for _, c := range grid[d-1] {
			if c == nil {
				free++
			}
		}
		for ; free > 0 && next < len(slots); next++ {
			if slots[next].Pinned {
				continue
			}
			slots[next].Day = d
			free--
		}
	}
	return grid, warnings, nil
}
