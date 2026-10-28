// SPDX-License-Identifier: GPL-3.0-or-later

package plan

import "time"

// DateLayout is the only date format accepted in events and emitted in plans.
const DateLayout = "2006-01-02"

var weekdayShort = [...]string{"Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"}

// ParseDate parses YYYY-MM-DD.
func ParseDate(s string) (time.Time, error) {
	return time.Parse(DateLayout, s)
}

// Dates returns the calendar dates of days 1..n starting at start: every
// every-th plan day (every < 1 counts as 1). Plan days are all days with
// weekends, else Monday to Friday, because then the voucher process rests on
// Saturday and Sunday too: every counts weekdays, and a start on a weekend
// moves day 1 to the following Monday.
func Dates(start time.Time, n int, weekends bool, every int) []time.Time {
	every = max(every, 1)
	out := make([]time.Time, 0, n)
	for i, d := range planDays(start, n*every, weekends) {
		if i%every == 0 {
			out = append(out, d)
		}
	}
	return out
}

// planDays returns the first n days from start on which the voucher process
// runs: all days with weekends, else Monday to Friday.
func planDays(start time.Time, n int, weekends bool) []time.Time {
	out := make([]time.Time, 0, n)
	for d := start; len(out) < n; d = d.AddDate(0, 0, 1) {
		if weekends || (d.Weekday() != time.Saturday && d.Weekday() != time.Sunday) {
			out = append(out, d)
		}
	}
	return out
}

// DayLabel formats a date as "Fri 30/10" for tables.
func DayLabel(t time.Time) string {
	return weekdayShort[t.Weekday()] + " " + t.Format("02/01")
}
