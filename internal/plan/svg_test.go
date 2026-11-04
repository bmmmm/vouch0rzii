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
	var dayCount, voucherCount, dayX, dayY, voucherX, voucherY int
	for _, label := range diagram.Texts {
		switch {
		case label.Weight == "600":
			dayCount++
			if dayCount > len(p.Grid) {
				t.Fatalf("extra day label with font-weight 600: %q", label.Label)
			}
			if label.Label != p.dayLabel(dayCount) {
				t.Errorf("day label %d = %q, want %q", dayCount, label.Label, p.dayLabel(dayCount))
			}
			if dayCount > 1 && (label.X != dayX || label.Y <= dayY) {
				t.Errorf("day labels must share x and increase y: (%d,%d) then (%d,%d)", dayX, dayY, label.X, label.Y)
			}
			dayX, dayY = label.X, label.Y
		case strings.HasPrefix(label.Label, "V"):
			voucherCount++
			if want := "V" + strconv.Itoa(voucherCount); label.Label != want {
				t.Errorf("voucher header = %q, want %q", label.Label, want)
			}
			if voucherCount > 1 && (label.Y != voucherY || label.X <= voucherX) {
				t.Errorf("voucher headers must share y and increase x: (%d,%d) then (%d,%d)", voucherX, voucherY, label.X, label.Y)
			}
			voucherX, voucherY = label.X, label.Y
		}
	}
	if dayCount != len(p.Grid) || dayCount < 2 || voucherCount != len(p.From) || voucherCount < 2 {
		t.Fatalf("want all days and vouchers, at least two each; got %d days and %d vouchers", dayCount, voucherCount)
	}
	if len(diagram.Lines) != len(p.Handovers) || len(diagram.Lines) == 0 {
		t.Fatalf("want a nonempty line per handover, got %d lines for %d handovers", len(diagram.Lines), len(p.Handovers))
	}
	for _, line := range diagram.Lines {
		if line.X1 != line.X2 || line.Y2 <= line.Y1 {
			t.Errorf("handover must point straight down: (%d,%d) to (%d,%d)", line.X1, line.Y1, line.X2, line.Y2)
		}
	}
}
