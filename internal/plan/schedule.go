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
// Each tier is drawn with its own generator (seed + tier name), so the lot of a
// tier depends only on the seed and that tier's IDs: adding a guest never
// reshuffles the members. People who left (Out) still take part in the draw,
// so nobody else moves, but only their pinned slots (keep) are emitted.
func buildSlots(e *Event, keep map[string]bool) []*Slot {
	var named []*Person
	for tier := TierMember; tier <= TierAcquaintance; tier++ {
		rng := newRand(e.SeedOrDefault() + "/" + tier.String())
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
		label := SlotLabel(p.ID, k)
		if p.Out && !keep[label] {
			return
		}
		slots = append(slots, &Slot{Label: label, PersonID: p.ID, K: k})
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

// voucherStarts returns, per voucher, the first plan day on which it exists:
// 1 for the base vouchers, later days for the extras.
func voucherStarts(e *Event) ([]int, error) {
	from := make([]int, e.Vouchers)
	for i := range from {
		from[i] = 1
	}
	if len(e.Extra) == 0 {
		return from, nil
	}
	start, _ := ParseDate(e.Start)
	dates := Dates(start, MaxDays, e.Weekends)
	for _, x := range e.Extra {
		day := 0
		for i, d := range dates {
			if d.Format(DateLayout) >= x.Date {
				day = i + 1
				break
			}
		}
		if day == 0 {
			return nil, fmt.Errorf("extra vouchers on %s: beyond the plan horizon of %d days", x.Date, MaxDays)
		}
		for i := 0; i < x.Count; i++ {
			from = append(from, day)
		}
	}
	return from, nil
}

// phaseDays returns the number of plan days up to and including End, capped
// at MaxDays; 0 when the event has no end.
func phaseDays(e *Event) int {
	if e.End == "" {
		return 0
	}
	start, _ := ParseDate(e.Start)
	n := 0
	for _, d := range Dates(start, MaxDays, e.Weekends) {
		if d.Format(DateLayout) > e.End {
			break
		}
		n++
	}
	return n
}

// pinCellOK reports whether a pin names a cell that exists: a day within the
// horizon and a voucher that is available on that day.
func pinCellOK(pin Pin, from []int, horizon int) bool {
	return pin.Day >= 1 && pin.Day <= horizon && pin.Voucher >= 1 && pin.Voucher <= len(from) && from[pin.Voucher-1] <= pin.Day
}

// placeSlots fixes pinned slots to their cells and fills the remaining cells
// day by day with the unpinned slots in priority order, using every voucher
// from its start day on, up to the horizon (the end of the voucher phase, or
// MaxDays). Slots that do not fit keep Day 0: they go to the public sale;
// without an end they are an error. Vouchers of unpinned slots stay 0 until
// assignChains runs. Returns the grid and warnings about pins that could not
// be applied; conflicting pins are an error.
func placeSlots(slots []*Slot, pins []Pin, from []int, horizon int) (grid [][]*Slot, warnings []string, err error) {
	total := len(from)
	byLabel := make(map[string]*Slot, len(slots))
	for _, s := range slots {
		byLabel[s.Label] = s
	}
	type cell struct{ d, v int }
	taken := map[cell]string{}
	maxPin := 0
	for _, pin := range pins {
		s, ok := byLabel[pin.Label]
		switch {
		case !ok:
			warnings = append(warnings, fmt.Sprintf("pin %s: no such slot, dropped", pin.Label))
			continue
		case pin.Day < 1 || pin.Day > MaxDays || pin.Voucher < 1 || pin.Voucher > total:
			warnings = append(warnings, fmt.Sprintf("pin %s: cell day %d / voucher %d outside the plan, dropped", pin.Label, pin.Day, pin.Voucher))
			continue
		case pin.Day > horizon:
			warnings = append(warnings, fmt.Sprintf("pin %s: day %d is after the voucher phase (%d days), dropped", pin.Label, pin.Day, horizon))
			continue
		case from[pin.Voucher-1] > pin.Day:
			warnings = append(warnings, fmt.Sprintf("pin %s: voucher %d is not yet available on day %d, dropped", pin.Label, pin.Voucher, pin.Day))
			continue
		case s.Pinned:
			return nil, nil, fmt.Errorf("pin %s: slot pinned twice", pin.Label)
		}
		if other, dup := taken[cell{pin.Day, pin.Voucher}]; dup {
			return nil, nil, fmt.Errorf("pins %s and %s on the same cell (day %d, voucher %d)", other, pin.Label, pin.Day, pin.Voucher)
		}
		taken[cell{pin.Day, pin.Voucher}] = pin.Label
		s.Pinned, s.Day, s.Voucher = true, pin.Day, pin.Voucher
		if pin.Day > maxPin {
			maxPin = pin.Day
		}
	}

	row := func(d int) []*Slot {
		for len(grid) < d {
			grid = append(grid, make([]*Slot, total))
		}
		return grid[d-1]
	}
	for _, s := range slots {
		if s.Pinned {
			row(s.Day)[s.Voucher-1] = s
		}
	}
	next := 0
	skipPinned := func() {
		for next < len(slots) && slots[next].Pinned {
			next++
		}
	}
	skipPinned()
	lastFill := 0
	for d := 1; next < len(slots); d++ {
		if d > horizon {
			if horizon < MaxDays {
				break // the rest goes to the public sale
			}
			return nil, nil, fmt.Errorf("plan would need more than %d days", MaxDays)
		}
		r := row(d)
		free := 0
		for v := 1; v <= total; v++ {
			if from[v-1] <= d && r[v-1] == nil {
				free++
			}
		}
		for ; free > 0 && next < len(slots); free-- {
			slots[next].Day = d
			lastFill = d
			next++
			skipPinned()
		}
	}
	days := max(lastFill, maxPin, 1)
	row(days)
	grid = grid[:days]
	if lastFill > 0 && maxPin > lastFill {
		warnings = append(warnings, fmt.Sprintf("pins stretch the plan to %d days; the automatic placement ends on day %d", days, lastFill))
	}
	return grid, warnings, nil
}
