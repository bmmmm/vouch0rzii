// SPDX-License-Identifier: GPL-3.0-or-later

package plan

import (
	"strings"
	"testing"
)

// a knows c<x and a+1 is pinned to day 2 on voucher 1, so the planner puts
// b on voucher 1 (b → a+1, strangers) and a on voucher 2 (a → c<x, known)
// instead of the forbidden a → a+1.
func TestSVGDrawsBoxesArrowsAndEscapes(t *testing.T) {
	e := renderEvent()
	e.People[2].ID = "c<x" // allowed in a handle, must be escaped
	e.People[0].Knows = []string{"c<x"}
	p := mustCompute(t, e, []Pin{{Label: "a+1", Day: 2, Voucher: 1}})
	svg := p.SVG()
	if !strings.HasPrefix(svg, `<svg xmlns="http://www.w3.org/2000/svg"`) || !strings.HasSuffix(svg, "</svg>\n") {
		t.Fatalf("not a complete svg:\n%s", svg)
	}
	for _, want := range []string{
		">c&lt;x<", ">a+1<", "Fr 30.10.", "Mo 02.11.", ">V2<",
		`stroke-width="3"`,
		`marker-end="url(#known)"`,
		`marker-end="url(#stranger)"`,
	} {
		if !strings.Contains(svg, want) {
			t.Errorf("missing %q in\n%s", want, svg)
		}
	}
	if strings.Contains(svg, "c<x") || strings.Contains(svg, "url(#forbidden)") {
		t.Errorf("unescaped label or forbidden arrow:\n%s", svg)
	}
	if n := strings.Count(svg, "<rect "); n != 4 {
		t.Errorf("want 4 boxes, got %d", n)
	}
	if n := strings.Count(svg, "<line "); n != 2 || p.Stats.Handovers != 2 {
		t.Errorf("want 2 arrows for 2 handovers, got %d / %d", n, p.Stats.Handovers)
	}
}

func TestSVGEscapesTitle(t *testing.T) {
	e := renderEvent()
	e.Name = "t<1"
	if svg := mustCompute(t, e, nil).SVG(); !strings.Contains(svg, "<title>t&lt;1</title>") {
		t.Fatalf("title not escaped:\n%.200s", svg)
	}
}
