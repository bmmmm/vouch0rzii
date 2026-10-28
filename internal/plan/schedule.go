// SPDX-License-Identifier: GPL-3.0-or-later

package plan

import (
	"fmt"
	"hash/fnv"
	"math/big"
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

// DrawExport says how much chance is in the draw: per tier, how many people
// are drawn by lot and how many orders that allows, and all tiers together.
type DrawExport struct {
	Seed    string     `json:"seed"`
	Tiers   []DrawTier `json:"tiers"`
	Lineups string     `json:"lineups"` // product of the tiers' orders, decimal
}

// DrawTier is one tier of the draw: Drawn people shuffled (Orders = Drawn!,
// decimal), Fixed people placed by their manual Order.
type DrawTier struct {
	Tier   string `json:"tier"`
	Drawn  int    `json:"drawn"`
	Fixed  int    `json:"fixed"`
	Orders string `json:"orders"`
}

// Draw counts the draw buildSlots performs (tierSplit): people who left are
// drawn too, so Lineups counts possible results of the lot.
func (e *Event) Draw() DrawExport {
	d := DrawExport{Seed: e.SeedOrDefault(), Tiers: []DrawTier{}}
	lineups := big.NewInt(1)
	for tier := TierMember; tier <= TierAcquaintance; tier++ {
		fixed, drawn := e.tierSplit(tier)
		orders := new(big.Int).MulRange(1, int64(len(drawn))) // 0! = 1
		lineups.Mul(lineups, orders)
		d.Tiers = append(d.Tiers, DrawTier{Tier: tier.String(), Drawn: len(drawn), Fixed: len(fixed), Orders: orders.String()})
	}
	d.Lineups = lineups.String()
	return d
}

// tierSplit returns a tier's people with a manual Order and those left to
// the lot, both in event order.
func (e *Event) tierSplit(tier Tier) (fixed, drawn []*Person) {
	for i := range e.People {
		switch p := &e.People[i]; {
		case p.Tier != tier:
		case p.Order > 0:
			fixed = append(fixed, p)
		default:
			drawn = append(drawn, p)
		}
	}
	return fixed, drawn
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
		fixed, drawn := e.tierSplit(tier)
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
	dates := Dates(start, MaxDays, e.Weekends, e.Every)
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
	for _, d := range Dates(start, MaxDays, e.Weekends, e.Every) {
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

// chainGroups lets every group keep its voucher. It runs after placeSlots and
// keeps its choice of who is in the plan, so nobody goes to the public sale
// for a group; it only moves days. Day by day, the unpinned slots fill the
// free cells in priority order, as placeSlots does. A group (person with
// the group flag) starts its chain on the day its first slot comes up or
// its first pin sits, whichever is earlier: at the end of that day, groups
// in priority order, its other slots take target days from that day on,
// one per day while one chain suffices, otherwise the first chain from
// that day and further chains ending on the last day. A pinned slot takes
// the target of its day, a day without a free cell is skipped, and a slot
// left without a target waits for its turn like everybody else's: a group
// chains only once, so pinning the past keeps the future. The last day is
// the plan's, so the group runs on as few vouchers as the plan's length
// allows; with OneChain it is the horizon, so one chain may make the plan
// longer.
// assignChains then keeps each chain on its voucher (costKeep) unless a
// forbidden handover would outweigh it. Returns the grid, grown to the last
// placed day.
func chainGroups(e *Event, slots []*Slot, grid [][]*Slot, from []int, horizon int) [][]*Slot {
	days, last := len(grid), len(grid)
	if e.OneChain {
		last = horizon
	}
	free := make([]int, last+1) // cells per day not taken by a pin
	for d := 1; d <= last; d++ {
		for v, f := range from {
			if f <= d && (d > days || grid[d-1][v] == nil) {
				free[d]++
			}
		}
	}
	var groups []string // in priority order
	var queue []*Slot   // unpinned placed slots
	member, pinDay := map[string][]*Slot{}, map[string]int{}
	for _, s := range slots {
		if s.Day == 0 {
			continue
		}
		if p := e.Person(s.PersonID); p != nil && p.Group {
			if member[s.PersonID] == nil {
				groups = append(groups, s.PersonID)
			}
			member[s.PersonID] = append(member[s.PersonID], s)
			if s.Pinned && (pinDay[s.PersonID] == 0 || s.Day < pinDay[s.PersonID]) {
				pinDay[s.PersonID] = s.Day
			}
		}
		if !s.Pinned {
			queue = append(queue, s)
		}
	}
	placed, started := map[*Slot]bool{}, map[string]bool{}
	chain := func(id string, start int) {
		started[id] = true
		gs := member[id]
		n, span := len(gs), last-start+1
		want := make([]int, last+1)
		for d := start; d <= last; d++ {
			want[d] = 1
			if n > span {
				want[d] = n / span
				if d > last-n%span {
					want[d]++
				}
			}
		}
		for _, s := range gs {
			if s.Pinned && want[s.Day] > 0 {
				want[s.Day]--
			}
		}
		d := start
		for _, s := range gs {
			if s.Pinned || placed[s] {
				continue
			}
			for d <= last && (want[d] == 0 || free[d] == 0) {
				d++
			}
			if d > last {
				return
			}
			s.Day, placed[s] = d, true
			want[d]--
			free[d]--
		}
	}
	next := 0
	for d := 1; d <= last; d++ {
		due := map[string]bool{}
		for ; free[d] > 0 && next < len(queue); next++ {
			s := queue[next]
			if placed[s] {
				continue
			}
			s.Day, placed[s] = d, true
			free[d]--
			due[s.PersonID] = true
		}
		for _, id := range groups { // in priority order, by pin or by turn alike
			if (due[id] || pinDay[id] == d) && !started[id] {
				chain(id, d)
			}
		}
	}
	for _, s := range slots {
		for s.Day > len(grid) {
			grid = append(grid, make([]*Slot, len(from)))
		}
	}
	return grid
}
