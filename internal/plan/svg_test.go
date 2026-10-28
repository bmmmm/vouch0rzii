// SPDX-License-Identifier: GPL-3.0-or-later

package plan

import (
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// TestEdgeColoursContrast keeps every handover line at 3:1 (WCAG non-text)
// against both card colours of web/style.css, light and dark.
func TestEdgeColoursContrast(t *testing.T) {
	css, err := os.ReadFile("../../web/style.css")
	if err != nil {
		t.Fatal(err)
	}
	cards := regexp.MustCompile(`--card: (#[0-9a-f]{6})`).FindAllStringSubmatch(string(css), -1)
	if len(cards) != 2 {
		t.Fatalf("want a light and a dark --card in web/style.css, got %v", cards)
	}
	lum := func(hex string) float64 {
		var c [3]float64
		for i := range c {
			v, _ := strconv.ParseUint(hex[1+2*i:3+2*i], 16, 8)
			x := float64(v) / 255
			if x <= 0.03928 {
				c[i] = x / 12.92
			} else {
				c[i] = math.Pow((x+0.055)/1.055, 2.4)
			}
		}
		return 0.2126*c[0] + 0.7152*c[1] + 0.0722*c[2]
	}
	for _, k := range edgeKinds {
		for _, m := range cards {
			card := m[1]
			a, b := lum(k.color), lum(card)
			if r := (math.Max(a, b) + 0.05) / (math.Min(a, b) + 0.05); r < 3 {
				t.Errorf("%s %s on %s: %.2f:1, want 3:1", k.name, k.color, card, r)
			}
		}
	}
}

// a is a group and a+1 is pinned to day 2 on voucher 1, so a keeps voucher 1
// (a → a+1, kept) and b passes voucher 2 on (b → c<x).
func TestSVGDrawsBoxesArrowsAndEscapes(t *testing.T) {
	e := renderEvent()
	e.People[2].ID = "c<x" // allowed in a handle, must be escaped
	e.People[0].Group = true
	p := mustCompute(t, e, []Pin{{Label: "a+1", Day: 2, Voucher: 1}})
	svg := p.SVG()
	if !strings.HasPrefix(svg, `<svg xmlns="http://www.w3.org/2000/svg"`) || !strings.HasSuffix(svg, "</svg>\n") {
		t.Fatalf("not a complete svg:\n%s", svg)
	}
	for _, want := range []string{
		">c&lt;x<", ">a+1<", "Fri 30/10", "Mon 02/11", ">V2<",
		`stroke-width="3"`,
		`marker-end="url(#kept)"`,
		`marker-end="url(#pass)"`,
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
