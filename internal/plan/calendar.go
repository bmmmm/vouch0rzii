// SPDX-License-Identifier: GPL-3.0-or-later

package plan

import (
	"strings"
	"time"
)

// DateLayout is the only date format accepted in events and emitted in plans.
const DateLayout = "2006-01-02"

var weekdayShort = [...]string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}

// Week marks, indexed by time.Weekday, the weekdays on which the voucher
// process runs: buying days, and the evenings a new voucher can come.
type Week [7]bool

var (
	workweek = Week{time.Monday: true, time.Tuesday: true, time.Wednesday: true, time.Thursday: true, time.Friday: true}
	fullWeek = Week{true, true, true, true, true, true, true}
)

// weekdayAliases maps lower-cased weekday names, English and German, to a
// weekday; the canonical spelling is weekdayShort lower-cased.
var weekdayAliases = map[string]time.Weekday{
	"sun": time.Sunday, "so": time.Sunday,
	"mon": time.Monday, "mo": time.Monday,
	"tue": time.Tuesday, "di": time.Tuesday,
	"wed": time.Wednesday, "mi": time.Wednesday,
	"thu": time.Thursday, "do": time.Thursday,
	"fri": time.Friday, "fr": time.Friday,
	"sat": time.Saturday, "sa": time.Saturday,
}

// parseWeekday reads one entry of Event.Days.
func parseWeekday(s string) (time.Weekday, bool) {
	d, ok := weekdayAliases[strings.ToLower(strings.TrimSpace(s))]
	return d, ok
}

// Week returns the plan weekdays of the event: Days, else all seven with
// Weekends, else Monday to Friday.
func (e *Event) Week() Week {
	if len(e.Days) == 0 {
		if e.Weekends {
			return fullWeek
		}
		return workweek
	}
	var w Week
	for _, s := range e.Days {
		if d, ok := parseWeekday(s); ok {
			w[d] = true
		}
	}
	return w
}

// ParseDate parses YYYY-MM-DD.
func ParseDate(s string) (time.Time, error) {
	return time.Parse(DateLayout, s)
}

// Dates returns the calendar dates of days 1..n starting at start: every
// every-th plan day (every < 1 counts as 1). Plan days are the days of week,
// because on the other days the voucher process rests too: every counts plan
// days only, and a start on another day moves day 1 to the next plan day.
func Dates(start time.Time, n int, week Week, every int) []time.Time {
	every = max(every, 1)
	out := make([]time.Time, 0, n)
	for i, d := range planDays(start, n*every, week) {
		if i%every == 0 {
			out = append(out, d)
		}
	}
	return out
}

// planDays returns the first n days from start on which the voucher process
// runs, the days of week; an empty week yields none.
func planDays(start time.Time, n int, week Week) []time.Time {
	out := make([]time.Time, 0, n)
	if week == (Week{}) {
		return out
	}
	for d := start; len(out) < n; d = d.AddDate(0, 0, 1) {
		if week[d.Weekday()] {
			out = append(out, d)
		}
	}
	return out
}

// DayLabel formats a date as "Fri 30/10" for tables.
func DayLabel(t time.Time) string {
	return weekdayShort[t.Weekday()] + " " + t.Format("02/01")
}
