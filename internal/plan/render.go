// SPDX-License-Identifier: GPL-3.0-or-later

package plan

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"
)

// Export is the serialisable view of a plan: what the CLI prints as JSON and
// what the page renders. Grid is [day-1][voucher-1] with "" for an empty cell.
type Export struct {
	Event     string           `json:"event"`
	Vouchers  int              `json:"vouchers"`
	Handover  string           `json:"handover"`
	Days      []DayExport      `json:"days"`
	Grid      [][]string       `json:"grid"`
	Handovers []HandoverExport `json:"handovers"`
	Sheets    []Sheet          `json:"sheets"`
	Pins      []Pin            `json:"pins"`
	Stats     Stats            `json:"stats"`
	Warnings  []string         `json:"warnings"`
}

// DayExport names one column of the plan.
type DayExport struct {
	Day   int    `json:"day"`
	Date  string `json:"date"`
	Label string `json:"label"`
}

// HandoverExport is one evening transfer with contact addresses.
type HandoverExport struct {
	Day       int    `json:"day"`
	Date      string `json:"date"`
	Voucher   int    `json:"voucher"`
	From      string `json:"from"`
	FromEmail string `json:"fromEmail,omitempty"`
	To        string `json:"to"`
	ToEmail   string `json:"toEmail,omitempty"`
	Known     bool   `json:"known"`
	Forbidden bool   `json:"forbidden,omitempty"`
}

// Sheet is the per-slot note: when you get the voucher, from whom, and to whom
// you pass it on. For +k slots the contact is the sponsor.
type Sheet struct {
	Label     string `json:"label"`
	Person    string `json:"person"`
	Email     string `json:"email,omitempty"`
	Tier      string `json:"tier"`
	Card      bool   `json:"card,omitempty"`
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
const OrgaLabel = "Orga"

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

// ToExport flattens the plan.
func (p *Plan) ToExport() *Export {
	ex := &Export{
		Event:    p.Event.Name,
		Vouchers: p.Event.Vouchers,
		Handover: p.Event.HandoverOrDefault(),
		Pins:     Freeze(p),
		Stats:    p.Stats,
		Warnings: append([]string{}, p.Warnings...),
	}
	for d := range p.Grid {
		ex.Days = append(ex.Days, DayExport{Day: d + 1, Date: p.Dates[d], Label: p.dayLabel(d + 1)})
		row := make([]string, p.Event.Vouchers)
		for v, s := range p.Grid[d] {
			if s != nil {
				row[v] = s.Label
			}
		}
		ex.Grid = append(ex.Grid, row)
	}
	for _, h := range p.Handovers {
		ex.Handovers = append(ex.Handovers, HandoverExport{
			Day: h.Day, Date: p.Dates[h.Day-1], Voucher: h.Voucher,
			From: h.From.Label, FromEmail: p.email(h.From),
			To: h.To.Label, ToEmail: p.email(h.To),
			Known: h.Known, Forbidden: h.Forbidden,
		})
	}
	for d := range p.Grid {
		for v, s := range p.Grid[d] {
			if s == nil {
				continue
			}
			q := p.Event.Person(s.PersonID)
			sh := Sheet{Label: s.Label, Person: s.PersonID, Day: d + 1, Date: p.Dates[d], Voucher: v + 1, Pinned: s.Pinned, From: OrgaLabel}
			if q != nil {
				sh.Email, sh.Tier, sh.Card = q.Email, q.Tier.String(), q.Card
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
	if ex.Handovers == nil {
		ex.Handovers = []HandoverExport{}
	}
	if ex.Warnings == nil {
		ex.Warnings = []string{}
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

func pad(s string, w int) string {
	if n := utf8.RuneCountInString(s); n < w {
		return s + strings.Repeat(" ", w-n)
	}
	return s
}

// Text renders the matrix (vouchers × days), the evening handovers per day,
// the statistics and the warnings as plain text for terminals.
func (p *Plan) Text() string {
	ex := p.ToExport()
	var b strings.Builder
	fmt.Fprintf(&b, "%s — %d vouchers, %d days, %d slots, handover %s\n\n", ex.Event, ex.Vouchers, ex.Stats.Days, ex.Stats.Slots, ex.Handover)
	w := 0
	for _, row := range ex.Grid {
		for _, c := range row {
			if n := utf8.RuneCountInString(c); n > w {
				w = n
			}
		}
	}
	for _, d := range ex.Days {
		if n := utf8.RuneCountInString(d.Label); n > w {
			w = n
		}
	}
	b.WriteString(pad("Voucher", 8))
	for _, d := range ex.Days {
		b.WriteString("  " + pad(d.Label, w))
	}
	b.WriteByte('\n')
	for v := 1; v <= ex.Vouchers; v++ {
		b.WriteString(pad(fmt.Sprint(v), 8))
		for d := range ex.Days {
			cell := ex.Grid[d][v-1]
			if cell == "" {
				cell = "·"
			}
			if s := p.Grid[d][v-1]; s != nil && s.Pinned {
				cell += "*"
			}
			b.WriteString("  " + pad(cell, w))
		}
		b.WriteByte('\n')
	}
	b.WriteString("\nHandovers (* = pinned, ✓ = know each other, ! = direct to own +N):\n")
	day := 0
	for _, h := range ex.Handovers {
		if h.Day != day {
			day = h.Day
			fmt.Fprintf(&b, "%s %s\n", ex.Days[day-1].Label, ex.Handover)
		}
		mark := " "
		if h.Known {
			mark = "✓"
		}
		if h.Forbidden {
			mark = "!"
		}
		fmt.Fprintf(&b, "  V%d %s %s → %s", h.Voucher, mark, h.From, h.To)
		if h.ToEmail != "" {
			fmt.Fprintf(&b, " <%s>", h.ToEmail)
		}
		b.WriteByte('\n')
	}
	fmt.Fprintf(&b, "\nHandovers: %d, between people who know each other: %d, strangers: %d", ex.Stats.Handovers, ex.Stats.Known, ex.Stats.Unknown)
	if ex.Stats.Forbidden > 0 {
		fmt.Fprintf(&b, ", direct to own +N: %d", ex.Stats.Forbidden)
	}
	if ex.Stats.Pinned > 0 {
		fmt.Fprintf(&b, ", pinned: %d", ex.Stats.Pinned)
	}
	b.WriteByte('\n')
	for _, wmsg := range ex.Warnings {
		fmt.Fprintf(&b, "warning: %s\n", wmsg)
	}
	return b.String()
}

// Markdown renders the matrix and the handovers per day as Markdown.
func (p *Plan) Markdown() string {
	ex := p.ToExport()
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n%d vouchers, %d days, %d slots, handover at %s.\n\n", ex.Event, ex.Vouchers, ex.Stats.Days, ex.Stats.Slots, ex.Handover)
	b.WriteString("| Voucher |")
	for _, d := range ex.Days {
		fmt.Fprintf(&b, " %s |", d.Label)
	}
	b.WriteString("\n|---|")
	for range ex.Days {
		b.WriteString("---|")
	}
	b.WriteByte('\n')
	for v := 1; v <= ex.Vouchers; v++ {
		fmt.Fprintf(&b, "| %d |", v)
		for d := range ex.Days {
			cell := ex.Grid[d][v-1]
			if s := p.Grid[d][v-1]; s != nil && s.Pinned {
				cell = "**" + cell + "**"
			}
			fmt.Fprintf(&b, " %s |", cell)
		}
		b.WriteByte('\n')
	}
	b.WriteString("\n## Handovers\n")
	day := 0
	for _, h := range ex.Handovers {
		if h.Day != day {
			day = h.Day
			fmt.Fprintf(&b, "\n**%s, %s**\n\n", ex.Days[day-1].Label, ex.Handover)
		}
		fmt.Fprintf(&b, "- Voucher %d: %s → %s", h.Voucher, h.From, h.To)
		if h.ToEmail != "" {
			fmt.Fprintf(&b, " (%s)", h.ToEmail)
		}
		if h.Known {
			b.WriteString(" ✓")
		}
		if h.Forbidden {
			b.WriteString(" ⚠ direct to own +N")
		}
		b.WriteByte('\n')
	}
	fmt.Fprintf(&b, "\n%d handovers, %d between people who know each other, %d between strangers.\n", ex.Stats.Handovers, ex.Stats.Known, ex.Stats.Unknown)
	for _, w := range ex.Warnings {
		fmt.Fprintf(&b, "\n> ⚠ %s\n", w)
	}
	return b.String()
}

// CSV renders the matrix semicolon-separated, one row per voucher, for
// spreadsheets.
func (p *Plan) CSV() string {
	ex := p.ToExport()
	var b strings.Builder
	b.WriteString("Voucher")
	for _, d := range ex.Days {
		fmt.Fprintf(&b, ";%s %s", d.Label, d.Date)
	}
	b.WriteByte('\n')
	for v := 1; v <= ex.Vouchers; v++ {
		fmt.Fprint(&b, v)
		for d := range ex.Days {
			b.WriteString(";" + ex.Grid[d][v-1])
		}
		b.WriteByte('\n')
	}
	return b.String()
}
