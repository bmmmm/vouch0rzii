// SPDX-License-Identifier: GPL-3.0-or-later

package plan

import (
	"fmt"
	"sort"
)

// Compute normalises the event and builds the plan. Pins fix slots to cells
// before the automatic placement fills the rest. The returned plan carries all
// warnings (unknown handles, dropped pins, forbidden handovers that could not
// be avoided); hard problems come back as the error.
func Compute(e *Event, pins []Pin) (*Plan, error) {
	warnings, err := Normalize(e)
	if err != nil {
		return nil, err
	}
	start, _ := ParseDate(e.Start)
	slots := buildSlots(e)
	grid, pinWarnings, err := placeSlots(e, slots, pins)
	if err != nil {
		return nil, err
	}
	warnings = append(warnings, pinWarnings...)
	assignChains(e, slots, grid)

	p := &Plan{Event: e, Slots: slots, Grid: grid, Warnings: warnings}
	for _, t := range Dates(start, len(grid), e.Weekends) {
		p.Dates = append(p.Dates, t.Format(DateLayout))
	}
	// Every holder except the first of a chain receives the voucher from the
	// previous holder, even across days on which the voucher idles.
	for d := 1; d <= len(grid); d++ {
		for v := 1; v <= e.Vouchers; v++ {
			to := grid[d-1][v-1]
			if to == nil {
				continue
			}
			from := prevHolder(grid, d, v)
			if from == nil {
				continue
			}
			c := edgeCost(e, from, to)
			p.Handovers = append(p.Handovers, Handover{Day: from.Day, Voucher: v, From: from, To: to, Known: c == costKnown, Forbidden: c == costForbidden})
		}
	}
	sort.SliceStable(p.Handovers, func(i, j int) bool {
		a, b := p.Handovers[i], p.Handovers[j]
		if a.Day != b.Day {
			return a.Day < b.Day
		}
		return a.Voucher < b.Voucher
	})
	p.Stats = Stats{Days: len(grid), Slots: len(slots), Handovers: len(p.Handovers)}
	for _, h := range p.Handovers {
		switch {
		case h.Forbidden:
			p.Stats.Forbidden++
			p.Warnings = append(p.Warnings, fmt.Sprintf("day %d, voucher %d: %s hands directly to %s (no alternative)", h.Day, h.Voucher, h.From.Label, h.To.Label))
		case h.Known:
			p.Stats.Known++
		default:
			p.Stats.Unknown++
		}
	}
	for _, s := range slots {
		if s.Pinned {
			p.Stats.Pinned++
		}
	}
	return p, nil
}

// Freeze returns a pin for every placed slot, so that the same plan renders
// again regardless of seed or algorithm version. Used for viewer links.
func Freeze(p *Plan) []Pin {
	pins := make([]Pin, 0, len(p.Slots))
	for _, s := range p.Slots {
		if s.Day > 0 && s.Voucher > 0 {
			pins = append(pins, Pin{Label: s.Label, Day: s.Day, Voucher: s.Voucher})
		}
	}
	return pins
}

// Slot returns the slot with the given label, or nil.
func (p *Plan) Slot(label string) *Slot {
	for _, s := range p.Slots {
		if s.Label == label {
			return s
		}
	}
	return nil
}
