// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/bmmmm/vouch0rzii/internal/plan"
)

const example = "events/congress2026.json"

func runOK(t *testing.T, args ...string) string {
	t.Helper()
	var out, errb bytes.Buffer
	if code := run(args, &out, &errb); code != 0 {
		t.Fatalf("exit %d: %s", code, errb.String())
	}
	return out.String()
}

func TestExampleMarkdown(t *testing.T) {
	out := runOK(t, "-event", example)
	for _, want := range []string{"# congress2026", "7 vouchers, 7 days, 47 slots", "Fr 30.10.", "Mo 09.11.", "mammut+10"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in\n%s", want, out)
		}
	}
}

func TestExampleJSONRespectsTiersAndCounts(t *testing.T) {
	var ex plan.Export
	if err := json.Unmarshal([]byte(runOK(t, "-event", example, "-format", "json")), &ex); err != nil {
		t.Fatal(err)
	}
	if len(ex.Days) != 7 || ex.Stats.Slots != 47 || ex.Stats.Handovers != 40 {
		t.Fatalf("stats %+v, days %d", ex.Stats, len(ex.Days))
	}
	named := map[string]int{}
	plus := 0
	maxNamedDay, minPlusDay := 0, 99
	for d, row := range ex.Grid {
		for _, l := range row {
			switch {
			case l == "":
			case strings.Contains(l, "+"):
				plus++
				if d+1 < minPlusDay {
					minPlusDay = d + 1
				}
			default:
				named[l]++
				if d+1 > maxNamedDay {
					maxNamedDay = d + 1
				}
			}
		}
	}
	if len(named) != 25 || plus != 22 {
		t.Fatalf("named %d plus %d", len(named), plus)
	}
	for id, n := range named {
		if n != 1 {
			t.Errorf("%s appears %d times", id, n)
		}
	}
	if maxNamedDay > minPlusDay {
		t.Fatalf("a +N slot (day %d) comes before a named person (day %d)", minPlusDay, maxNamedDay)
	}
	for _, h := range ex.Handovers {
		if h.Forbidden {
			t.Errorf("forbidden handover %s → %s", h.From, h.To)
		}
	}
	if ex.Stats.Known == 0 {
		t.Error("the example has connections; some handovers should be between people who know each other")
	}
}

func TestSeedChangesDrawNotLength(t *testing.T) {
	a := runOK(t, "-event", example, "-format", "csv")
	b := runOK(t, "-event", example, "-format", "csv", "-seed", "other")
	if a == b {
		t.Fatal("another seed should change the draw")
	}
	if !strings.Contains(runOK(t, "-event", example, "-seed", "other"), "7 days") {
		t.Fatal("another seed must not change the number of days")
	}
}

func TestRosterFileReplacesPeople(t *testing.T) {
	path := filepath.Join(t.TempDir(), "roster.tsv")
	if err := os.WriteFile(path, []byte("Name\tStatus\t+N\nigel\tMitglied\t1\nkoala\tGast\t\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var ex plan.Export
	if err := json.Unmarshal([]byte(runOK(t, "-event", example, "-roster", path, "-format", "json")), &ex); err != nil {
		t.Fatal(err)
	}
	if ex.Stats.Slots != 3 || ex.Grid[0][0] != "igel" {
		t.Fatalf("export %+v", ex.Stats)
	}
}

func TestShareLinks(t *testing.T) {
	out := strings.TrimSpace(runOK(t, "-event", example, "-share", "view"))
	if !strings.HasPrefix(out, DefaultBase+"#s=") {
		t.Fatalf("link %q", out)
	}
	st, err := plan.DecodeState(strings.TrimPrefix(out, DefaultBase+"#s="))
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode != plan.ModeView || len(st.Pins) != 47 || len(st.Event.People) != 25 {
		t.Fatalf("view state: mode %s pins %d people %d", st.Mode, len(st.Pins), len(st.Event.People))
	}
	out = strings.TrimSpace(runOK(t, "-event", example, "-share", "collect", "-to", "orga@example.org", "-base", "http://localhost/"))
	st, err = plan.DecodeState(strings.TrimPrefix(out, "http://localhost/#s="))
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode != plan.ModeCollect || len(st.Event.People) != 0 || st.Collect.To != "orga@example.org" {
		t.Fatalf("collect state %+v", st)
	}
	var errb bytes.Buffer
	if code := run([]string{"-event", example, "-share", "collect"}, &bytes.Buffer{}, &errb); code != 2 || !strings.Contains(errb.String(), "-to") {
		t.Fatalf("collect without -to: exit %d, %s", code, errb.String())
	}
}

func TestRejectsBadInput(t *testing.T) {
	for _, args := range [][]string{{}, {"-event", "events/nope.json"}, {"-event", example, "-format", "bogus"}} {
		var errb bytes.Buffer
		if code := run(args, &bytes.Buffer{}, &errb); code != 2 {
			t.Errorf("%v: exit %d, want 2 (%s)", args, code, errb.String())
		}
	}
}

func TestSVGFormat(t *testing.T) {
	svg := runOK(t, "-event", example, "-format", "svg")
	if !strings.HasPrefix(svg, "<svg ") || !strings.Contains(svg, ">mammut+10<") || strings.Count(svg, "<rect ") != 47 {
		t.Fatalf("svg:\n%.300s", svg)
	}
}

func TestEventFormatPrintsNormalisedFile(t *testing.T) {
	ev := runOK(t, "-event", example, "-format", "event")
	if !strings.Contains(ev, `"name": "congress2026"`) || !strings.Contains(ev, `"id": "mammut"`) {
		t.Fatalf("event:\n%s", ev)
	}
}
