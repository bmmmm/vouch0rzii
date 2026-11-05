// SPDX-License-Identifier: GPL-3.0-or-later

package plan

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

// dates formats the first n plan days from start for comparison.
func dates(t *testing.T, start string, n int, week Week, every int) []string {
	t.Helper()
	s, err := ParseDate(start)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, d := range Dates(s, n, week, every) {
		out = append(out, d.Format(DateLayout))
	}
	return out
}

func TestDatesSkipWeekends(t *testing.T) {
	got := dates(t, "2026-10-30", 7, workweek, 1) // a Friday
	want := []string{"2026-10-30", "2026-11-02", "2026-11-03", "2026-11-04", "2026-11-05", "2026-11-06", "2026-11-09"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("skip weekends: got %v want %v", got, want)
	}
	got = dates(t, "2026-10-30", 3, fullWeek, 0)
	want = []string{"2026-10-30", "2026-10-31", "2026-11-01"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("with weekends: got %v want %v", got, want)
	}
}

func TestDatesEveryOtherDay(t *testing.T) {
	for _, tc := range []struct {
		name string
		week Week
		want []string
	}{
		{"workweek", workweek, []string{"2026-10-30", "2026-11-03", "2026-11-05", "2026-11-09", "2026-11-11"}}, // the weekend rests: every counts weekdays
		{"full week", fullWeek, []string{"2026-10-30", "2026-11-01", "2026-11-03", "2026-11-05", "2026-11-07"}},
	} {
		if got := dates(t, "2026-10-30", 5, tc.week, 2); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got %v want %v", tc.name, got, tc.want)
		}
	}
}

// A week of Monday, Wednesday and Friday repeats every week, which no every
// over the workweek can: the voucher process runs on those days only.
func TestDatesMondayWednesdayFriday(t *testing.T) {
	mwf := Week{1: true, 3: true, 5: true}
	for _, tc := range []struct {
		start string
		every int
		want  []string
	}{
		{"2026-10-30", 1, []string{"2026-10-30", "2026-11-02", "2026-11-04", "2026-11-06", "2026-11-09"}},
		{"2026-10-30", 2, []string{"2026-10-30", "2026-11-04", "2026-11-09", "2026-11-13", "2026-11-18"}},
		{"2026-11-03", 1, []string{"2026-11-04", "2026-11-06", "2026-11-09"}}, // a Tuesday start moves to Wednesday
	} {
		if got := dates(t, tc.start, len(tc.want), mwf, tc.every); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("start %s every %d: got %v want %v", tc.start, tc.every, got, tc.want)
		}
	}
	if got := Dates(mustDate(t, "2026-10-30"), 3, Week{}, 1); len(got) != 0 {
		t.Errorf("an empty week must yield no day, got %v", got)
	}
}

func mustDate(t *testing.T, s string) (d time.Time) {
	t.Helper()
	d, err := ParseDate(s)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func TestDatesWeekendStartMovesToMonday(t *testing.T) {
	if got := dates(t, "2026-10-31", 1, workweek, 2); got[0] != "2026-11-02" { // Saturday
		t.Fatalf("got %s want 2026-11-02", got[0])
	}
}

// Days is written in one canonical form, so one week is one event and one
// fingerprint: the workweek as no days, all seven as weekends, other sets from
// Monday on in English.
func TestNormalizeDays(t *testing.T) {
	for _, tc := range []struct {
		name     string
		days     []string
		weekends bool
		want     []string
		wantWE   bool
		err      string
	}{
		{name: "none is the workweek", want: nil},
		{name: "none with weekends", weekends: true, wantWE: true},
		{name: "the workweek is no days", days: []string{"fri", "thu", "wed", "tue", "mon"}, want: nil},
		{name: "German, mixed case, unordered", days: []string{" Fr", "MO", "mi", "mo"}, want: []string{"mon", "wed", "fri"}},
		{name: "Sunday last", days: []string{"so", "sat"}, want: []string{"sat", "sun"}},
		{name: "all seven are weekends", days: []string{"so", "mo", "di", "mi", "do", "fr", "sa"}, wantWE: true},
		{name: "all seven and weekends", days: []string{"sun", "mon", "tue", "wed", "thu", "fri", "sat"}, weekends: true, wantWE: true},
		{name: "empty list", days: []string{}, err: "names no weekday"},
		{name: "unknown name", days: []string{"mon", "monday"}, err: `"monday" is not a weekday`},
		{name: "weekends disagree", days: []string{"mon"}, weekends: true, err: "disagree"},
		{name: "too many", days: strings.Fields("mon tue wed thu fri sat sun mon"), err: "a week has 7"},
	} {
		e := mkEvent(2, ordered("a", "b")...)
		e.Days, e.Weekends = tc.days, tc.weekends
		_, err := Normalize(e)
		switch {
		case tc.err != "":
			if err == nil || !strings.Contains(err.Error(), tc.err) {
				t.Errorf("%s: err %v, want %q", tc.name, err, tc.err)
			}
		case err != nil:
			t.Errorf("%s: %v", tc.name, err)
		case !reflect.DeepEqual(e.Days, tc.want) || e.Weekends != tc.wantWE:
			t.Errorf("%s: days %q weekends %v, want %q %v", tc.name, e.Days, e.Weekends, tc.want, tc.wantWE)
		}
	}
}

// "days": [] in a file is not the default workweek but an error, unlike no
// days at all.
func TestParseEventEmptyDays(t *testing.T) {
	e, err := ParseEvent([]byte(`{"name": "t", "vouchers": 1, "start": "2026-10-30", "days": [], "people": [{"id": "a", "tier": "member"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Normalize(e); err == nil || !strings.Contains(err.Error(), "names no weekday") {
		t.Fatalf("want an error for an empty days list, got %v", err)
	}
}

// The page ticks its boxes from the week Go read, also when another field
// fails: a valid days list is canonical after Normalize either way.
func TestNormalizeDaysWhenAnotherFieldFails(t *testing.T) {
	e := &Event{Name: "t", Vouchers: 0, Start: "2026-10-30", Days: []string{"Mo", " mi", "FR"}, People: []Person{{ID: "a", Tier: TierMember}}}
	if _, err := Normalize(e); err == nil || !strings.Contains(err.Error(), "vouchers must be") {
		t.Fatalf("want the vouchers error, got %v", err)
	}
	if got := strings.Join(e.Days, ","); got != "mon,wed,fri" {
		t.Fatalf("days %q, want mon,wed,fri", got)
	}
}

func TestDayLabel(t *testing.T) {
	d, _ := ParseDate("2026-10-30")
	if got := DayLabel(d); got != "Fri 30/10" {
		t.Fatalf("got %q", got)
	}
}
