// SPDX-License-Identifier: GPL-3.0-or-later

package plan

import (
	"reflect"
	"testing"
)

func TestDatesSkipWeekends(t *testing.T) {
	start, err := ParseDate("2026-10-30") // a Friday
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, d := range Dates(start, 7, false) {
		got = append(got, d.Format(DateLayout))
	}
	want := []string{"2026-10-30", "2026-11-02", "2026-11-03", "2026-11-04", "2026-11-05", "2026-11-06", "2026-11-09"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("skip weekends: got %v want %v", got, want)
	}
	got = got[:0]
	for _, d := range Dates(start, 3, true) {
		got = append(got, d.Format(DateLayout))
	}
	want = []string{"2026-10-30", "2026-10-31", "2026-11-01"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("with weekends: got %v want %v", got, want)
	}
}

func TestDatesWeekendStartMovesToMonday(t *testing.T) {
	start, _ := ParseDate("2026-10-31") // Saturday
	if got := Dates(start, 1, false)[0].Format(DateLayout); got != "2026-11-02" {
		t.Fatalf("got %s want 2026-11-02", got)
	}
}

func TestDayLabel(t *testing.T) {
	d, _ := ParseDate("2026-10-30")
	if got := DayLabel(d); got != "Fri 30/10" {
		t.Fatalf("got %q", got)
	}
}
