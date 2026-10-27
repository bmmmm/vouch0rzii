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
	starts, err := voucherStarts(e)
	if err != nil {
		return nil, err
	}
	phase, horizon := phaseDays(e), MaxDays
	if e.End != "" {
		if phase == 0 {
			return nil, fmt.Errorf("end %s: no plan day between start and end", e.End)
		}
		horizon = phase
	}
	// Slots of people who left survive only under a pin that will actually
	// be applied, so a pin on a vanished cell does not resurrect them.
	keep := make(map[string]bool, len(pins))
	for _, pin := range pins {
		if pinCellOK(pin, starts, horizon) {
			keep[pin.Label] = true
		}
	}
	slots := buildSlots(e, keep)
	grid, pinWarnings, err := placeSlots(slots, pins, starts, horizon)
	if err != nil {
		return nil, err
	}
	warnings = append(warnings, pinWarnings...)
	for v, f := range starts {
		if phase > 0 && f > phase {
			warnings = append(warnings, fmt.Sprintf("voucher %d: available from day %d, after the voucher phase (%d days), unused", v+1, f, phase))
		}
	}
	assignChains(e, slots, grid, starts)

	p := &Plan{Event: e, PhaseDays: phase, From: starts, Slots: slots, Grid: grid, Warnings: warnings}
	for _, t := range Dates(start, len(grid), e.Weekends) {
		p.Dates = append(p.Dates, t.Format(DateLayout))
	}
	// Every holder except the first of a chain receives the voucher from the
	// previous holder, even across days on which the voucher idles.
	for d := 1; d <= len(grid); d++ {
		for v := 1; v <= len(starts); v++ {
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
	// Free voucher days: capacity of the phase (or of the grid, without an
	// end) minus the placed slots. Vouchers that arrive after the end add none.
	span, placed := phase, 0
	if span == 0 {
		span = len(grid)
	}
	for _, f := range starts {
		if f <= span {
			p.Stats.Free += span - f + 1
		}
	}
	for _, s := range slots {
		if s.Pinned {
			p.Stats.Pinned++
		}
		if s.Day > 0 {
			placed++
		}
	}
	p.Stats.Free -= placed
	return p, nil
}

// Public lists the slots that did not fit before the end of the voucher
// phase, in priority order: those tickets are bought in the public sale.
func (p *Plan) Public() []string {
	out := []string{}
	for _, s := range p.Slots {
		if s.Day == 0 {
			out = append(out, s.Label)
		}
	}
	return out
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
