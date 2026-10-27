// SPDX-License-Identifier: GPL-3.0-or-later

package plan

import (
	"testing"
	"unicode/utf8"
)

func TestFingerprint(t *testing.T) {
	if len(fingerprintEmoji) != 64 {
		t.Fatalf("want 64 emoji, got %d", len(fingerprintEmoji))
	}
	e := renderEvent()
	e.People[0].Note = "private"
	p := mustCompute(t, e, nil)
	fp := p.Fingerprint()
	if utf8.RuneCountInString(fp) != 4 {
		t.Fatalf("want 4 runes, got %q", fp)
	}
	// The viewer link (frozen pins, no notes) renders the same fingerprint.
	enc, err := EncodeState(&State{Mode: ModeView, Event: e, Pins: Freeze(p)})
	if err != nil {
		t.Fatal(err)
	}
	st, err := DecodeState(enc)
	if err != nil {
		t.Fatal(err)
	}
	if got := mustCompute(t, st.Event, st.Pins).Fingerprint(); got != fp {
		t.Fatalf("viewer %q, admin %q", got, fp)
	}
	e2 := renderEvent()
	e2.People[1].Email = "other@example.org"
	if mustCompute(t, e2, nil).Fingerprint() == fp {
		t.Fatal("a changed e-mail must change the fingerprint")
	}
	if mustCompute(t, renderEvent(), []Pin{{Label: "c", Day: 2, Voucher: 2}}).Fingerprint() == fp {
		t.Fatal("a changed placement must change the fingerprint")
	}
}
