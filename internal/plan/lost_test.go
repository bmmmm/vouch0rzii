// SPDX-License-Identifier: GPL-3.0-or-later

package plan

import (
	"reflect"
	"strings"
	"testing"
)

// A lost voucher carries nobody from its day on: the slots move to the
// vouchers left, the free count leaves its row out, and the export and the
// Markdown say from which day it is gone.
func TestLostVoucherClosesItsRow(t *testing.T) {
	e := mkEvent(2, ordered("a", "b", "c", "d", "e", "f")...)
	e.Lost = []Lost{{Voucher: 2, Date: " 2026-11-03"}} // day 3; the date is trimmed like the others
	p := mustCompute(t, e, nil)
	ex := p.ToExport()
	want := [][]string{{"a", "b"}, {"d", "c"}, {"e", ""}, {"f", ""}}
	if !reflect.DeepEqual(ex.Grid, want) || !reflect.DeepEqual(ex.VoucherLost, []int{0, 3}) || p.Stats.Free != 0 {
		t.Fatalf("grid %v, voucherLost %v, free %d", ex.Grid, ex.VoucherLost, p.Stats.Free)
	}
	day, _ := ParseDate("2026-11-03")
	if md := p.Markdown(); !strings.Contains(md, "Voucher 2 lost from "+DayLabel(day)+".") {
		t.Fatalf("markdown without the lost voucher:\n%s", md)
	}
	if ex := mustCompute(t, mkEvent(2, ordered("a")...), nil).ToExport(); !reflect.DeepEqual(ex.VoucherLost, []int{0, 0}) {
		t.Fatalf("voucherLost without a lost voucher: %v", ex.VoucherLost)
	}
}

// An extra voucher can be lost as well, numbered after the base vouchers,
// and a voucher lost from day 1 is named in the Markdown too.
func TestLostExtraVoucher(t *testing.T) {
	e := mkEventExtra() // voucher 2 from day 3
	e.People = ordered("a", "b", "c", "d", "e", "f")
	e.Lost = []Lost{{Voucher: 2, Date: "2026-11-04"}} // day 4
	p := mustCompute(t, e, nil)
	if ex := p.ToExport(); !reflect.DeepEqual(ex.VoucherLost, []int{0, 4}) || p.Stats.Days != 5 {
		t.Fatalf("voucherLost %v, %d days, grid %v", ex.VoucherLost, p.Stats.Days, ex.Grid)
	}
	e = mkEvent(2, ordered("a", "b")...)
	e.Lost = []Lost{{Voucher: 1, Date: "2026-10-30"}}
	day, _ := ParseDate("2026-10-30")
	if md := mustCompute(t, e, nil).Markdown(); !strings.Contains(md, "Voucher 1 lost from "+DayLabel(day)+".") {
		t.Fatalf("markdown without the voucher lost from day 1:\n%s", md)
	}
}

// With every voucher lost the rest goes to the public sale, also without an
// end; one voucher left keeps the plan going.
func TestAllVouchersLost(t *testing.T) {
	e := mkEvent(2, ordered("a", "b", "c", "d", "e")...)
	e.Lost = []Lost{{Voucher: 1, Date: "2026-11-02"}, {Voucher: 2, Date: "2026-11-03"}} // days 2 and 3
	p := mustCompute(t, e, nil)
	if got := p.ToExport().Grid; !reflect.DeepEqual(got, [][]string{{"a", "b"}, {"", "c"}}) || !reflect.DeepEqual(p.Public(), []string{"d", "e"}) {
		t.Fatalf("grid %v, public %v", got, p.Public())
	}
	e.Lost = e.Lost[:1]
	if p := mustCompute(t, e, nil); len(p.Public()) != 0 || p.Stats.Days != 4 {
		t.Fatalf("one voucher left: public %v, %d days", p.Public(), p.Stats.Days)
	}
}

// Marking a voucher lost after pinning the past: pins on its closed cells
// are dropped with a warning, those slots move behind the frozen days, a
// person who left keeps no slot through such a pin, and the other pins stay.
func TestLostVoucherDropsItsPins(t *testing.T) {
	ps := ordered("a", "b", "c", "d", "e", "f", "g", "h")
	ps = append(ps, Person{ID: "x", Out: true})
	e := mkEvent(2, ps...)
	e.Lost = []Lost{{Voucher: 2, Date: "2026-11-02"}} // day 2
	e.Frozen = "2026-11-03"                           // day 3
	pins := []Pin{{"a", 1, 1}, {"b", 1, 2}, {"c", 2, 1}, {"d", 2, 2}, {"e", 3, 1}, {"f", 3, 2}, {"x", 4, 2}}
	p := mustCompute(t, e, pins)
	want := [][]string{{"a", "b"}, {"c", ""}, {"e", ""}, {"d", ""}, {"f", ""}, {"g", ""}, {"h", ""}}
	if got := p.ToExport().Grid; !reflect.DeepEqual(got, want) || p.Stats.Free != 0 || p.Stats.Pinned != 4 {
		t.Fatalf("grid %v, free %d, pinned %d", got, p.Stats.Free, p.Stats.Pinned)
	}
	for _, w := range []string{"pin d: voucher 2 is lost from day 2, dropped", "pin f: voucher 2 is lost from day 2, dropped"} {
		if !strings.Contains(strings.Join(p.Warnings, "\n"), w) {
			t.Fatalf("warning %q missing: %v", w, p.Warnings)
		}
	}
}
