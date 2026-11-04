// SPDX-License-Identifier: GPL-3.0-or-later

package plan

import (
	"encoding/xml"
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

// TestLegendMatchesEdgeKinds keeps the page's diagram legend in step with the
// arrows: every kind has a swatch in its colour in web/style.css and is named
// in both languages in web/index.html.
func TestLegendMatchesEdgeKinds(t *testing.T) {
	css, err := os.ReadFile("../../web/style.css")
	if err != nil {
		t.Fatal(err)
	}
	page, err := os.ReadFile("../../web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	for _, k := range edgeKinds {
		m := regexp.MustCompile(`\.sw\.arrow\.` + k.name + ` \{ background: (#[0-9a-f]{6}); \}`).FindSubmatch(css)
		if m == nil || string(m[1]) != k.color {
			t.Errorf("web/style.css: .sw.arrow.%s should be %s, got %q", k.name, k.color, m)
		}
		if n := strings.Count(string(page), `<span class="sw arrow `+k.name+`"></span>`); n != 2 {
			t.Errorf("web/index.html: want the %s arrow in the German and the English legend, got %d", k.name, n)
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

func TestSVGTopDownOrientation(t *testing.T) {
	p := mustCompute(t, renderEvent(), nil)
	var diagram struct {
		Texts []struct {
			X      int    `xml:"x,attr"`
			Y      int    `xml:"y,attr"`
			Weight string `xml:"font-weight,attr"`
			Label  string `xml:",chardata"`
		} `xml:"text"`
		Lines []struct {
			X1 int `xml:"x1,attr"`
			Y1 int `xml:"y1,attr"`
			X2 int `xml:"x2,attr"`
			Y2 int `xml:"y2,attr"`
		} `xml:"line"`
	}
	if err := xml.Unmarshal([]byte(p.SVG()), &diagram); err != nil {
		t.Fatal(err)
	}
	// Day labels (font-weight 600) give each row's y, voucher headers (700)
	// each column's x; slot labels carry no weight and sit where both meet.
	var dayY, voucherX []int
	dayX, voucherY := -1, -1
	slotAt := map[string][2]int{}
	for _, label := range diagram.Texts {
		switch label.Weight {
		case "600":
			if dayX >= 0 && (label.X != dayX || label.Y <= dayY[len(dayY)-1]) {
				t.Errorf("day labels must share x and increase y: %q at (%d,%d)", label.Label, label.X, label.Y)
			}
			dayX, dayY = label.X, append(dayY, label.Y)
			if d := len(dayY); d > len(p.Grid) || label.Label != p.dayLabel(d) {
				t.Errorf("day label %d = %q, want one per plan day in order", d, label.Label)
			}
		case "700":
			if voucherY >= 0 && (label.Y != voucherY || label.X <= voucherX[len(voucherX)-1]) {
				t.Errorf("voucher headers must share y and increase x: %q at (%d,%d)", label.Label, label.X, label.Y)
			}
			voucherY, voucherX = label.Y, append(voucherX, label.X)
			if want := "V" + strconv.Itoa(len(voucherX)); label.Label != want {
				t.Errorf("voucher header = %q, want %q", label.Label, want)
			}
		case "":
			slotAt[label.Label] = [2]int{label.X, label.Y}
		}
	}
	if len(dayY) != len(p.Grid) || len(dayY) < 2 || len(voucherX) != len(p.From) || len(voucherX) < 2 {
		t.Fatalf("want all days and vouchers, at least two each; got %d days and %d vouchers", len(dayY), len(voucherX))
	}
	slots := 0
	for d, row := range p.Grid {
		for v, s := range row {
			if s == nil {
				continue
			}
			slots++
			if at, want := slotAt[s.Label], [2]int{voucherX[v], dayY[d]}; at != want {
				t.Errorf("slot %s (day %d, voucher %d) drawn at %v, want %v: in its voucher's column and its day's row", s.Label, d+1, v+1, at, want)
			}
		}
	}
	if slots == 0 || len(slotAt) != slots {
		t.Errorf("want one label per slot, got %d labels for %d slots", len(slotAt), slots)
	}
	if len(diagram.Lines) != len(p.Handovers) || len(diagram.Lines) == 0 {
		t.Fatalf("want a nonempty line per handover, got %d lines for %d handovers", len(diagram.Lines), len(p.Handovers))
	}
	for i, h := range p.Handovers { // arrows are drawn in handover order
		line, x := diagram.Lines[i], voucherX[h.Voucher-1]
		if line.X1 != x || line.X2 != x || !(dayY[h.From.Day-1] < line.Y1 && line.Y1 < line.Y2 && line.Y2 < dayY[h.To.Day-1]) {
			t.Errorf("handover %s → %s must run straight down voucher %d from below the giver to above the receiver: (%d,%d) to (%d,%d)", h.From.Label, h.To.Label, h.Voucher, line.X1, line.Y1, line.X2, line.Y2)
		}
	}
}
