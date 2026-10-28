// SPDX-License-Identifier: GPL-3.0-or-later

package plan

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestFrozenWhitespace(t *testing.T) {
	for _, frozen := range []string{" \t2026-10-30\n ", " \t\n"} {
		t.Run(fmt.Sprintf("%q", frozen), func(t *testing.T) {
			e := mkEvent(1, Person{ID: "a"})
			e.Frozen = frozen
			p := mustCompute(t, e, nil)
			if e.Frozen != strings.TrimSpace(frozen) {
				t.Fatalf("frozen = %q", e.Frozen)
			}
			wantDay := 1
			if e.Frozen != "" {
				wantDay = 2
			}
			if p.Slot("a").Day != wantDay {
				t.Fatalf("slot day = %d, want %d", p.Slot("a").Day, wantDay)
			}
		})
	}
}

func TestFrozenHorizonError(t *testing.T) {
	e := mkEvent(1, Person{ID: "a"})
	start, _ := ParseDate(e.Start)
	dates := Dates(start, MaxDays+1, e.Weekends, e.Every)
	for _, days := range []int{MaxDays - 1, MaxDays, MaxDays + 1} {
		t.Run(fmt.Sprint(days), func(t *testing.T) {
			e.Frozen = dates[days-1].Format(DateLayout)
			p, err := Compute(e, nil)
			if days < MaxDays {
				if err != nil || p.Slot("a").Day != MaxDays {
					t.Fatalf("last available day: plan %v, error %v", p, err)
				}
				return
			}
			want := fmt.Sprintf("all %d plan days are frozen; no day remains for unpinned slots", MaxDays)
			if err == nil || err.Error() != want {
				t.Fatalf("got %v, want %q", err, want)
			}
			if _, err := Compute(e, []Pin{{Label: "a", Day: 1, Voucher: 1}}); err != nil {
				t.Fatalf("fully pinned past needs no future day: %v", err)
			}
		})
	}
}

func TestLostOrderCanonical(t *testing.T) {
	want := []Lost{{Voucher: 1, Date: "2026-11-04"}, {Voucher: 2, Date: "2026-11-03"}, {Voucher: 3, Date: "2026-11-02"}}
	var fingerprint, link string
	var export []byte
	for _, order := range [][]int{{0, 1, 2}, {2, 0, 1}, {1, 2, 0}} {
		e := mkEvent(3, members(6)...)
		for _, i := range order {
			e.Lost = append(e.Lost, want[i])
		}
		p := mustCompute(t, e, nil)
		if !reflect.DeepEqual(e.Lost, want) {
			t.Errorf("normalized lost = %v, want %v", e.Lost, want)
		}
		gotLink, err := EncodeState(&State{Mode: ModeAdmin, Event: e})
		if err != nil {
			t.Fatal(err)
		}
		gotExport, err := p.JSON()
		if err != nil {
			t.Fatal(err)
		}
		if fingerprint == "" {
			fingerprint, link, export = p.Fingerprint(), gotLink, gotExport
		} else {
			if p.Fingerprint() != fingerprint {
				t.Error("lost order changed fingerprint")
			}
			if gotLink != link {
				t.Error("lost order changed link")
			}
			if !bytes.Equal(gotExport, export) {
				t.Error("lost order changed export")
			}
		}
	}
}
