// SPDX-License-Identifier: GPL-3.0-or-later

package plan

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Export is the serialisable view of a plan: what the CLI prints as JSON and
// what the page renders. Grid is [day-1][voucher-1] with "" for an empty cell.
type Export struct {
	Event       string           `json:"event"`
	Fingerprint string           `json:"fingerprint"`
	Vouchers    int              `json:"vouchers"`    // total, including extras
	VoucherFrom []int            `json:"voucherFrom"` // first day per voucher, 1-based
	VoucherLost []int            `json:"voucherLost"` // first day without the voucher, 0 = not lost
	Handover    string           `json:"handover"`
	End         DayExport        `json:"end"` // last day of the voucher phase; zero without an end
	Days        []DayExport      `json:"days"`
	Grid        [][]string       `json:"grid"`
	Handovers   []HandoverExport `json:"handovers"`
	Sheets      []Sheet          `json:"sheets"`
	Public      []string         `json:"public"` // slots that did not fit before End, in priority order
	Pins        []Pin            `json:"pins"`
	Stats       Stats            `json:"stats"`
	Draw        DrawExport       `json:"draw"`
	Warnings    []string         `json:"warnings"`
}

// DayExport names one column of the plan.
type DayExport struct {
	Day   int    `json:"day"`
	Date  string `json:"date"`
	Label string `json:"label"`
}

// HandoverExport is one evening transfer with contact addresses. Day is the
// buying day of From, Date and Label the evening the new voucher is expected.
type HandoverExport struct {
	Day       int    `json:"day"`
	Date      string `json:"date"`
	Label     string `json:"label"`
	Voucher   int    `json:"voucher"`
	From      string `json:"from"`
	FromEmail string `json:"fromEmail,omitempty"`
	To        string `json:"to"`
	ToEmail   string `json:"toEmail,omitempty"`
	Kept      bool   `json:"kept,omitempty"`
	Forbidden bool   `json:"forbidden,omitempty"`
}

// Sheet is the per-slot note: when you get the voucher, from whom, and to whom
// you pass it on. For +k slots the contact is the sponsor.
type Sheet struct {
	Label     string `json:"label"`
	Person    string `json:"person"`
	Email     string `json:"email,omitempty"`
	Day       int    `json:"day"`
	Date      string `json:"date"`
	Voucher   int    `json:"voucher"`
	From      string `json:"from"`
	FromEmail string `json:"fromEmail,omitempty"`
	To        string `json:"to"`
	ToEmail   string `json:"toEmail,omitempty"`
	Pinned    bool   `json:"pinned,omitempty"`
}

// OrgaLabel marks the first holder of a chain: the voucher comes from the organisers.
const OrgaLabel = "organisers"

func (p *Plan) email(s *Slot) string {
	if s == nil {
		return ""
	}
	if q := p.Event.Person(s.PersonID); q != nil {
		return q.Email
	}
	return ""
}

func (p *Plan) dayLabel(d int) string {
	t, err := ParseDate(p.Dates[d-1])
	if err != nil {
		return p.Dates[d-1]
	}
	return DayLabel(t)
}

// handoverDate is the evening on which the new voucher of h is expected:
// Every-1 plan days after From's buying day.
func (p *Plan) handoverDate(h Handover) time.Time {
	t, _ := ParseDate(p.Dates[h.Day-1])
	every := p.Event.EveryOrDefault()
	return planDays(t, every, p.Event.Weekends)[every-1]
}

// end describes the last day of the voucher phase; zero without an end.
func (p *Plan) end() DayExport {
	if p.PhaseDays == 0 {
		return DayExport{}
	}
	t, _ := ParseDate(p.Event.End)
	return DayExport{Day: p.PhaseDays, Date: p.Event.End, Label: DayLabel(t)}
}

// ToExport flattens the plan.
func (p *Plan) ToExport() *Export {
	ex := &Export{
		Event:       p.Event.Name,
		Fingerprint: p.Fingerprint(),
		Vouchers:    len(p.From),
		VoucherFrom: p.From,
		VoucherLost: p.Until,
		Handover:    p.Event.HandoverOrDefault(),
		End:         p.end(),
		Handovers:   []HandoverExport{},
		Sheets:      []Sheet{},
		Public:      p.Public(),
		Pins:        Freeze(p),
		Stats:       p.Stats,
		Draw:        p.Event.Draw(),
		Warnings:    append([]string{}, p.Warnings...),
	}
	for d := range p.Grid {
		ex.Days = append(ex.Days, DayExport{Day: d + 1, Date: p.Dates[d], Label: p.dayLabel(d + 1)})
		row := make([]string, len(p.From))
		for v, s := range p.Grid[d] {
			if s != nil {
				row[v] = s.Label
			}
		}
		ex.Grid = append(ex.Grid, row)
	}
	for _, h := range p.Handovers {
		at := p.handoverDate(h)
		ex.Handovers = append(ex.Handovers, HandoverExport{
			Day: h.Day, Date: at.Format(DateLayout), Label: DayLabel(at), Voucher: h.Voucher,
			From: h.From.Label, FromEmail: p.email(h.From),
			To: h.To.Label, ToEmail: p.email(h.To),
			Kept: h.Kept, Forbidden: h.Forbidden,
		})
	}
	for d := range p.Grid {
		for v, s := range p.Grid[d] {
			if s == nil {
				continue
			}
			sh := Sheet{Label: s.Label, Person: s.PersonID, Day: d + 1, Date: p.Dates[d], Voucher: v + 1, Pinned: s.Pinned, From: OrgaLabel}
			if q := p.Event.Person(s.PersonID); q != nil {
				sh.Email = q.Email
			}
			if prev := prevHolder(p.Grid, d+1, v+1); prev != nil {
				sh.From, sh.FromEmail = prev.Label, p.email(prev)
			}
			if next := nextHolder(p.Grid, d+1, v+1); next != nil {
				sh.To, sh.ToEmail = next.Label, p.email(next)
			}
			ex.Sheets = append(ex.Sheets, sh)
		}
	}
	return ex
}

// JSON renders the export with two-space indentation and a trailing newline.
func (p *Plan) JSON() ([]byte, error) {
	b, err := json.MarshalIndent(p.ToExport(), "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// Markdown renders the matrix (vouchers × days, pinned cells bold), the
// voucher phase with its free slots and the tickets left for the public
// sale, the evening handovers per day, the statistics and the warnings.
func (p *Plan) Markdown() string {
	var b strings.Builder
	s := p.Stats
	fmt.Fprintf(&b, "# %s\n\n%d vouchers, %d days, %d slots, handover at %s. Version %s\n\n", p.Event.Name, len(p.From), s.Days, s.Slots, p.Event.HandoverOrDefault(), p.Fingerprint())
	if p.PhaseDays > 0 {
		fmt.Fprintf(&b, "Voucher phase until %s (%d days), %d slots free.\n\n", p.end().Label, p.PhaseDays, s.Free)
	} else {
		fmt.Fprintf(&b, "%d slots free.\n\n", s.Free)
	}
	for v, u := range p.Until {
		if u > 0 { // the day may lie beyond the grid
			start, _ := ParseDate(p.Event.Start)
			fmt.Fprintf(&b, "Voucher %d lost from %s.\n\n", v+1, DayLabel(Dates(start, u, p.Event.Weekends, p.Event.Every)[u-1]))
		}
	}
	if pub := p.Public(); len(pub) > 0 {
		fmt.Fprintf(&b, "**Public sale:** %d ticket(s) did not fit into the voucher phase: %s\n\n", len(pub), strings.Join(pub, ", "))
	}
	b.WriteString("| Voucher |")
	for d := range p.Grid {
		fmt.Fprintf(&b, " %s |", p.dayLabel(d+1))
	}
	b.WriteString("\n|---|" + strings.Repeat("---|", len(p.Grid)) + "\n")
	for v := 1; v <= len(p.From); v++ {
		fmt.Fprintf(&b, "| %d |", v)
		for d := range p.Grid {
			cell := ""
			if slot := p.Grid[d][v-1]; slot != nil {
				cell = slot.Label
				if slot.Pinned {
					cell = "**" + cell + "**"
				}
			}
			fmt.Fprintf(&b, " %s |", cell)
		}
		b.WriteByte('\n')
	}
	b.WriteString("\n## Handovers\n")
	day := 0
	for _, h := range p.Handovers {
		if h.Day != day {
			day = h.Day
			fmt.Fprintf(&b, "\n**%s, %s**\n\n", DayLabel(p.handoverDate(h)), p.Event.HandoverOrDefault())
		}
		fmt.Fprintf(&b, "- Voucher %d: %s → %s", h.Voucher, h.From.Label, h.To.Label)
		if e := p.email(h.To); e != "" {
			fmt.Fprintf(&b, " (%s)", e)
		}
		if h.To.Day != h.Day+1 {
			fmt.Fprintf(&b, ", buys on %s", p.dayLabel(h.To.Day))
		}
		if h.Kept {
			b.WriteString(" ✓")
		}
		if h.Forbidden {
			b.WriteString(" ⚠ own +N directly")
		}
		b.WriteByte('\n')
	}
	fmt.Fprintf(&b, "\n%d handovers, %d of them stay within a group (✓).\n", s.Handovers, s.Kept)
	for _, w := range p.Warnings {
		fmt.Fprintf(&b, "\n> ⚠ %s\n", w)
	}
	return b.String()
}

// CSV renders the matrix semicolon-separated, one row per voucher, for
// spreadsheets.
func (p *Plan) CSV() string {
	var b strings.Builder
	b.WriteString("Voucher")
	for d := range p.Grid {
		fmt.Fprintf(&b, ";%s %s", p.dayLabel(d+1), p.Dates[d])
	}
	b.WriteByte('\n')
	for v := 1; v <= len(p.From); v++ {
		fmt.Fprint(&b, v)
		for d := range p.Grid {
			b.WriteByte(';')
			if s := p.Grid[d][v-1]; s != nil {
				b.WriteString(s.Label)
			}
		}
		b.WriteByte('\n')
	}
	return b.String()
}
