// SPDX-License-Identifier: GPL-3.0-or-later

package plan

import (
	"fmt"
	"html"
	"strings"
)

// edgeKinds colours handovers green = know each other, grey = strangers,
// red = direct to own +N; one colour each that reaches 3:1 on both the light
// and the dark card of the page (TestEdgeColoursContrast).
var edgeKinds = [...]struct{ name, color string }{
	{"known", "#2f8f5b"}, {"stranger", "#8c8c8c"}, {"forbidden", "#e04a4a"},
}

func kindOf(h Handover) int {
	switch {
	case h.Forbidden:
		return 2
	case h.Known:
		return 0
	}
	return 1
}

// SVG draws the plan as a flow diagram: one lane per voucher, one column per
// day, a box per slot and an arrow per handover (also across idle days).
// Pinned slots get a thick border. Text and boxes use currentColor, so the
// diagram follows the page's light or dark theme when inlined and is black on
// transparent as a file. Labels are escaped; the output is deterministic, so
// CLI and wasm agree byte for byte.
func (p *Plan) SVG() string {
	const (
		cellW, cellH = 124, 46
		boxW, boxH   = 108, 30
		padX, padY   = 48, 34
	)
	days, vouchers := len(p.Grid), len(p.From)
	w, h := padX+days*cellW+8, padY+vouchers*cellH+8
	box := func(d, v int) (x, y int) {
		return padX + (d-1)*cellW + (cellW-boxW)/2, padY + (v-1)*cellH + (cellH-boxH)/2
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d" font-family="system-ui, sans-serif" font-size="12" fill="currentColor">`+"\n", w, h, w, h)
	fmt.Fprintf(&b, "<title>%s</title>\n<defs>", html.EscapeString(p.Event.Name))
	for _, k := range edgeKinds {
		fmt.Fprintf(&b, `<marker id="%s" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto"><path d="M0 0L10 5L0 10z" fill="%s"/></marker>`, k.name, k.color)
	}
	b.WriteString("</defs>\n")
	for d := 1; d <= days; d++ {
		fmt.Fprintf(&b, `<text x="%d" y="22" text-anchor="middle" font-weight="600">%s</text>`+"\n", padX+(d-1)*cellW+cellW/2, html.EscapeString(p.dayLabel(d)))
	}
	for v := 1; v <= vouchers; v++ {
		fmt.Fprintf(&b, `<text x="8" y="%d" opacity="0.7">V%d</text>`+"\n", padY+(v-1)*cellH+cellH/2+4, v)
	}
	for _, hd := range p.Handovers { // arrows first, boxes cover their ends
		x1, y1 := box(hd.From.Day, hd.Voucher)
		x2, y2 := box(hd.To.Day, hd.Voucher)
		k := edgeKinds[kindOf(hd)]
		fmt.Fprintf(&b, `<line x1="%d" y1="%d" x2="%d" y2="%d" stroke="%s" stroke-width="2" marker-end="url(#%s)"/>`+"\n", x1+boxW, y1+boxH/2, x2, y2+boxH/2, k.color, k.name)
	}
	for d := 1; d <= days; d++ {
		for v := 1; v <= vouchers; v++ {
			s := p.Grid[d-1][v-1]
			if s == nil {
				continue
			}
			x, y := box(d, v)
			stroke := 1
			if s.Pinned {
				stroke = 3
			}
			fit := ""
			if len([]rune(s.Label)) > 14 {
				fit = fmt.Sprintf(` textLength="%d" lengthAdjust="spacingAndGlyphs"`, boxW-10)
			}
			fmt.Fprintf(&b, `<rect x="%d" y="%d" width="%d" height="%d" rx="6" fill="none" stroke="currentColor" stroke-width="%d"/>`+"\n", x, y, boxW, boxH, stroke)
			fmt.Fprintf(&b, `<text x="%d" y="%d" text-anchor="middle"%s>%s</text>`+"\n", x+boxW/2, y+boxH/2+4, fit, html.EscapeString(s.Label))
		}
	}
	b.WriteString("</svg>\n")
	return b.String()
}
