// SPDX-License-Identifier: GPL-3.0-or-later

package plan

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"reflect"
	"strings"
	"testing"
)

func TestStateRoundTrip(t *testing.T) {
	e := mkEvent(2, ordered("a", "b", "c")...)
	st := &State{Mode: ModeAdmin, Event: e, Pins: []Pin{{Label: "a", Day: 1, Voucher: 2}}}
	enc, err := EncodeState(st)
	if err != nil {
		t.Fatal(err)
	}
	if strings.ContainsAny(enc, "+/=") {
		t.Fatalf("not URL-safe: %s", enc)
	}
	got, err := DecodeState(enc)
	if err != nil {
		t.Fatal(err)
	}
	if got.V != StateVersion || got.Mode != ModeAdmin || !reflect.DeepEqual(got.Pins, st.Pins) || !reflect.DeepEqual(got.Event, e) {
		t.Fatalf("round trip changed state: %+v", got)
	}
}

func TestCollectStateCarriesNoRoster(t *testing.T) {
	e := mkEvent(2, ordered("a", "b")...)
	enc, err := EncodeState(&State{Mode: ModeCollect, Event: e, Pins: []Pin{{Label: "a", Day: 1, Voucher: 1}}, Collect: &Collect{To: "orga@example.org"}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := DecodeState(enc)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Event.People) != 0 || got.Pins != nil || got.Event.Name != "t" || got.Collect.To != "orga@example.org" {
		t.Fatalf("collect state leaks roster: %+v", got)
	}
	if len(e.People) != 2 {
		t.Fatal("EncodeState must not mutate the caller's event")
	}
}

func rawState(js string) string {
	var buf bytes.Buffer
	w, _ := flate.NewWriter(&buf, flate.BestCompression)
	w.Write([]byte(js))
	w.Close()
	return base64.RawURLEncoding.EncodeToString(buf.Bytes())
}

func TestDecodeStateRejectsBadInput(t *testing.T) {
	cases := []struct{ in, want string }{
		{"%%%", "not base64url"},
		{base64.RawURLEncoding.EncodeToString([]byte("plain")), "not deflate"},
		{rawState(`{"v":2,"mode":"admin"}`), "version 2"},
		{rawState(`{"v":1,"mode":"root"}`), "unknown mode"},
		{rawState(`{"v":1,"mode":`), "bad json"},
	}
	for _, tc := range cases {
		_, err := DecodeState(tc.in)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%q: got %v want %q", tc.in, err, tc.want)
		}
	}
	if _, err := EncodeState(&State{Mode: "root"}); err == nil {
		t.Error("unknown mode must not encode")
	}
}
