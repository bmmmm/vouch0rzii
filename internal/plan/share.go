// SPDX-License-Identifier: GPL-3.0-or-later

package plan

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// StateVersion is bumped when the State schema changes incompatibly; older
// links then fail loudly instead of rendering nonsense.
const StateVersion = 1

// Modes of the page. They are views, not access control: whoever holds a link
// holds the data in it.
const (
	ModeAdmin   = "admin"   // edit roster and settings, pin slots, share links
	ModeView    = "view"    // read the plan; carries a frozen plan (all slots pinned)
	ModeCollect = "collect" // sign-up form; carries settings and a contact address, no roster
)

// State is everything the page knows, serialised into the URL fragment.
type State struct {
	V       int      `json:"v"`
	Mode    string   `json:"mode"`
	Event   *Event   `json:"event,omitempty"`
	Pins    []Pin    `json:"pins,omitempty"`
	Collect *Collect `json:"collect,omitempty"`
}

// Collect configures the sign-up mode.
type Collect struct {
	To string `json:"to"` // address the sign-up line is mailed to
}

// maxStateBytes bounds the decompressed state; a roster of a few hundred
// people stays far below it.
const maxStateBytes = 4 << 20

// EncodeState serialises a state as base64url(deflate(json)) for a URL
// fragment. Collect states are stripped of roster and pins.
func EncodeState(st *State) (string, error) {
	if st.Mode != ModeAdmin && st.Mode != ModeView && st.Mode != ModeCollect {
		return "", fmt.Errorf("unknown mode %q", st.Mode)
	}
	out := *st
	out.V = StateVersion
	if out.Mode == ModeCollect && out.Event != nil {
		ev := *out.Event
		ev.People = nil
		out.Event = &ev
		out.Pins = nil
	}
	js, err := json.Marshal(&out)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	w, err := flate.NewWriter(&buf, flate.BestCompression)
	if err != nil {
		return "", err
	}
	if _, err := w.Write(js); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf.Bytes()), nil
}

// DecodeState reverses EncodeState and checks the schema version and mode.
func DecodeState(s string) (*State, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("state: not base64url: %w", err)
	}
	r := flate.NewReader(bytes.NewReader(raw))
	defer r.Close()
	js, err := io.ReadAll(io.LimitReader(r, maxStateBytes+1))
	if err != nil {
		return nil, fmt.Errorf("state: not deflate: %w", err)
	}
	if len(js) > maxStateBytes {
		return nil, errors.New("state: too large")
	}
	var st State
	if err := json.Unmarshal(js, &st); err != nil {
		return nil, fmt.Errorf("state: bad json: %w", err)
	}
	if st.V != StateVersion {
		return nil, fmt.Errorf("state: version %d, this page reads %d", st.V, StateVersion)
	}
	switch st.Mode {
	case ModeAdmin, ModeView, ModeCollect:
	default:
		return nil, fmt.Errorf("state: unknown mode %q", st.Mode)
	}
	return &st, nil
}
