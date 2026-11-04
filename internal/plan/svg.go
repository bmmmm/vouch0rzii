// SPDX-License-Identifier: GPL-3.0-or-later

package plan

import (
	"fmt"
	"html"
	"strings"
)

// edgeKinds colours handovers green = the group keeps the voucher, grey =
// passed on, red = direct to own +N; one colour each that reaches 3:1 on both
// the light and the dark card of the page (TestEdgeColoursContrast).
var edgeKinds = [...]struct{ name, color string }{
	{"kept", "#2f8f5b"}, {"pass", "#8c8c8c"}, {"forbidden", "#e04a4a"},
}

func kindOf(h Handover) int {
	switch {
	case h.Forbidden:
		return 2
	case h.Kept:
		return 0
	}
	return 1
}

// SVG draws the plan top to bottom: one column per voucher, one row per day,
// a box per slot and a downward arrow per handover (also across idle days).
// Faint separators mark day rows; pinned slots get a thick border and tint.
// Text and boxes use currentColor, so the
// diagram follows the page's light or dark theme when inlined and is black on
// transparent as a file. Labels are escaped; the output is deterministic, so
// CLI and wasm agree byte for byte.
func (p *Plan) SVG() string {
	const (
		cellW, cellH = 140, 64
		boxW, boxH   = 124, 32
		padX, padY   = 100, 40
	)
	days, vouchers := len(p.Grid), len(p.From)
	w, h := padX+vouchers*cellW+8, padY+days*cellH+8
	box := func(d, v int) (x, y int) {
		return padX + (v-1)*cellW + (cellW-boxW)/2, padY + (d-1)*cellH + (cellH-boxH)/2
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" font-family="system-ui, sans-serif" font-size="12" fill="currentColor">`+"\n", w, h, w, h)
	fmt.Fprintf(&b, "<title>%s</title>\n<defs>", html.EscapeString(p.Event.Name))
	for _, k := range edgeKinds {
		fmt.Fprintf(&b, `<marker id="%s" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto"><path d="M0 0L10 5L0 10z" fill="%s"/></marker>`, k.name, k.color)
	}
	b.WriteString("</defs>\n")
	for d := 1; d <= days; d++ {
		y := padY + (d-1)*cellH
		fmt.Fprintf(&b, `<path d="M8 %dH%d" fill="none" stroke="currentColor" stroke-opacity="0.12"/>`+"\n", y, w-8)
		fmt.Fprintf(&b, `<text x="8" y="%d" font-weight="600">%s</text>`+"\n", y+cellH/2+4, html.EscapeString(p.dayLabel(d)))
	}
	for v := 1; v <= vouchers; v++ {
		fmt.Fprintf(&b, `<text x="%d" y="26" text-anchor="middle" font-size="14" font-weight="700">V%d</text>`+"\n", padX+(v-1)*cellW+cellW/2, v)
	}
	for _, hd := range p.Handovers { // arrows first, boxes cover their ends
		x1, y1 := box(hd.From.Day, hd.Voucher)
		x2, y2 := box(hd.To.Day, hd.Voucher)
		k := edgeKinds[kindOf(hd)]
		fmt.Fprintf(&b, `<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="%s" stroke-width="2.5" marker-end="url(#%s)"/>`+"\n", x1+boxW/2, y1+boxH, x2+boxW/2, y2, k.color, k.name)
	}
	for d := 1; d <= days; d++ {
		for v := 1; v <= vouchers; v++ {
			s := p.Grid[d-1][v-1]
			if s == nil {
				continue
			}
			x, y := box(d, v)
			stroke := 1
			fill := `fill="none"`
			if s.Pinned {
				stroke = 3
				fill = `fill="currentColor" fill-opacity="0.12"`
			}
			fit := ""
			if len([]rune(s.Label)) > 14 {
				fit = fmt.Sprintf(` textLength="%d" lengthAdjust="spacingAndGlyphs"`, boxW-10)
			}
			fmt.Fprintf(&b, `<rect x="%d" y="%d" width="%d" height="%d" rx="6" %s stroke="currentColor" stroke-width="%d"/>`+"\n", x, y, boxW, boxH, fill, stroke)
			fmt.Fprintf(&b, `<text x="%d" y="%d" text-anchor="middle"%s>%s</text>`+"\n", x+boxW/2, y+boxH/2+4, fit, html.EscapeString(s.Label))
		}
	}
	b.WriteString("</svg>\n")
	return b.String()
}
